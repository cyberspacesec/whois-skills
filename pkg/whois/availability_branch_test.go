package whois

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== availability.go isParserError 各分支 ====================
// ExecuteQueryWithResultContext 在 Query 阶段返回的错误若不可重试，会直接透传（不包装）。
// 用 stub provider 的 queryErr 返回原始 parser 错误，触发 isParserError 各分支。

// TestCheckDomainAvailabilityWithContext_NotFoundDomain Query 返回 ErrNotFoundDomain
// → available 状态（line 48-52）。
func TestCheckDomainAvailabilityWithContext_NotFoundDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		queryErr: whoisparser.ErrNotFoundDomain,
	})
	defer restore()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.Available)
	assert.Equal(t, "available", res.Status)
	assert.Equal(t, "域名可以注册", res.Message)
}

// TestCheckDomainAvailabilityWithContext_ReservedDomain Query 返回 ErrReservedDomain
// → reserved 状态（line 53-56）。
func TestCheckDomainAvailabilityWithContext_ReservedDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		queryErr: whoisparser.ErrReservedDomain,
	})
	defer restore()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "reserved", res.Status)
	assert.Equal(t, "域名已被保留", res.Message)
}

// TestCheckDomainAvailabilityWithContext_PremiumDomain Query 返回 ErrPremiumDomain
// → premium 状态（line 57-60）。
func TestCheckDomainAvailabilityWithContext_PremiumDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		queryErr: whoisparser.ErrPremiumDomain,
	})
	defer restore()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "premium", res.Status)
	assert.Equal(t, "域名可以溢价注册", res.Message)
}

// TestCheckDomainAvailabilityWithContext_BlockedDomain Query 返回 ErrBlockedDomain
// → blocked 状态（line 61-64）。
func TestCheckDomainAvailabilityWithContext_BlockedDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		queryErr: whoisparser.ErrBlockedDomain,
	})
	defer restore()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "blocked", res.Status)
	assert.Equal(t, "域名已被品牌保护屏蔽", res.Message)
}

// TestCheckDomainAvailabilityWithContext_DomainLimitExceed 注：ErrDomainLimitExceed 的
// .Error() 含 "limit exceeded"，CheckError 会映射为 ErrRateLimited（可重试），重试耗尽后
// 返回 ErrServerConnectFailed，不会命中 isParserError(ErrDomainLimitExceed) 分支。
// 故 line 65-68 对当前 CheckError 实现不可达，记录为死代码。

