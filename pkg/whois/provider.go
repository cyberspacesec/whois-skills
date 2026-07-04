package whois

import (
	"context"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// ============================================================================
// Provider 接口体系
//
// 本包核心查询路径（域名/IP/ASN/RDAP）历史上硬编码 likexian/whois 与
// likexian/whois-parser。为支持上层替换数据源或解析器，将这些能力抽象为
// Provider 接口，likexian 实现降级为默认 provider。
//
// 上层可通过 Set*Provider 注入自定义实现，影响后续全部查询。
// ============================================================================

// WhoisQueryProvider 域名 WHOIS 查询与解析的提供者接口。
//
// Query 返回指定域名在指定 WHOIS 服务器上的原始响应文本；
// Parse 将原始文本解析为结构化 WhoisInfo。
//
// 默认实现 DefaultWhoisQueryProvider 走 likexian/whois + whois-parser。
type WhoisQueryProvider interface {
	// Query 向 server 查询 domain 的原始 WHOIS 文本。
	// server 为空时由实现自行选择默认服务器。
	Query(ctx context.Context, domain, server string, useProxy bool) (string, error)
	// Parse 将原始 WHOIS 文本解析为结构化信息。
	Parse(raw string) (whoisparser.WhoisInfo, error)
}

// IPWhoisProvider IP WHOIS 查询提供者接口。
//
// 默认实现走 IANA bootstrap → RIR referral 两步查询（ipwhois.go）。
type IPWhoisProvider interface {
	// QueryIP 返回 IP 的结构化 WHOIS 信息与原始响应。
	QueryIP(ctx context.Context, ip string, opts *IPWhoisOptions) (*IPWhoisResult, error)
}

// ASNProvider ASN 查询提供者接口。
//
// 默认实现走 RADB + RDAP 双源（asn_enhanced.go）。
type ASNProvider interface {
	// QueryASN 返回 ASN 详情。
	QueryASN(ctx context.Context, opts *ASNQueryOptions) (*ASNDetail, error)
}

// RDAPProvider RDAP 查询提供者接口。
//
// 默认实现走内置 bootstrap + HTTP（rdap.go）。各方法返回对应的具体结果类型，
// 与 QueryRDAP*WithContext 系列函数一一对应。
type RDAPProvider interface {
	// QueryDomain 查询域名的 RDAP 记录。
	QueryDomain(ctx context.Context, opts *RDAPQueryOptions) (*RDAPResult, error)
	// QueryIP 查询 IP 的 RDAP 记录。
	QueryIP(ctx context.Context, opts *RDAPQueryOptions) (*RDAPIPResult, error)
	// QueryASN 查询 ASN 的 RDAP 记录。
	QueryASN(ctx context.Context, opts *RDAPQueryOptions) (*RDAPASNResult, error)
	// QueryEntity 查询实体的 RDAP 记录。
	QueryEntity(ctx context.Context, opts *RDAPQueryOptions) (*RDAPEntityResult, error)
}

// ---- 全局 Provider 注入 ----

var (
	globalWhoisQueryProvider WhoisQueryProvider
	globalIPWhoisProvider    IPWhoisProvider
	globalASNProvider        ASNProvider
	globalRDAPProvider       RDAPProvider
)

// GetWhoisQueryProvider 返回全局域名 WHOIS 查询 provider（懒加载默认实现）。
func GetWhoisQueryProvider() WhoisQueryProvider {
	if globalWhoisQueryProvider == nil {
		globalWhoisQueryProvider = &DefaultWhoisQueryProvider{}
	}
	return globalWhoisQueryProvider
}

// SetWhoisQueryProvider 注入自定义域名 WHOIS 查询 provider。
// 传 nil 恢复默认实现。
func SetWhoisQueryProvider(p WhoisQueryProvider) {
	if p == nil {
		globalWhoisQueryProvider = &DefaultWhoisQueryProvider{}
		return
	}
	globalWhoisQueryProvider = p
}

// GetIPWhoisProvider 返回全局 IP WHOIS provider。
func GetIPWhoisProvider() IPWhoisProvider {
	if globalIPWhoisProvider == nil {
		globalIPWhoisProvider = &DefaultIPWhoisProvider{}
	}
	return globalIPWhoisProvider
}

// SetIPWhoisProvider 注入自定义 IP WHOIS provider。
func SetIPWhoisProvider(p IPWhoisProvider) {
	if p == nil {
		globalIPWhoisProvider = &DefaultIPWhoisProvider{}
		return
	}
	globalIPWhoisProvider = p
}

// GetASNProvider 返回全局 ASN provider。
func GetASNProvider() ASNProvider {
	if globalASNProvider == nil {
		globalASNProvider = &DefaultASNProvider{}
	}
	return globalASNProvider
}

// SetASNProvider 注入自定义 ASN provider。
func SetASNProvider(p ASNProvider) {
	if p == nil {
		globalASNProvider = &DefaultASNProvider{}
		return
	}
	globalASNProvider = p
}

// GetRDAPProvider 返回全局 RDAP provider。
func GetRDAPProvider() RDAPProvider {
	if globalRDAPProvider == nil {
		globalRDAPProvider = &DefaultRDAPProvider{}
	}
	return globalRDAPProvider
}

// SetRDAPProvider 注入自定义 RDAP provider。
func SetRDAPProvider(p RDAPProvider) {
	if p == nil {
		globalRDAPProvider = &DefaultRDAPProvider{}
		return
	}
	globalRDAPProvider = p
}

// DefaultWhoisQueryProvider 基于 likexian/whois + whois-parser 的默认实现。
type DefaultWhoisQueryProvider struct{}

// Query 向 server 查询 domain 的原始 WHOIS 文本。
func (p *DefaultWhoisQueryProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	// server 参数保留供未来按服务器定向查询使用；当前 likexian 库内部自行选择服务器。
	_ = server
	if useProxy {
		return DirectWhoisWithContext(ctx, domain)
	}
	// whois.Whois 不支持 context，用 goroutine + select 模拟，与原 executeQueryWithTimeout 一致。
	type r struct {
		resp string
		err  error
	}
	ch := make(chan r, 1)
	go func() {
		resp, err := whois.Whois(domain)
		ch <- r{resp, err}
	}()
	select {
	case <-ctx.Done():
		return "", NewWhoisError(ErrQueryTimeout, "查询超时", ctx.Err())
	case res := <-ch:
		return res.resp, res.err
	}
}

// Parse 将原始 WHOIS 文本解析为结构化信息。
func (p *DefaultWhoisQueryProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	return whoisparser.Parse(raw)
}

// DefaultIPWhoisProvider 默认 IP WHOIS provider（走现有 QueryIPWithContext 逻辑）。
type DefaultIPWhoisProvider struct{}

// QueryIP 实现 IPWhoisProvider。
func (p *DefaultIPWhoisProvider) QueryIP(ctx context.Context, ip string, opts *IPWhoisOptions) (*IPWhoisResult, error) {
	if opts == nil {
		opts = &IPWhoisOptions{}
	}
	opts.IP = ip
	return QueryIPWithContext(ctx, opts)
}

// DefaultASNProvider 默认 ASN provider（走现有 QueryASNWithContext 逻辑）。
type DefaultASNProvider struct{}

// QueryASN 实现 ASNProvider。
func (p *DefaultASNProvider) QueryASN(ctx context.Context, opts *ASNQueryOptions) (*ASNDetail, error) {
	return QueryASNWithContext(ctx, opts)
}

// DefaultRDAPProvider 默认 RDAP provider（走现有 QueryRDAP* 逻辑）。
type DefaultRDAPProvider struct{}

// QueryDomain 实现 RDAPProvider。
func (p *DefaultRDAPProvider) QueryDomain(ctx context.Context, opts *RDAPQueryOptions) (*RDAPResult, error) {
	return QueryRDAPWithContext(ctx, opts)
}

// QueryIP 实现 RDAPProvider。
func (p *DefaultRDAPProvider) QueryIP(ctx context.Context, opts *RDAPQueryOptions) (*RDAPIPResult, error) {
	return QueryRDAP_IPWithContext(ctx, opts)
}

// QueryASN 实现 RDAPProvider。
func (p *DefaultRDAPProvider) QueryASN(ctx context.Context, opts *RDAPQueryOptions) (*RDAPASNResult, error) {
	return QueryRDAP_ASNWithContext(ctx, opts)
}

// QueryEntity 实现 RDAPProvider。
func (p *DefaultRDAPProvider) QueryEntity(ctx context.Context, opts *RDAPQueryOptions) (*RDAPEntityResult, error) {
	return QueryRDAP_EntityWithContext(ctx, opts)
}
