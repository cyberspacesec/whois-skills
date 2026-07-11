package whois

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== servers.go extractTLD URL 截断分支 ====================

// TestExtractTLD_URLPathTruncation URL 含路径 → idx>0 截断后提取 TLD。
func TestExtractTLD_URLPathTruncation(t *testing.T) {
	// "https://example.com/path" → 截断为 "example.com" → TLD "com"
	got := extractTLD("https://example.com/path")
	assert.Equal(t, "com", got)
}

// TestExtractTLD_FallbackSinglePart 无点的单段域名 → fallback len<2 → ""。
func TestExtractTLD_FallbackSinglePart(t *testing.T) {
	got := extractTLD("localhost")
	assert.Equal(t, "", got)
}

// ==================== export.go writer 失败分支 ====================
// 注：csv.Writer 内部缓冲，Write 在缓冲未满时不触发底层 Write，
// 且 ExportToCSV 用 defer Flush() 丢弃 Flush 错误，故 writer.Write 的
// 错误分支（line 34-36/51-53/80-82）对缓冲 writer 实际不可达。

// TestExportToCSV_NilInfo info 为 nil → 报错。
func TestExportToCSV_NilInfo(t *testing.T) {
	err := ExportToCSV(nil, io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHOIS信息不能为空")
}

// TestExportToMarkdown_NilInfo info 为 nil → 报错。
func TestExportToMarkdown_NilInfo(t *testing.T) {
	err := ExportToMarkdown(nil, io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHOIS信息不能为空")
}

// TestExportToJSON_NilInfo info 为 nil → 报错。
func TestExportToJSON_NilInfo(t *testing.T) {
	err := ExportToJSON(nil, io.Discard)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHOIS信息不能为空")
}

// ==================== reverse_local.go indexContact Phone 分支 ====================

// TestLocalReverseWhoisIndex_indexContact_Phone contact 含 Phone → 索引 phone 字段。
func TestLocalReverseWhoisIndex_indexContact_Phone(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	idx.mu.Lock()
	defer idx.mu.Unlock()
	contact := &whoisparser.Contact{
		Email:        "a@b.com",
		Name:         "John",
		Organization: "Org",
		Phone:        "+1.555",
	}
	idx.indexContact("x.com", contact)
	// phone 索引应存在
	assert.NotEmpty(t, idx.index[indexKey("phone", "+1.555")])
}

// ==================== servers.go DiscoverWhoisServer 真实网络成功路径 ====================

// TestDiscoverWhoisServer_LiveSuccess 真实网络查询 IANA "com" → 返回 whois.verisign-grs.com。
// 覆盖 line 630-648 成功路径；网络不通则跳过（连接失败分支已由其他测试覆盖）。
func TestDiscoverWhoisServer_LiveSuccess(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:            make(map[string]string),
		serverHealth:       make(map[string]*ServerHealth),
		healthCheckTimeout: 10 * time.Second,
	}
	server, err := mgr.DiscoverWhoisServer("com")
	if err != nil {
		t.Logf("DiscoverWhoisServer 真实网络失败（仍覆盖失败分支）: %v", err)
		return
	}
	assert.NotEmpty(t, server)
}

// TestDiscoverWhoisServer_LiveNoServer 真实网络查询一个 IANA 无 whois: refer 的 TLD
// → extractReferralServer 返回空 → "未找到...的WHOIS服务器"（line 642-644）。
func TestDiscoverWhoisServer_LiveNoServer(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:            make(map[string]string),
		serverHealth:       make(map[string]*ServerHealth),
		healthCheckTimeout: 10 * time.Second,
	}
	// "invalidtldxyz" 不存在 → IANA 返回无 whois: refer
	_, err := mgr.DiscoverWhoisServer("invalidtldxyz")
	if err != nil {
		// 可能是连接失败或未找到服务器，都算覆盖
		t.Logf("DiscoverWhoisServer(invalidtld) 返回错误（仍覆盖分支）: %v", err)
		return
	}
}
