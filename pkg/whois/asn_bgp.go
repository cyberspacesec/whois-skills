package whois

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// ASN BGP 关系查询
//
// BGP AS 关系数据（上游 provider、下游 customer、对等 peer）来自 RouteViews
// 的 AS 关系文件（格式：from_as|to_as|relationship，-1=provider→customer，
// 0=peer）。本模块提供 ASNRelationProvider 接口与本地文件实现，上层可注入
// 其他数据源（如 BGPstream API、商业数据库）。
//
// 默认实现 LocalASNRelationProvider 解析 RouteViews 格式文件。
// ============================================================================

// ASNRelationType AS 关系类型。
type ASNRelationType int

const (
	// ASRelationProvider to_as 是 from_as 的上游 provider（from_as 向 to_as 付费 Transit）
	ASRelationProvider ASNRelationType = -1

	// ASRelationPeer to_as 与 from_as 是对等 peer（互免 Transit）
	ASRelationPeer ASNRelationType = 0

	// ASRelationCustomer to_as 是 from_as 的下游 customer（向 from_as 付费 Transit）
	ASRelationCustomer ASNRelationType = 1
)

// ASNRelationEdge 表示一条 AS 关系边（from→to，带类型）。
// 与已有的 ASNRelation（图节点聚合）区分。
type ASNRelationEdge struct {
	FromASN int             `json:"from_asn"`
	ToASN   int             `json:"to_asn"`
	Type    ASNRelationType `json:"type"`
}

// ASNRelationProvider AS 关系查询提供者接口。
type ASNRelationProvider interface {
	// QueryRelations 查询指定 ASN 的所有关系（返回上游/下游/对等）。
	QueryRelations(ctx context.Context, asn int) (*ASNRelations, error)
	// Close 关闭数据源（如有）。
	Close() error
}

// ASNRelations ASN 的所有 BGP 关系汇总。
type ASNRelations struct {
	ASN           int              `json:"asn"`
	UpstreamASNs  []int            `json:"upstream_asns,omitempty"`  // provider（本 AS 向它们付费）
	DownstreamASNs []int           `json:"downstream_asns,omitempty"` // customer（向本 AS 付费）
	PeerASNs      []int            `json:"peer_asns,omitempty"`      // 对等（互免 Transit）
	Relations     []ASNRelationEdge `json:"relations,omitempty"`     // 原始关系边列表
	QueryTime     time.Time        `json:"query_time"`
	Source        string           `json:"source"`
}

// ASNRelationConfig AS 关系数据源配置。
type ASNRelationConfig struct {
	// 是否启用 BGP 关系查询
	Enabled bool `json:"enabled"`

	// 数据源类型 (local/api)
	Type string `json:"type"`

	// 本地文件路径（type=local）
	FilePath string `json:"file_path,omitempty"`

	// 刷新间隔（秒，type=local 时定期重载文件）
	RefreshInterval int `json:"refresh_interval,omitempty"`
}

// ---- 全局 ASNRelationProvider ----

var globalASNRelationProvider ASNRelationProvider

// GetASNRelationProvider 返回全局 AS 关系提供者。
func GetASNRelationProvider() ASNRelationProvider {
	return globalASNRelationProvider
}

// SetASNRelationProvider 注入自定义 AS 关系提供者。
func SetASNRelationProvider(p ASNRelationProvider) {
	globalASNRelationProvider = p
}

// InitASNRelationFromConfig 从配置初始化全局 ASNRelationProvider。
func InitASNRelationFromConfig(cfg *ASNRelationConfig) error {
	if !cfg.Enabled {
		globalASNRelationProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		p, err := NewLocalASNRelationProvider(cfg.FilePath)
		if err != nil {
			return err
		}
		globalASNRelationProvider = p
		return nil
	default:
		return fmt.Errorf("未知 AS 关系数据源类型: %s", cfg.Type)
	}
}

// ---- LocalASNRelationProvider 本地文件实现 ----

// LocalASNRelationProvider 解析 RouteViews AS 关系文件。
// 文件格式：<from_as>|<to_as>|<relationship>|<source>
// relationship: -1 = provider→customer, 0 = peer
type LocalASNRelationProvider struct {
	mu       sync.RWMutex
	filePath string
	// 关系索引：asn → 该 ASN 作为 from_as 的所有关系边
	fromIndex map[int][]ASNRelationEdge
	// 关系索引：asn → 该 ASN 作为 to_as 的所有关系边
	toIndex   map[int][]ASNRelationEdge
	// 数据加载时间
	loadTime  time.Time
	// 定期刷新（若配置）
	refreshInterval int
	stopRefresh     chan struct{}
}

// NewLocalASNRelationProvider 创建本地文件 AS 关系提供者。
func NewLocalASNRelationProvider(filePath string) (*LocalASNRelationProvider, error) {
	if filePath == "" {
		filePath = "data/as-rel.txt"
	}
	p := &LocalASNRelationProvider{
		filePath: filePath,
		fromIndex: make(map[int][]ASNRelationEdge),
		toIndex:   make(map[int][]ASNRelationEdge),
	}
	if err := p.loadFile(); err != nil {
		logrus.Warnf("加载 AS 关系文件失败: %v（后续查询返回空关系）", err)
	}
	return p, nil
}

// loadFile 解析 AS 关系文件并构建索引。
func (p *LocalASNRelationProvider) loadFile() error {
	f, err := os.Open(p.filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 清空索引
	p.fromIndex = make(map[int][]ASNRelationEdge)
	p.toIndex = make(map[int][]ASNRelationEdge)

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 格式: from_as|to_as|relationship|source
		// relationship: -1=provider→customer, 0=peer
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			logrus.Debugf("跳过格式错误的行 %d: %s", lineNum, line)
			continue
		}
		fromASN, err1 := strconv.Atoi(parts[0])
		toASN, err2 := strconv.Atoi(parts[1])
		relType, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			logrus.Debugf("跳过解析错误的行 %d: %s", lineNum, line)
			continue
		}
		rel := ASNRelationEdge{
			FromASN: fromASN,
			ToASN:   toASN,
			Type:    ASNRelationType(relType),
		}
		p.fromIndex[fromASN] = append(p.fromIndex[fromASN], rel)
		p.toIndex[toASN] = append(p.toIndex[toASN], rel)
	}
	p.loadTime = time.Now()
	logrus.Infof("已加载 %d 条 AS 关系（%d from-index, %d to-index）",
		lineNum, len(p.fromIndex), len(p.toIndex))
	return scanner.Err()
}

// QueryRelations 查询指定 ASN 的所有关系。
func (p *LocalASNRelationProvider) QueryRelations(ctx context.Context, asn int) (*ASNRelations, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := &ASNRelations{
		ASN:       asn,
		QueryTime: time.Now(),
		Source:    "local:" + p.filePath,
	}

	// 作为 from_as 的关系：本 ASN 主动发起的关系
	for _, rel := range p.fromIndex[asn] {
		result.Relations = append(result.Relations, rel)
		switch rel.Type {
		case ASRelationProvider:
			// from_as 向 to_as 付费 Transit → to_as 是 from_as 的上游 provider
			result.UpstreamASNs = append(result.UpstreamASNs, rel.ToASN)
		case ASRelationPeer:
			// 对等关系
			result.PeerASNs = append(result.PeerASNs, rel.ToASN)
		}
	}

	// 作为 to_as 的关系：其他 ASN 主动发起与本 ASN 的关系
	for _, rel := range p.toIndex[asn] {
		result.Relations = append(result.Relations, rel)
		switch rel.Type {
		case ASRelationProvider:
			// from_as 向 to_as 付费 Transit → 本 ASN（to_as）是 from_as 的上游 provider
			// 反过来：from_as 是本 ASN 的下游 customer
			result.DownstreamASNs = append(result.DownstreamASNs, rel.FromASN)
		case ASRelationPeer:
			// 对等关系（已在 fromIndex 中处理，这里可能重复，去重）
			if !containsInt(result.PeerASNs, rel.FromASN) {
				result.PeerASNs = append(result.PeerASNs, rel.FromASN)
			}
		}
	}

	return result, nil
}

// Close 停止定期刷新。
func (p *LocalASNRelationProvider) Close() error {
	if p.stopRefresh != nil {
		close(p.stopRefresh)
	}
	return nil
}

// containsInt 检查 int 是否在 slice 中。
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// ---- 在 QueryASNWithContext 中集成 BGP 关系查询 ----

// queryASNRelations 若 IncludeBGP=true 且全局 provider 已注入，查询并填充 BGP 关系。
func queryASNRelations(ctx context.Context, opts *ASNQueryOptions, info *ASNDetail) {
	if !opts.IncludeBGP {
		return
	}
	provider := GetASNRelationProvider()
	if provider == nil {
		logrus.Debugf("ASN BGP 关系查询未启用（未注入 ASNRelationProvider）")
		return
	}
	relations, err := provider.QueryRelations(ctx, opts.ASN)
	if err != nil {
		logrus.Warnf("查询 ASN %d BGP 关系失败: %v", opts.ASN, err)
		return
	}
	info.UpstreamASNs = relations.UpstreamASNs
	info.DownstreamASNs = relations.DownstreamASNs
	info.PeerASNs = relations.PeerASNs
}