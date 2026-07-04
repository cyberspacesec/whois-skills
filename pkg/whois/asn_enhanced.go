package whois

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ASNDetail ASN详细信息（增强版）
type ASNDetail struct {
	// ASN号码
	ASN int `json:"asn"`

	// ASN字符串表示 (AS12345)
	ASNString string `json:"asn_string"`

	// AS名称
	Name string `json:"name,omitempty"`

	// 所属组织
	Organization string `json:"organization,omitempty"`

	// 所属国家
	Country string `json:"country,omitempty"`

	// 所属RIR
	RIR string `json:"rir,omitempty"`

	// 分配日期
	AllocationDate string `json:"allocation_date,omitempty"`

	// 注册状态
	Status string `json:"status,omitempty"`

	// AS描述
	Description string `json:"description,omitempty"`

	// IPv4前缀列表
	IPv4Prefixes []string `json:"ipv4_prefixes,omitempty"`

	// IPv6前缀列表
	IPv6Prefixes []string `json:"ipv6_prefixes,omitempty"`

	// 上游AS列表
	UpstreamASNs []int `json:"upstream_asns,omitempty"`

	// 下游AS列表
	DownstreamASNs []int `json:"downstream_asns,omitempty"`

	// 对等AS列表
	PeerASNs []int `json:"peer_asns,omitempty"`

	// 查询来源
	Source string `json:"source,omitempty"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`
}

// ASNQueryOptions ASN查询选项
type ASNQueryOptions struct {
	// ASN号码
	ASN int `json:"asn"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// 查询来源
	Source ASNQuerySource `json:"source,omitempty"`

	// 是否查询前缀信息
	IncludePrefixes bool `json:"include_prefixes,omitempty"`

	// 是否查询BGP关系
	IncludeBGP bool `json:"include_bgp,omitempty"`
}

// ASNQuerySource ASN查询来源
type ASNQuerySource string

const (
	// ASNSourceRADB 从RADB查询
	ASNSourceRADB ASNQuerySource = "radb"

	// ASNSourceRDAP 从RDAP查询
	ASNSourceRDAP ASNQuerySource = "rdap"

	// ASNSourceAll 从所有来源查询并合并
	ASNSourceAll ASNQuerySource = "all"
)

// ASNBatchResult ASN批量查询结果
type ASNBatchResult struct {
	// 成功结果
	Results map[int]*ASNDetail `json:"results"`

	// 失败结果
	Errors map[int]error `json:"errors"`

	// 统计信息
	TotalQueried int `json:"total_queried"`
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
}

// asnDetailCache ASN信息缓存
var asnDetailCache struct {
	mu    sync.RWMutex
	items map[int]*ASNDetail
}

func init() {
	asnDetailCache.items = make(map[int]*ASNDetail)
}

// QueryASN 查询ASN详细信息
func QueryASN(asn int) (*ASNDetail, error) {
	return QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:             asn,
		Source:          ASNSourceAll,
		IncludePrefixes: true,
	})
}

// QueryASNWithContext 使用上下文查询ASN信息
func QueryASNWithContext(ctx context.Context, opts *ASNQueryOptions) (*ASNDetail, error) {
	if opts == nil || opts.ASN <= 0 {
		return nil, fmt.Errorf("ASN号码无效")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	// 检查缓存
	asnDetailCache.mu.RLock()
	if cached, ok := asnDetailCache.items[opts.ASN]; ok {
		asnDetailCache.mu.RUnlock()
		return cached, nil
	}
	asnDetailCache.mu.RUnlock()

	info := &ASNDetail{
		ASN:       opts.ASN,
		ASNString: fmt.Sprintf("AS%d", opts.ASN),
		QueryTime: time.Now(),
	}

	var lastErr error

	// 根据来源查询
	switch opts.Source {
	case ASNSourceRADB:
		err := queryASNFromRADB(ctx, opts, info)
		if err != nil {
			return nil, err
		}
		info.Source = "radb"

	case ASNSourceRDAP:
		err := queryASNFromRDAP(ctx, opts, info)
		if err != nil {
			return nil, err
		}
		info.Source = "rdap"

	case ASNSourceAll:
		// 先尝试RDAP（更结构化）
		if err := queryASNFromRDAP(ctx, opts, info); err != nil {
			logrus.Debugf("RDAP ASN查询失败: %v, 尝试RADB", err)
			lastErr = err
			// 回退到RADB
			if err2 := queryASNFromRADB(ctx, opts, info); err2 != nil {
				if lastErr != nil {
					return nil, fmt.Errorf("RDAP错误: %v; RADB错误: %v", lastErr, err2)
				}
				return nil, err2
			}
			info.Source = "radb"
		} else {
			info.Source = "rdap"
		}

		// 补充前缀信息
		if opts.IncludePrefixes && len(info.IPv4Prefixes) == 0 && len(info.IPv6Prefixes) == 0 {
			_ = queryASNPrefixesFromRADB(ctx, opts, info)
		}

	default:
		return nil, fmt.Errorf("不支持的查询来源: %s", opts.Source)
	}

	// 补充 BGP 关系（若启用）
	queryASNRelations(ctx, opts, info)

	// 缓存结果
	asnDetailCache.mu.Lock()
	asnDetailCache.items[opts.ASN] = info
	asnDetailCache.mu.Unlock()

	return info, nil
}

// queryASNFromRDAP 通过RDAP查询ASN信息
func queryASNFromRDAP(ctx context.Context, opts *ASNQueryOptions, info *ASNDetail) error {
	rdapResult, err := QueryRDAP_ASNWithContext(ctx, &RDAPQueryOptions{
		ASN:     fmt.Sprintf("AS%d", opts.ASN),
		Timeout: opts.Timeout,
	})

	if err != nil {
		return err
	}

	if rdapResult.Name != "" {
		info.Name = rdapResult.Name
	}
	if rdapResult.Country != "" {
		info.Country = rdapResult.Country
	}

	// 从Handle中提取RIR信息
	if rdapResult.Handle != "" {
		info.RIR = extractRIRFromHandle(rdapResult.Handle)
	}

	// 解析事件
	for _, event := range rdapResult.Events {
		if event.EventAction == "registration" {
			info.AllocationDate = event.EventDate
		}
	}

	if rdapResult.Type != "" {
		info.Status = rdapResult.Type
	}

	return nil
}

// queryASNFromRADB 通过RADB查询ASN信息
func queryASNFromRADB(ctx context.Context, opts *ASNQueryOptions, info *ASNDetail) error {
	dialer := net.Dialer{Timeout: time.Duration(opts.Timeout) * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", "whois.radb.net:43")
	if err != nil {
		return fmt.Errorf("连接RADB服务器失败: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(time.Duration(opts.Timeout) * time.Second)
	conn.SetDeadline(deadline)

	query := fmt.Sprintf("AS%d\r\n", opts.ASN)
	_, err = conn.Write([]byte(query))
	if err != nil {
		return fmt.Errorf("发送RADB查询失败: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "as-name:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, "as-name:"))
		} else if strings.HasPrefix(line, "descr:") {
			descr := strings.TrimSpace(strings.TrimPrefix(line, "descr:"))
			if info.Description == "" {
				info.Description = descr
			} else {
				info.Description += "; " + descr
			}
		} else if strings.HasPrefix(line, "country:") {
			info.Country = strings.TrimSpace(strings.TrimPrefix(line, "country:"))
		} else if strings.HasPrefix(line, "source:") {
			info.RIR = strings.TrimSpace(strings.TrimPrefix(line, "source:"))
		} else if strings.HasPrefix(line, "route:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				info.IPv4Prefixes = append(info.IPv4Prefixes, fields[1])
			}
		} else if strings.HasPrefix(line, "route6:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				info.IPv6Prefixes = append(info.IPv6Prefixes, fields[1])
			}
		}
	}

	return nil
}

// queryASNPrefixesFromRADB 从RADB查询ASN的IP前缀信息
func queryASNPrefixesFromRADB(ctx context.Context, opts *ASNQueryOptions, info *ASNDetail) error {
	dialer := net.Dialer{Timeout: time.Duration(opts.Timeout) * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", "whois.radb.net:43")
	if err != nil {
		return fmt.Errorf("连接RADB服务器失败: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(time.Duration(opts.Timeout) * time.Second)
	conn.SetDeadline(deadline)

	query := fmt.Sprintf("!gAS%d\r\n", opts.ASN)
	_, err = conn.Write([]byte(query))
	if err != nil {
		return fmt.Errorf("发送RADB前缀查询失败: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// RADB !g命令返回格式: A{prefixes}
		if strings.HasPrefix(line, "A") {
			prefixes := strings.TrimPrefix(line, "A")
			for _, p := range strings.Split(prefixes, " ") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if strings.Contains(p, ":") {
					info.IPv6Prefixes = append(info.IPv6Prefixes, p)
				} else {
					info.IPv4Prefixes = append(info.IPv4Prefixes, p)
				}
			}
		}
	}

	return nil
}

// BatchQueryASN 批量查询ASN信息
func BatchQueryASN(ctx context.Context, asnList []int, concurrency int) *ASNBatchResult {
	if concurrency <= 0 {
		concurrency = 5
	}

	result := &ASNBatchResult{
		Results:      make(map[int]*ASNDetail),
		Errors:       make(map[int]error),
		TotalQueried: len(asnList),
	}

	if len(asnList) == 0 {
		return result
	}

	type queryResult struct {
		asn  int
		info *ASNDetail
		err  error
	}

	resultChan := make(chan queryResult, len(asnList))
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for _, asn := range asnList {
		wg.Add(1)
		go func(a int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := QueryASNWithContext(ctx, &ASNQueryOptions{
				ASN:             a,
				Source:          ASNSourceAll,
				IncludePrefixes: true,
			})

			resultChan <- queryResult{asn: a, info: info, err: err}
		}(asn)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for qr := range resultChan {
		if qr.err != nil {
			result.Errors[qr.asn] = qr.err
			result.FailureCount++
		} else {
			result.Results[qr.asn] = qr.info
			result.SuccessCount++
		}
	}

	return result
}

// extractRIRFromHandle 从RDAP Handle中提取RIR信息
func extractRIRFromHandle(handle string) string {
	handle = strings.ToUpper(handle)
	suffixes := map[string]string{
		"-ARIN":    "ARIN",
		"-RIPE":    "RIPE NCC",
		"-AP":      "APNIC",
		"-LACNIC":  "LACNIC",
		"-AFRINIC": "AFRINIC",
	}

	for suffix, rir := range suffixes {
		if strings.HasSuffix(handle, suffix) {
			return rir
		}
	}

	return ""
}

// GetASNDetailCache 获取ASN缓存
func GetASNDetailCache() map[int]*ASNDetail {
	asnDetailCache.mu.RLock()
	defer asnDetailCache.mu.RUnlock()

	result := make(map[int]*ASNDetail, len(asnDetailCache.items))
	for k, v := range asnDetailCache.items {
		result[k] = v
	}
	return result
}

// ClearASNDetailCache 清除ASN缓存
func ClearASNDetailCache() {
	asnDetailCache.mu.Lock()
	defer asnDetailCache.mu.Unlock()
	asnDetailCache.items = make(map[int]*ASNDetail)
}

// ParseASNString 从字符串解析ASN号码
// 支持格式: "AS12345", "as12345", "12345"
func ParseASNString(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToUpper(s), "AS")
	return strconv.Atoi(s)
}

// ASNToPrefixCount 统计ASN的IP前缀数量
func ASNToPrefixCount(info *ASNDetail) (ipv4Count, ipv6Count int) {
	if info == nil {
		return 0, 0
	}
	return len(info.IPv4Prefixes), len(info.IPv6Prefixes)
}

// ASNRelation ASN关系图节点
type ASNRelation struct {
	// 中心ASN
	ASN int `json:"asn"`

	// 上游AS (提供transit的AS)
	Upstream []ASNPeer `json:"upstream,omitempty"`

	// 下游AS (接收transit的AS)
	Downstream []ASNPeer `json:"downstream,omitempty"`

	// 对等AS (peering关系)
	Peers []ASNPeer `json:"peers,omitempty"`
}

// ASNPeer ASN对等关系
type ASNPeer struct {
	// ASN号码
	ASN int `json:"asn"`

	// AS名称
	Name string `json:"name,omitempty"`

	// 关系来源
	Source string `json:"source,omitempty"`
}
