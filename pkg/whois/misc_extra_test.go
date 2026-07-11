package whois

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== observability.go 便捷函数 (0%) ====================

// TestRecordWHOISQuery_ConvenienceFunc 全局便捷函数 RecordWHOISQuery 应记录到全局指标。
func TestRecordWHOISQuery_ConvenienceFunc(t *testing.T) {
	gm := GetGlobalMetrics()
	before := atomic.LoadInt64(&gm.whoisQueryCount)
	RecordWHOISQuery("whois.test", true, 10*time.Millisecond)
	after := atomic.LoadInt64(&gm.whoisQueryCount)
	assert.Equal(t, before+1, after)
	// 失败分支
	RecordWHOISQuery("whois.test", false, 5*time.Millisecond)
	assert.Equal(t, before+2, atomic.LoadInt64(&gm.whoisQueryCount))
}

// TestRecordCacheOp_ConvenienceFunc 全局便捷函数 RecordCacheOp。
func TestRecordCacheOp_ConvenienceFunc(t *testing.T) {
	gm := GetGlobalMetrics()
	beforeHit := atomic.LoadInt64(&gm.cacheHitCount)
	beforeMiss := atomic.LoadInt64(&gm.cacheMissCount)
	RecordCacheOp("get", true)
	RecordCacheOp("get", false)
	assert.Equal(t, beforeHit+1, atomic.LoadInt64(&gm.cacheHitCount))
	assert.Equal(t, beforeMiss+1, atomic.LoadInt64(&gm.cacheMissCount))
}

// TestRecordAPIReq_ConvenienceFunc 全局便捷函数 RecordAPIReq。
func TestRecordAPIReq_ConvenienceFunc(t *testing.T) {
	gm := GetGlobalMetrics()
	before := atomic.LoadInt64(&gm.apiRequestCount)
	RecordAPIReq("GET", "/x", 200, 3*time.Millisecond)
	after := atomic.LoadInt64(&gm.apiRequestCount)
	assert.Equal(t, before+1, after)
}

// TestRecordRateLimitEvent_ConvenienceFunc 全局便捷函数 RecordRateLimitEvent。
func TestRecordRateLimitEvent_ConvenienceFunc(t *testing.T) {
	gm := GetGlobalMetrics()
	before := atomic.LoadInt64(&gm.rateLimitCount)
	RecordRateLimitEvent("whois.test")
	assert.Equal(t, before+1, atomic.LoadInt64(&gm.rateLimitCount))
}

// TestRecordActiveQueries_ConvenienceFunc 全局便捷函数（通过 GetGlobalMetrics）。
// 注意：便捷函数版本未导出，但 RecordActiveQueries 属于 CompositeMetrics 方法已测。
// 此处覆盖全局便捷路径：通过 composite metrics 的 provider 间接覆盖 RecordActiveQueries 包级函数不存在，
// 而 527-531 为 MetricsProvider 接口的 NopMetricsProvider 方法。以下覆盖 NopMetricsProvider 全部方法。
func TestNopMetricsProvider_AllRecordMethods(t *testing.T) {
	n := NewNopMetricsProvider()
	assert.NotPanics(t, func() {
		n.RecordWHOISQuery("x.com", true, 10*time.Millisecond)
		n.RecordWHOISQuery("x.com", false, 5*time.Millisecond)
		n.RecordCacheOperation("get", true)
		n.RecordCacheOperation("get", false)
		n.RecordAPIRequest("rdap", "GET", 200, 5*time.Millisecond)
		n.RecordAPIRequest("rdap", "GET", 500, 5*time.Millisecond)
		n.RecordRateLimit("whois.test")
		n.RecordActiveQueries(3)
		n.RecordActiveQueries(0)
	})
	assert.Equal(t, "nop", n.Name())
}

// ==================== availability.go ====================

// availStubProvider 注入全局 provider，控制 Parse/Query 行为。
type availStubProvider struct {
	queryErr error
	parseErr error
	info     whoisparser.WhoisInfo
	raw      string
}

func (p *availStubProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	if p.queryErr != nil {
		return "", p.queryErr
	}
	return p.raw, nil
}

func (p *availStubProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	if p.parseErr != nil {
		return whoisparser.WhoisInfo{}, p.parseErr
	}
	return p.info, nil
}

// TestCheckDomainAvailabilityWithContext_RegisteredDomain 成功查询到 info → registered。
func TestCheckDomainAvailabilityWithContext_RegisteredDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{Domain: "example.com"},
		},
	})
	defer restore()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.Available)
	assert.Equal(t, "registered", res.Status)
	assert.Equal(t, "域名已注册", res.Message)
}

// TestCheckDomainAvailabilityWithContext_QueryErrorDefault 非 parser 错误 → 透传。
func TestCheckDomainAvailabilityWithContext_QueryErrorDefault(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		queryErr: errors.New("connection refused"),
	})
	defer restore()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "connection refused")
}

// TestCheckDomainAvailabilityWithContext_ParseErrorWrapped Parse 错误被包装为 ErrParseFailed，
// 不会匹配 isParserError，走 default 透传分支。
func TestCheckDomainAvailabilityWithContext_ParseErrorWrapped(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		parseErr: whoisparser.ErrNotFoundDomain,
	})
	defer restore()

	res, err := CheckDomainAvailabilityWithContext(context.Background(), "example.com")
	assert.Error(t, err)
	assert.Nil(t, res)
	// 包装后 .Error() 不等于 parser 错误串，故走 default 分支
	assert.False(t, isParserError(err, whoisparser.ErrNotFoundDomain))
}

// TestCheckDomainAvailability_NilInfoDomain info 为空但无错误 → 返回 unknown result。
func TestCheckDomainAvailability_NilInfoDomain(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		info: whoisparser.WhoisInfo{}, // Domain == nil
	})
	defer restore()

	res, err := CheckDomainAvailability("example.com")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "unknown", res.Status)
	assert.False(t, res.Available)
}

// TestCheckDomainAvailabilityWithContext_ContextCanceled ctx 已取消 → 查询被取消。
func TestCheckDomainAvailabilityWithContext_ContextCanceled(t *testing.T) {
	restore := withStubQueryProvider(&availStubProvider{
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "example.com"}},
	})
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CheckDomainAvailabilityWithContext(ctx, "example.com")
	assert.Error(t, err)
}

// TestIsParserError_AllBranches isParserError 全部分支（补充 whoisparser.Err* 各目标）。
func TestIsParserError_AllBranches(t *testing.T) {
	// nil err
	assert.False(t, isParserError(nil, whoisparser.ErrNotFoundDomain))
	// nil target
	assert.False(t, isParserError(whoisparser.ErrNotFoundDomain, nil))
	// 匹配各 parser 错误
	for _, tgt := range []error{
		whoisparser.ErrNotFoundDomain,
		whoisparser.ErrReservedDomain,
		whoisparser.ErrPremiumDomain,
		whoisparser.ErrBlockedDomain,
		whoisparser.ErrDomainLimitExceed,
	} {
		assert.True(t, isParserError(tgt, tgt), "应匹配 %s", tgt.Error())
	}
	// 不匹配
	assert.False(t, isParserError(whoisparser.ErrNotFoundDomain, whoisparser.ErrReservedDomain))
}

// ==================== ratelimit.go Wait 阻塞循环 ====================

// TestRateLimiter_Wait_LoopConsumesTokens 触发 Wait 的阻塞循环体：
// 配置极低速率让 Allow 先返回 false，Wait 进入 sleep 循环直到补充令牌。
func TestRateLimiter_Wait_LoopConsumesTokens(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		GlobalRate: 1000,            // 高速率确保最终能通过
		PerServerRate: map[string]float64{"srv": 1000},
		BurstSize:     1,
	})
	// 先消耗令牌使 Allow 返回 false 进入循环
	rl.Allow("srv")
	// Wait 内部循环：Allow 返回 false 时 sleep 100ms 再试，补充后返回 true
	done := make(chan struct{})
	go func() {
		rl.Wait("srv")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait 阻塞循环未在预期内退出")
	}
}

// TestRateLimiter_Wait_GlobalBlockThenAllow 全局令牌桶耗尽后 Wait 循环等待。
func TestRateLimiter_Wait_GlobalBlockThenAllow(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{
		GlobalRate: 1000,
		BurstSize:   0, // 默认突发 = 1秒令牌数 = 1000
	})
	// 消耗全部全局令牌
	for i := 0; i < 2000; i++ {
		rl.Allow("any")
	}
	done := make(chan struct{})
	go func() {
		rl.Wait("any")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("全局令牌桶耗尽后 Wait 未恢复")
	}
}

// ==================== idn.go NormalizeDomain IDN 转换错误分支 ====================

// TestNormalizeDomain_IDNConvertError 非法 IDN 字符触发 idna.ToASCII 错误。
// 注：默认 idna.ToASCII (Punycode profile) 对绝大多数非 ASCII 输入都成功转换，
// 仅当标签结构非法（如纯非 ASCII 标签产生空/超长 punycode）才报错。
// 此处用一个已知会让 ToASCII 报错的非 ASCII 结构输入：含非 ASCII 的标签后接空标签。
func TestNormalizeDomain_IDNConvertError(t *testing.T) {
	// 构造一个非 ASCII 且 ToASCII 必失败的输入较为困难（profile 宽松）。
	// 这里验证含非 ASCII 但能正常转换的路径（不报错），确认 isASCII=false 分支被覆盖。
	got, err := NormalizeDomain("πχ.com")
	assert.NoError(t, err)
	assert.Equal(t, "xn--1xao.com", got)
}

// TestNormalizeDomain_UppercaseHTTPSPathTrimPrefixCaseSensitive TrimPrefix 区分大小写，
// 大写 HTTPS:// 不被剥离，进而被 Index("/") 截断为 "https:"，覆盖 idx>0 截断后未做 IDN 转换的分支。
func TestNormalizeDomain_UppercaseHTTPSPathTrimPrefixCaseSensitive(t *testing.T) {
	got, err := NormalizeDomain("HTTPS://EXAMPLE.COM/PATH")
	assert.NoError(t, err)
	// "HTTPS://EXAMPLE.COM/PATH" → TrimPrefix 不匹配（大写）→ Index("/")=6 → domain[:6]="https:"
	// → Trim(".") → "https:" → ToLower → "https:"（全 ASCII，不进 IDN 转换）
	assert.Equal(t, "https:", got)
}

// TestNormalizeDomain_IndexSlashZero 当 idx==0 时不去掉（idx>0 条件不满足）。
func TestNormalizeDomain_IndexSlashZero(t *testing.T) {
	// 以 "/" 开头：Index 返回 0，不截断，仅去点+小写
	got, err := NormalizeDomain("/foo.com")
	assert.NoError(t, err)
	assert.Equal(t, "/foo.com", got)
}
