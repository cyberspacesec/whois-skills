package whois

import "context"

// ReverseWhoisProvider 反向WHOIS查询提供者接口
type ReverseWhoisProvider interface {
	// SearchByRegistrant 根据注册人搜索域名
	SearchByRegistrant(ctx context.Context, query string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error)

	// SearchByEmail 根据邮箱搜索域名
	SearchByEmail(ctx context.Context, email string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error)

	// SearchByOrganization 根据组织搜索域名
	SearchByOrganization(ctx context.Context, org string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error)

	// Name 返回提供者名称
	Name() string
}

// ReverseWhoisOptions 反向WHOIS查询选项
type ReverseWhoisOptions struct {
	// 最大返回数量
	Limit int `json:"limit,omitempty"`

	// 是否包含过期域名
	IncludeExpired bool `json:"include_expired,omitempty"`
}

// ReverseWhoisResult 反向WHOIS查询结果
type ReverseWhoisResult struct {
	// 域名
	Domain string `json:"domain"`

	// 注册人
	Registrant string `json:"registrant,omitempty"`

	// 邮箱
	Email string `json:"email,omitempty"`

	// 组织
	Organization string `json:"organization,omitempty"`

	// 创建日期
	CreationDate string `json:"creation_date,omitempty"`

	// 过期日期
	ExpirationDate string `json:"expiration_date,omitempty"`

	// 注册商
	Registrar string `json:"registrar,omitempty"`
}

// ReverseWhoisClient 反向WHOIS查询客户端
type ReverseWhoisClient struct {
	provider ReverseWhoisProvider
}

// NewReverseWhoisClient 创建反向WHOIS查询客户端
func NewReverseWhoisClient(provider ReverseWhoisProvider) *ReverseWhoisClient {
	return &ReverseWhoisClient{provider: provider}
}

// SearchByRegistrant 根据注册人搜索域名
func (c *ReverseWhoisClient) SearchByRegistrant(ctx context.Context, query string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return c.provider.SearchByRegistrant(ctx, query, opts)
}

// SearchByEmail 根据邮箱搜索域名
func (c *ReverseWhoisClient) SearchByEmail(ctx context.Context, email string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return c.provider.SearchByEmail(ctx, email, opts)
}

// SearchByOrganization 根据组织搜索域名
func (c *ReverseWhoisClient) SearchByOrganization(ctx context.Context, org string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return c.provider.SearchByOrganization(ctx, org, opts)
}

// ProviderName 返回当前提供者名称
func (c *ReverseWhoisClient) ProviderName() string {
	if c.provider != nil {
		return c.provider.Name()
	}
	return "none"
}
