package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"
)

// ============================================================
// Fake providers —— 让 handler 调用的 whois 包级函数返回错误，
// 从而离线覆盖各 query/export handler 的 500（查询失败）分支。
// 每个 provider 都返回 errQueryFailed，无状态、无网络。
// ============================================================

var errQueryFailed = errors.New("fake provider: query failed")

// failingWhoisProvider 实现 whois.WhoisQueryProvider，Query/Parse 均返回错误。
// whois 系 handler（whois/diff/quality/export/correlation/availability）通过
// ExecuteQueryWithResult[Context] 间接调用该全局 provider，注入后可离线
// 覆盖这些 handler 的 500（查询失败）分支。
type failingWhoisProvider struct{}

func (failingWhoisProvider) Query(_ context.Context, _, _ string, _ bool) (string, error) {
	return "", errQueryFailed
}
func (failingWhoisProvider) Parse(_ string) (whoisparser.WhoisInfo, error) {
	return whoisparser.WhoisInfo{}, errQueryFailed
}

// injectFailingWhoisProvider 注入 failing 域名 provider，返回恢复函数
func injectFailingWhoisProvider() func() {
	whois.SetWhoisQueryProvider(failingWhoisProvider{})
	return func() {
		whois.SetWhoisQueryProvider(nil)
	}
}

// ============================================================
// WHOIS 核心查询 —— 500 分支（查询失败）
// ============================================================

func TestHandleWhoisQuery_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080) // EnableMetrics=false，避免 result==nil 时 panic
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, postJSON("/api/whois", map[string]interface{}{
		"domain": "example.com",
		"timeout": 1, "max_retries": 1,
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询失败")
}

// 注：handleIPQuery / handleASNQuery / handleRDAP*Query 调用的
// QueryIPWithOptions / QueryASNWithContext / QueryRDAP*WithContext 直接走
// 网络（IANA bootstrap + HTTP/whois），不走可注入的 Provider 接口，
// 其 500（查询失败）分支需真实网络故障才能触发，未在此离线覆盖。

// ============================================================
// 域名分析 —— 查询失败分支
// ============================================================

func TestHandleAvailabilityCheck_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleAvailabilityCheck(rr, postJSON("/api/availability", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "可用性检查失败")
}

func TestHandleDiff_FirstQueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleDiff(rr, postJSON("/api/diff", map[string]interface{}{
		"domain1": "example.com", "domain2": "test.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询 example.com 失败")
}

func TestHandleQuality_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleQuality(rr, postJSON("/api/quality", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询失败")
}

// handleCorrelation 在查询失败时会 log+continue（不返回500），最终返回 Analyze 结果。
// 这里用 failing provider 触发 continue 分支 + Analyze 成功路径。
func TestHandleCorrelation_AllQueriesFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleCorrelation(rr, postJSON("/api/correlation", map[string]interface{}{
		"domains": []string{"a.invalid", "b.invalid"},
	}))
	// 所有查询失败 -> continue -> Analyze 空结果 -> 200
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	assert.True(t, resp.Success)
}

// ============================================================
// 导出 handler —— 查询失败 500 分支
// ============================================================

func TestHandleExportJSON_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportJSON(rr, postJSON("/api/export/json", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询失败")
}

func TestHandleExportCSV_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportCSV(rr, postJSON("/api/export/csv", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询失败")
}

func TestHandleExportMarkdown_QueryFailed(t *testing.T) {
	restore := injectFailingWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportMarkdown(rr, postJSON("/api/export/markdown", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "查询失败")
}

// ============================================================
// Success fake —— 让 whois 系 handler 的 happy-path 可离线覆盖
// （Query 返回占位 raw，Parse 返回有效 WhoisInfo，无网络）
// ============================================================

// successWhoisProvider 返回成功结果的 provider
type successWhoisProvider struct{}

func (successWhoisProvider) Query(_ context.Context, domain, _ string, _ bool) (string, error) {
	return "Domain: " + domain + "\nRegistrar: Test Registrar\n", nil
}

func (successWhoisProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	// 从 raw 中提取域名（简单实现，足够让下游纯函数处理）
	domain := ""
	if idx := indexOf(raw, "Domain: "); idx >= 0 {
		rest := raw[idx+len("Domain: "):]
		if end := indexOf(rest, "\n"); end >= 0 {
			domain = rest[:end]
		} else {
			domain = rest
		}
	}
	return whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         domain,
			Name:           domain,
			WhoisServer:    "whois.example.net",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com", "ns2.example.com"},
			CreatedDate:    "2020-01-01T00:00:00Z",
			UpdatedDate:    "2023-01-01T00:00:00Z",
			ExpirationDate: "2025-01-01T00:00:00Z",
		},
		Registrar: &whoisparser.Contact{
			Name: "Test Registrar",
		},
	}, nil
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func injectSuccessWhoisProvider() func() {
	whois.SetWhoisQueryProvider(successWhoisProvider{})
	return func() {
		whois.SetWhoisQueryProvider(nil)
	}
}

// ============================================================
// whois 系 handler happy-path（查询成功）
// ============================================================

func TestHandleWhoisQuery_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, postJSON("/api/whois", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, decodeAPI(t, rr).Success)
}

func TestHandleWhoisQuery_SuccessWithMetrics(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	s.EnableMetrics = true // 覆盖 metrics 记录分支（result 非 nil，不会 panic）
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, postJSON("/api/whois", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, decodeAPI(t, rr).Success)
}

func TestHandleAvailabilityCheck_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleAvailabilityCheck(rr, postJSON("/api/availability", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	assert.True(t, resp.Success)
}

func TestHandleDiff_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleDiff(rr, postJSON("/api/diff", map[string]interface{}{
		"domain1": "example.com", "domain2": "test.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	m := resp.Data.(map[string]interface{})
	assert.Equal(t, "example.com", m["domain1"])
	assert.Equal(t, "test.com", m["domain2"])
	// count 字段应存在（changes 可能为空切片，两个 info 结构高度相似）
	assert.Contains(t, m, "count")
}

func TestHandleQuality_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleQuality(rr, postJSON("/api/quality", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, decodeAPI(t, rr).Success)
}

func TestHandleCorrelation_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleCorrelation(rr, postJSON("/api/correlation", map[string]interface{}{
		"domains": []string{"a.com", "b.com"},
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, decodeAPI(t, rr).Success)
}

func TestHandleExportJSON_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportJSON(rr, postJSON("/api/export/json", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.NotEmpty(t, rr.Body.String())
}

func TestHandleExportCSV_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportCSV(rr, postJSON("/api/export/csv", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/csv", rr.Header().Get("Content-Type"))
	assert.NotEmpty(t, rr.Body.String())
}

func TestHandleExportMarkdown_Success(t *testing.T) {
	restore := injectSuccessWhoisProvider()
	defer restore()

	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportMarkdown(rr, postJSON("/api/export/markdown", map[string]interface{}{
		"domain": "example.com",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "text/markdown", rr.Header().Get("Content-Type"))
	assert.NotEmpty(t, rr.Body.String())
}

// ============================================================
// IP / RDAP-ASN 的 500 分支 —— 通过输入校验失败离线触发（无网络）
// （QueryIP* 对无效 IP 在 net.ParseIP 阶段即返回错误；
//  QueryRDAP_ASN* 对无效 ASN 在 extractASNNumber 阶段即返回错误）
// ============================================================

func TestHandleIPQuery_InvalidIP(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// 非法 IP 格式：QueryIPWithContext 内 net.ParseIP 失败，直接返回错误，无网络
	s.handleIPQuery(rr, postJSON("/api/ip", map[string]interface{}{
		"ip": "not.a.valid.ip.address",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "IP查询失败")
}

func TestHandleRDAPASNQuery_InvalidASN(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// 非法 ASN：extractASNNumber 返回 0，直接返回错误，无网络
	s.handleRDAPASNQuery(rr, postJSON("/api/rdap/asn", map[string]interface{}{
		"asn": "not-an-asn",
	}))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "RDAP ASN查询失败")
}
