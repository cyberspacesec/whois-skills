package whois

import (
	"context"
	"fmt"
	"net"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

// IPWhoisResult IP WHOIS查询结果
type IPWhoisResult struct {
	// IP地址
	IP string `json:"ip"`

	// 原始响应
	RawResponse string `json:"raw_response"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 使用的服务器
	Server string `json:"server"`

	// 查询耗时（毫秒）
	Latency int64 `json:"latency"`

	// 解析后的信息（如果可用）
	Info *whoisparser.WhoisInfo `json:"info,omitempty"`
}

// IPWhoisOptions IP WHOIS查询选项
type IPWhoisOptions struct {
	// IP地址
	IP string `json:"ip"`

	// 超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// 是否使用代理
	UseProxy bool `json:"use_proxy,omitempty"`
}

// QueryIP 查询IP地址的WHOIS信息
func QueryIP(ip string) (*IPWhoisResult, error) {
	return QueryIPWithContext(context.Background(), &IPWhoisOptions{IP: ip})
}

// QueryIPWithOptions 使用选项查询IP地址的WHOIS信息
func QueryIPWithOptions(opts *IPWhoisOptions) (*IPWhoisResult, error) {
	return QueryIPWithContext(context.Background(), opts)
}

// QueryIPWithContext 使用上下文查询IP地址的WHOIS信息
func QueryIPWithContext(ctx context.Context, opts *IPWhoisOptions) (*IPWhoisResult, error) {
	if opts == nil || opts.IP == "" {
		return nil, fmt.Errorf("IP地址不能为空")
	}

	ip := net.ParseIP(opts.IP)
	if ip == nil {
		return nil, fmt.Errorf("无效的IP地址: %s", opts.IP)
	}

	if opts.Timeout <= 0 {
		opts.Timeout = 10
	}

	startTime := time.Now()

	// Step 1: 查询IANA以获取对应的RIR服务器
	client := NewWhoisClient()
	if opts.UseProxy {
		client.pool = GetProxyPool()
	}
	client.SetTimeout(time.Duration(opts.Timeout) * time.Second)

	ianaResponse, err := client.rawWhoisQuery(ctx, "whois.iana.org", opts.IP)
	if err != nil {
		return nil, fmt.Errorf("查询IANA失败: %w", err)
	}

	// Step 2: 从IANA响应中提取RIR服务器
	rirServer := extractReferralServer(ianaResponse)
	if rirServer == "" {
		// 返回IANA响应
		result := &IPWhoisResult{
			IP:          opts.IP,
			RawResponse: ianaResponse,
			QueryTime:   startTime,
			Server:      "whois.iana.org",
			Latency:     time.Since(startTime).Milliseconds(),
		}
		info, err := whoisparser.Parse(ianaResponse)
		if err == nil {
			result.Info = &info
		}
		return result, nil
	}

	// Step 3: 查询RIR服务器
	response, err := client.rawWhoisQuery(ctx, rirServer, opts.IP)
	if err != nil {
		return nil, fmt.Errorf("查询RIR服务器失败: %w", err)
	}

	result := &IPWhoisResult{
		IP:          opts.IP,
		RawResponse: response,
		QueryTime:   startTime,
		Server:      rirServer,
		Latency:     time.Since(startTime).Milliseconds(),
	}

	// 尝试解析
	info, err := whoisparser.Parse(response)
	if err == nil {
		result.Info = &info
	}

	return result, nil
}
