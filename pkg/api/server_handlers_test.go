package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
	"github.com/stretchr/testify/assert"
)

// newIdleBatchProcessor 返回一个未启动 Process 的批量处理器，
// 供 handleBatchStatus 测试安全读取 GetStats（无并发写）。
func newIdleBatchProcessor() *whois.StreamBatchProcessor {
	cfg := whois.DefaultStreamBatchConfig()
	cfg.Timeout = 1
	cfg.MaxRetries = 1
	return whois.NewStreamBatchProcessor(cfg)
}

// ============================================================
// 辅助函数
// ============================================================

// postJSON 构造一个 POST 请求，body 为 v 的 JSON 编码
func postJSON(path string, v interface{}) *http.Request {
	b, _ := json.Marshal(v)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(b)))
	return req
}

// postRaw 构造一个带原始 body 的 POST 请求
func postRaw(path, body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
}

// getReq 构造一个 GET 请求
func getReq(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
}

// decodeAPI 将响应体解码为 APIResponse
func decodeAPI(t *testing.T, rr *httptest.ResponseRecorder) APIResponse {
	t.Helper()
	var resp APIResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	assert.NoError(t, err)
	return resp
}

// ============================================================
// WHOIS 核心查询 handler —— 仅覆盖错误分支（happy-path 需真实网络）
// ============================================================

func TestHandleWhoisQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, getReq("/api/whois"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "仅支持POST请求", decodeAPI(t, rr).Error)
}

func TestHandleWhoisQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, postRaw("/api/whois", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "无效的请求格式", decodeAPI(t, rr).Error)
}

func TestHandleWhoisQuery_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleWhoisQuery(rr, postJSON("/api/whois", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleIPQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIPQuery(rr, getReq("/api/ip"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleIPQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIPQuery(rr, postRaw("/api/ip", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "无效的请求格式", decodeAPI(t, rr).Error)
}

func TestHandleIPQuery_EmptyIP(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIPQuery(rr, postJSON("/api/ip", map[string]string{"ip": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "IP地址不能为空", decodeAPI(t, rr).Error)
}

func TestHandleASNQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleASNQuery(rr, getReq("/api/asn"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleASNQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleASNQuery(rr, postRaw("/api/asn", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleASNQuery_NonPositiveASN(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleASNQuery(rr, postJSON("/api/asn", map[string]int{"asn": 0}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "ASN必须为正整数", decodeAPI(t, rr).Error)
}

// ============================================================
// RDAP 查询 handler —— 错误分支
// ============================================================

func TestHandleRDAPDomainQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPDomainQuery(rr, getReq("/api/rdap/domain"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleRDAPDomainQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPDomainQuery(rr, postRaw("/api/rdap/domain", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRDAPDomainQuery_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPDomainQuery(rr, postJSON("/api/rdap/domain", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleRDAPIPQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPIPQuery(rr, getReq("/api/rdap/ip"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleRDAPIPQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPIPQuery(rr, postRaw("/api/rdap/ip", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRDAPIPQuery_EmptyIP(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPIPQuery(rr, postJSON("/api/rdap/ip", map[string]string{"ip": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "IP地址不能为空", decodeAPI(t, rr).Error)
}

func TestHandleRDAPASNQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPASNQuery(rr, getReq("/api/rdap/asn"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleRDAPASNQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPASNQuery(rr, postRaw("/api/rdap/asn", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleRDAPASNQuery_EmptyASN(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleRDAPASNQuery(rr, postJSON("/api/rdap/asn", map[string]string{"asn": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "ASN不能为空", decodeAPI(t, rr).Error)
}

// ============================================================
// 域名分析 handler —— 错误分支
// ============================================================

func TestHandleAvailabilityCheck_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleAvailabilityCheck(rr, getReq("/api/availability"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleAvailabilityCheck_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleAvailabilityCheck(rr, postRaw("/api/availability", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleAvailabilityCheck_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleAvailabilityCheck(rr, postJSON("/api/availability", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleDiff_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleDiff(rr, getReq("/api/diff"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleDiff_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleDiff(rr, postRaw("/api/diff", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleDiff_EmptyDomains(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// domain1 空
	s.handleDiff(rr, postJSON("/api/diff", map[string]string{"domain1": "", "domain2": "b.com"}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "两个域名都不能为空", decodeAPI(t, rr).Error)

	// domain2 空
	rr = httptest.NewRecorder()
	s.handleDiff(rr, postJSON("/api/diff", map[string]string{"domain1": "a.com", "domain2": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleQuality_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleQuality(rr, getReq("/api/quality"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleQuality_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleQuality(rr, postRaw("/api/quality", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleQuality_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleQuality(rr, postJSON("/api/quality", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleCorrelation_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleCorrelation(rr, getReq("/api/correlation"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleCorrelation_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleCorrelation(rr, postRaw("/api/correlation", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleCorrelation_NotEnoughDomains(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleCorrelation(rr, postJSON("/api/correlation", map[string][]string{"domains": {"a.com"}}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "至少需要2个域名进行关联分析", decodeAPI(t, rr).Error)
}

// ============================================================
// 批量查询 handler
// ============================================================

func TestHandleBatchQuery_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchQuery(rr, getReq("/api/batch"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleBatchQuery_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchQuery(rr, postRaw("/api/batch", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleBatchQuery_EmptyDomains(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchQuery(rr, postJSON("/api/batch", map[string][]string{"domains": {}}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名列表不能为空", decodeAPI(t, rr).Error)
}

// 注：handleBatchQuery 的成功路径（启动异步 Process）会触发 whois 包
// StreamBatchProcessor 的内部数据竞争与全局 provider 单例竞争，
// 属于需真实网络的 happy-path，未在此离线覆盖。其错误分支已由上方测试覆盖。

func TestHandleBatchStatus_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchStatus(rr, httptest.NewRequest(http.MethodPost, "/api/batch/status", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleBatchStatus_MissingID(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchStatus(rr, getReq("/api/batch/status"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "缺少会话ID参数", decodeAPI(t, rr).Error)
}

func TestHandleBatchStatus_SessionNotFound(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleBatchStatus(rr, getReq("/api/batch/status?id=nonexistent"))
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "会话不存在", decodeAPI(t, rr).Error)
}

func TestHandleBatchStatus_Success(t *testing.T) {
	s := NewServer("localhost", 8080)
	// 直接构造一个会话存入 store，避免触发异步 Process goroutine
	// （handleBatchQuery 总会启动真实网络查询的 goroutine，与之并发读 GetStats
	// 会触发 StreamBatchProcessor 的数据竞争——那是 whois 包的并发缺陷，本测试不负责触发）。
	processor := newIdleBatchProcessor()
	sessionID := "batch-test-idle"
	s.batchSessions.Store(sessionID, &batchSession{
		ID:        sessionID,
		Processor: processor,
		Domains:   []string{"example.invalid"},
		CreatedAt: time.Now(),
	})

	// 查询状态（此时无并发写，GetStats 安全）
	rr := httptest.NewRecorder()
	s.handleBatchStatus(rr, getReq("/api/batch/status?id="+sessionID))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	assert.True(t, resp.Success)
	m := resp.Data.(map[string]interface{})
	assert.Equal(t, sessionID, m["session_id"])
	assert.NotNil(t, m["stats"])
}

// ============================================================
// 格式化 handler —— 可完全离线覆盖（调纯函数）
// ============================================================

func TestHandleFormat_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleFormat(rr, getReq("/api/format"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleFormat_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleFormat(rr, postRaw("/api/format", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleFormat_EmptyRawResponse(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleFormat(rr, postJSON("/api/format", map[string]string{"raw_response": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "原始响应不能为空", decodeAPI(t, rr).Error)
}

func TestHandleFormat_DetectOnly(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleFormat(rr, postJSON("/api/format", map[string]interface{}{
		"raw_response": "Domain: example.com\nRegistrar: Test",
		"detect_only":  true,
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	m := resp.Data.(map[string]interface{})
	assert.NotEmpty(t, m["format"])
	// detect_only=true 时不应包含 formatted 字段
	_, hasFormatted := m["formatted"]
	assert.False(t, hasFormatted)
}

func TestHandleFormat_WithFormatting(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleFormat(rr, postJSON("/api/format", map[string]interface{}{
		"raw_response": "% Domain: example.com\nDomain: example.com\n\nRegistrar: Test",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	m := resp.Data.(map[string]interface{})
	assert.NotEmpty(t, m["format"])
	assert.NotNil(t, m["formatted"])
}

// ============================================================
// 导出 handler —— 仅覆盖错误分支（happy-path 需真实网络）
// ============================================================

func TestHandleExportJSON_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportJSON(rr, getReq("/api/export/json"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleExportJSON_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportJSON(rr, postRaw("/api/export/json", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleExportJSON_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportJSON(rr, postJSON("/api/export/json", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleExportCSV_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportCSV(rr, getReq("/api/export/csv"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleExportCSV_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportCSV(rr, postRaw("/api/export/csv", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleExportCSV_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportCSV(rr, postJSON("/api/export/csv", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleExportMarkdown_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportMarkdown(rr, getReq("/api/export/markdown"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleExportMarkdown_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportMarkdown(rr, postRaw("/api/export/markdown", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleExportMarkdown_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleExportMarkdown(rr, postJSON("/api/export/markdown", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ============================================================
// IDN handler —— 覆盖所有 action 分支
// ============================================================

func TestHandleIDN_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, getReq("/api/idn"))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandleIDN_BadJSON(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postRaw("/api/idn", "{invalid"))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleIDN_EmptyDomain(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{"domain": ""}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "域名不能为空", decodeAPI(t, rr).Error)
}

func TestHandleIDN_DefaultActionNormalize(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// 不传 action，应默认 normalize
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{"domain": "https://Example.COM."}))
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeAPI(t, rr).Data.(map[string]interface{})
	assert.Equal(t, "https://Example.COM.", m["original"])
	assert.Equal(t, "example.com", m["normalized"])
}

func TestHandleIDN_Normalize(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "example.com", "action": "normalize",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeAPI(t, rr).Data.(map[string]interface{})
	assert.Equal(t, "example.com", m["normalized"])
}

func TestHandleIDN_ToPunycode(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "例子.测试", "action": "to_punycode",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeAPI(t, rr).Data.(map[string]interface{})
	assert.Contains(t, m["punycode"].(string), "xn--")
	assert.True(t, m["is_idn"].(bool))
}

func TestHandleIDN_ToUnicode(t *testing.T) {
	s := NewServer("localhost", 8080)
	// 先得到 punycode
	rr1 := httptest.NewRecorder()
	s.handleIDN(rr1, postJSON("/api/idn", map[string]string{
		"domain": "例子.测试", "action": "to_punycode",
	}))
	puny := decodeAPI(t, rr1).Data.(map[string]interface{})["punycode"].(string)

	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": puny, "action": "to_unicode",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeAPI(t, rr).Data.(map[string]interface{})
	assert.Contains(t, m["unicode"].(string), "例子")
}

func TestHandleIDN_Check(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "例子.com", "action": "check",
	}))
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeAPI(t, rr).Data.(map[string]interface{})
	assert.True(t, m["is_idn"].(bool))
	// check 只返回 is_idn，不应有 normalized/punycode/unicode
	_, hasNorm := m["normalized"]
	assert.False(t, hasNorm)
}

func TestHandleIDN_NormalizeError(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// é.xn--- 非ASCII（会进入idna.ToASCII分支）且含非法punycode label，转换失败
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "é.xn---", "action": "normalize",
	}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "规范化失败")
}

func TestHandleIDN_InvalidAction(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "example.com", "action": "bogus",
	}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "无效的action")
}

func TestHandleIDN_ToPunycodeError(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	// xn--- 是非法 punycode label，转换会失败
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "xn---", "action": "to_punycode",
	}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "转换失败")
}

func TestHandleIDN_ToUnicodeError(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleIDN(rr, postJSON("/api/idn", map[string]string{
		"domain": "xn---", "action": "to_unicode",
	}))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, decodeAPI(t, rr).Error, "转换失败")
}

// ============================================================
// 服务器列表 handler —— 可离线覆盖
// ============================================================

func TestHandleServers_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleServers(rr, httptest.NewRequest(http.MethodPost, "/api/servers", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "仅支持GET请求", decodeAPI(t, rr).Error)
}

func TestHandleServers_Success(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleServers(rr, getReq("/api/servers"))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	m := resp.Data.(map[string]interface{})
	assert.NotNil(t, m["servers"])
	assert.NotNil(t, m["stats"])
}

// ============================================================
// 系统端点 handler
// ============================================================

func TestHandleHealth_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	rr := httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodPost, "/api/health", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "仅支持GET请求", decodeAPI(t, rr).Error)
}

func TestHandleMetrics_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	s.EnableMetrics = true
	rr := httptest.NewRecorder()
	s.handleMetrics(rr, httptest.NewRequest(http.MethodPost, "/api/metrics", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "仅支持GET请求", decodeAPI(t, rr).Error)
}

func TestHandleMetrics_Success(t *testing.T) {
	s := NewServer("localhost", 8080)
	s.EnableMetrics = true
	rr := httptest.NewRecorder()
	s.handleMetrics(rr, getReq("/api/metrics"))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
}

func TestHandleAlerts_Success(t *testing.T) {
	s := NewServer("localhost", 8080)
	s.EnableAlerts = true
	rr := httptest.NewRecorder()
	s.handleAlerts(rr, getReq("/api/alerts"))
	assert.Equal(t, http.StatusOK, rr.Code)
	resp := decodeAPI(t, rr)
	assert.True(t, resp.Success)
	// 告警历史初始为空数组
	assert.NotNil(t, resp.Data)
}

func TestHandleAlerts_MethodNotAllowed(t *testing.T) {
	s := NewServer("localhost", 8080)
	s.EnableAlerts = true
	rr := httptest.NewRecorder()
	s.handleAlerts(rr, httptest.NewRequest(http.MethodPost, "/api/alerts", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	assert.Equal(t, "仅支持GET请求", decodeAPI(t, rr).Error)
}

// ============================================================
// 路由与 handler 构造 —— 可离线覆盖
// ============================================================

func TestCreateHandler(t *testing.T) {
	s := NewServer("localhost", 8080)
	h := s.CreateHandler()
	assert.NotNil(t, h)

	// 验证路由确实注册了：访问 /api/health 应返回 200
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, getReq("/api/health"))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateRouter_RegistersAllRoutes(t *testing.T) {
	s := NewServer("localhost", 8080)
	router := s.createRouter()
	assert.NotNil(t, router)

	// 抽样验证若干路由可达
	paths := []string{
		"/api/whois", "/api/ip", "/api/asn",
		"/api/rdap/domain", "/api/rdap/ip", "/api/rdap/asn",
		"/api/availability", "/api/diff", "/api/quality", "/api/correlation",
		"/api/batch", "/api/batch/status",
		"/api/format", "/api/export/json", "/api/export/csv", "/api/export/markdown",
		"/api/idn", "/api/servers",
		"/api/metrics", "/api/alerts", "/api/health",
		// MCP 端点
		"/api/mcp/request_planning", "/api/mcp/get_next_task",
		"/api/mcp/list_requests",
	}
	for _, p := range paths {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, getReq(p))
		// 路由已注册：不应返回 404（NotFound）
		assert.NotEqual(t, http.StatusNotFound, rr.Code,
			"route %s should be registered", p)
	}
}

func TestRegisterMCPRoutes(t *testing.T) {
	s := NewServer("localhost", 8080)
	router := http.NewServeMux()
	// 不应 panic
	assert.NotPanics(t, func() {
		s.registerMCPRoutes(router)
	})

	// 每个 MCP 端点都应可调用（合法或错误响应，而非 404）
	mcpPaths := []string{
		"/api/mcp/request_planning", "/api/mcp/get_next_task",
		"/api/mcp/mark_task_done", "/api/mcp/approve_task_completion",
		"/api/mcp/approve_request_completion", "/api/mcp/open_task_details",
		"/api/mcp/list_requests", "/api/mcp/add_tasks_to_request",
		"/api/mcp/update_task", "/api/mcp/delete_task",
	}
	for _, p := range mcpPaths {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, postJSON(p, map[string]string{}))
		assert.NotEqual(t, http.StatusNotFound, rr.Code,
			"mcp route %s should be registered", p)
	}
}

// TestStart_BindFailure 验证 Start 在端口已被占用时立即返回错误，
// 而非阻塞。这覆盖了 Start 的全部语句（addr/日志/CreateHandler/ListenAndServe）。
func TestStart_BindFailure(t *testing.T) {
	// 先占用一个端口，保持 listener 不关闭
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	s := NewServer("127.0.0.1", port)

	// Start 应因端口已被占用而立即返回 error（不阻塞）
	done := make(chan error, 1)
	go func() {
		done <- s.Start()
	}()
	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Start 阻塞未返回，预期端口占用应立即失败")
	}
}
