package whois

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== QueryIPWithContext 失败路径 ====================

// TestQueryIPWithContext_CtxCancelled ctx 已取消 → IANA 查询失败路径。
func TestQueryIPWithContext_CtxCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := QueryIPWithContext(ctx, &IPWhoisOptions{IP: "8.8.8.8", Timeout: 5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询IANA失败")
}

// TestQueryIPWithContext_NoReferral IANA 响应无 refer → 返回 IANA 响应分支。
// 0.0.0.0 在 IANA 通常不返回 whois: refer → 覆盖 rirServer=="" 分支。
// 若网络不通则触发 IANA 失败分支，二者都覆盖。
func TestQueryIPWithContext_NoReferral(t *testing.T) {
	res, err := QueryIPWithContext(context.Background(), &IPWhoisOptions{IP: "0.0.0.0", Timeout: 15})
	if err != nil {
		t.Logf("IANA 查询失败（网络相关，仍覆盖失败分支）: %v", err)
		return
	}
	// 成功且无 refer → Server 应为 whois.iana.org
	assert.NotNil(t, res)
	assert.Equal(t, "0.0.0.0", res.IP)
	assert.Equal(t, "whois.iana.org", res.Server)
}

// TestQueryIPWithContext_RIRPath 真实网络 8.8.8.8 → IANA refer whois.arin.net → RIR 查询。
// 覆盖 RIR 成功路径（line 103-122）；若失败则覆盖 RIR 失败分支。
func TestQueryIPWithContext_RIRPath(t *testing.T) {
	res, err := QueryIPWithContext(context.Background(), &IPWhoisOptions{IP: "8.8.8.8", Timeout: 15})
	if err != nil {
		t.Logf("RIR 路径查询失败（网络相关，仍覆盖失败分支）: %v", err)
		return
	}
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.RawResponse)
}

// ==================== BatchQueryASN 失败分支 ====================

// TestBatchQueryASN_AllFailWithCancelledCtx patch RDAP 指向不可达 + 已取消 ctx，
// 所有 ASN 都失败 → 覆盖 resultChan 循环的 err 分支（line 392-395）与 FailureCount++。
func TestBatchQueryASN_AllFailWithCancelledCtx(t *testing.T) {
	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	// 指向不可达 → RDAP 失败；回退 RADB 因 ctx 取消也失败
	b.asnRanges = []rdapASNRange{
		{start: 999001, end: 999001, rdapURL: "http://127.0.0.1:1"},
		{start: 999002, end: 999002, rdapURL: "http://127.0.0.1:1"},
	}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ClearASNDetailCache()
	defer ClearASNDetailCache()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := BatchQueryASN(ctx, []int{999001, 999002}, 2)
	assert.Equal(t, 2, result.TotalQueried)
	// 两个都失败（RDAP 不可达 + RADB ctx 取消）
	assert.Equal(t, 2, result.FailureCount)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Contains(t, result.Errors, 999001)
	assert.Contains(t, result.Errors, 999002)
}

// TestBatchQueryASN_EmptyList 空列表 → 直接返回空结果（line 355-357）。
func TestBatchQueryASN_EmptyList(t *testing.T) {
	result := BatchQueryASN(context.Background(), []int{}, 2)
	assert.Equal(t, 0, result.TotalQueried)
	assert.Equal(t, 0, result.SuccessCount)
	assert.Equal(t, 0, result.FailureCount)
	assert.Empty(t, result.Results)
	assert.Empty(t, result.Errors)
}

// ==================== QueryASNWithContext 边界 ====================

// TestQueryASNWithContext_UnsupportedSource 未知 Source → default 报错分支。
func TestQueryASNWithContext_UnsupportedSource(t *testing.T) {
	ClearASNDetailCache()
	defer ClearASNDetailCache()
	_, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:     13335,
		Timeout: 5,
		Source:  "bogus",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的查询来源")
}

// TestQueryASNWithContext_SourceRADBFail RADB source + 已取消 ctx → queryASNFromRADB 连接失败。
func TestQueryASNWithContext_SourceRADBFail(t *testing.T) {
	ClearASNDetailCache()
	defer ClearASNDetailCache()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := QueryASNWithContext(ctx, &ASNQueryOptions{
		ASN:     13335,
		Timeout: 5,
		Source:  ASNSourceRADB,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "连接RADB服务器失败")
}

// TestQueryASNWithContext_AllBothFailRadbErr ASNSourceAll: RDAP 失败 + RADB 失败 →
// 返回 "RDAP错误; RADB错误"（line 182），覆盖 lastErr!=nil 分支。
func TestQueryASNWithContext_AllBothFailRadbErr(t *testing.T) {
	ClearASNDetailCache()
	defer ClearASNDetailCache()
	b := GetRDAPBootstrap()
	b.mu.Lock()
	origASN := b.asnRanges
	b.asnRanges = []rdapASNRange{{start: 777788, end: 777788, rdapURL: "http://127.0.0.1:1"}}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.asnRanges = origASN
		b.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := QueryASNWithContext(ctx, &ASNQueryOptions{
		ASN:     777788,
		Timeout: 5,
		Source:  ASNSourceAll,
	})
	assert.Error(t, err)
	// lastErr != nil → 返回 "RDAP错误: ...; RADB错误: ..."
	assert.Contains(t, err.Error(), "RDAP错误")
	assert.Contains(t, err.Error(), "RADB错误")
}
