package main

import (
	"bytes"
	"strings"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// runCacheCmd 构造 cache 命令树，执行给定子命令参数并返回 stdout 与 error。
// 使用独立的新 cache cmd 树，避免共享 root 的 PersistentPreRun（后者会尝试加载
// config/proxies 等文件并打印无关日志），同时把输出定向到 buffer。
func runCacheCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newCacheCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// fillCacheEntry 向全局缓存写入一条域名缓存条目，用于 cache get 的命中分支。
func fillCacheEntry(t *testing.T, domain, registrar string, raw string) {
	t.Helper()
	whois.GetCache().Set(domain, &whoisparser.WhoisInfo{
		Domain:    &whoisparser.Domain{Domain: domain},
		Registrar: &whoisparser.Contact{Name: registrar},
	}, raw)
}

// clearGlobalCache 清空全局缓存并清理过期条目，确保每个 cache 子命令测试有干净起点。
func clearGlobalCache(t *testing.T) {
	t.Helper()
	c := whois.GetCache()
	c.Clear()
	c.ClearExpired()
	whois.ClearASNDetailCache()
}

// --- newCacheCmd ---

// TestCacheCmd_Help 验证 cache 命令组本身可执行（无子命令走 help）。
func TestCacheCmd_Help(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t)
	assert.NoError(t, err)
	// cobra 无 RunE 时输出 Usage，包含子命令列表。
	assert.True(t, strings.Contains(out, "stats") || strings.Contains(out, "Usage"))
}

// --- newCacheStatsCmd ---

// TestCacheStats_Text 已启用缓存的文本摘要输出（含 hit_rate 分支）。
func TestCacheStats_Text(t *testing.T) {
	clearGlobalCache(t)
	// 注入一条记录并触发一次命中，使 requests/hits 非零，覆盖 hit_rate 输出分支。
	fillCacheEntry(t, "stats.example.com", "StatsRegistrar", "raw-stats")
	_, _ = whois.GetCache().Get("stats.example.com") // 命中
	out, err := runCacheCmd(t, "stats")
	assert.NoError(t, err)
	assert.Contains(t, out, "缓存: 已启用")
	assert.Contains(t, out, "类型: local")
}

// TestCacheStats_JSON --json 输出结构化统计。
func TestCacheStats_JSON(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t, "stats", "--json")
	assert.NoError(t, err)
	// JSON 输出应包含 type 字段
	assert.Contains(t, out, `"type"`)
}

// --- newCacheGetCmd ---

// TestCacheGet_NotFound 未命中分支（输出“缓存中未找到”）。
func TestCacheGet_NotFound(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t, "get", "not-exist.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "缓存中未找到")
}

// TestCacheGet_Text 命中文本输出，含 Domain 与 Registrar 字段。
func TestCacheGet_Text(t *testing.T) {
	clearGlobalCache(t)
	fillCacheEntry(t, "get.example.com", "GetRegistrar", "raw-get")
	out, err := runCacheCmd(t, "get", "get.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "域名: get.example.com")
	assert.Contains(t, out, "注册商: GetRegistrar")
	assert.Contains(t, out, "原始响应长度")
}

// TestCacheGet_JSON 命中且 --json 输出完整 WhoisInfo 的 JSON。
func TestCacheGet_JSON(t *testing.T) {
	clearGlobalCache(t)
	fillCacheEntry(t, "getjson.example.com", "JsonRegistrar", "raw-json")
	out, err := runCacheCmd(t, "get", "getjson.example.com", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"cached_at"`)
}

// TestCacheGet_BadInfo 仅 RawResponse、Info 为 nil 的边界（Domain/Registrar 不打印）。
func TestCacheGet_BadInfo(t *testing.T) {
	clearGlobalCache(t)
	// 写一条无 Info 的条目（直接构造 CacheEntry 入池）。
	whois.GetCache().Set("noinfo.example.com", nil, "raw-no-info")
	out, err := runCacheCmd(t, "get", "noinfo.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "域名: noinfo.example.com")
	// 不应输出注册商行
	assert.NotContains(t, out, "注册商:")
}

// --- newCacheDeleteCmd ---

// TestCacheDelete 删除存在的条目。
func TestCacheDelete(t *testing.T) {
	clearGlobalCache(t)
	fillCacheEntry(t, "del.example.com", "DelRegistrar", "raw-del")
	out, err := runCacheCmd(t, "delete", "del.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "已删除缓存条目: del.example.com")
	// 验证确实被删除（get 命中失败）
	out2, _ := runCacheCmd(t, "get", "del.example.com")
	assert.Contains(t, out2, "缓存中未找到")
}

// TestCacheDelete_NotExist 删除不存在的条目（Delete 为幂等，仍输出提示）。
func TestCacheDelete_NotExist(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t, "delete", "ghost.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "已删除缓存条目: ghost.example.com")
}

// --- newCacheClearCmd ---

// TestCacheClear 清空全部缓存。
func TestCacheClear(t *testing.T) {
	clearGlobalCache(t)
	fillCacheEntry(t, "c1.example.com", "R", "r1")
	out, err := runCacheCmd(t, "clear")
	assert.NoError(t, err)
	assert.Contains(t, out, "已清空全部缓存")
	// 验证确实清空
	out2, _ := runCacheCmd(t, "get", "c1.example.com")
	assert.Contains(t, out2, "缓存中未找到")
}

// --- newCacheClearExpiredCmd ---

// TestCacheClearExpired 清理过期缓存条目（含 LocalCache.clearExpired 调用分支）。
func TestCacheClearExpired(t *testing.T) {
	clearGlobalCache(t)
	fillCacheEntry(t, "notexpired.example.com", "R", "r1")
	out, err := runCacheCmd(t, "clear-expired")
	assert.NoError(t, err)
	assert.Contains(t, out, "已清理过期缓存条目")
}

// --- newCacheASNCmd ---

// TestCacheASN_ListEmpty ASN 详情缓存为空分支。
func TestCacheASN_ListEmpty(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t, "asn", "list")
	assert.NoError(t, err)
	assert.Contains(t, out, "ASN 详情缓存为空")
}

// TestCacheASN_Clear 清空 ASN 详情缓存。
func TestCacheASN_Clear(t *testing.T) {
	clearGlobalCache(t)
	out, err := runCacheCmd(t, "asn", "clear")
	assert.NoError(t, err)
	assert.Contains(t, out, "已清空 ASN 详情缓存")
}

// TestCacheASN_UnknownSubcommand 未知子命令：asn 命令组无 RunE，
// cobra 输出建议而非报错，验证 stdout 含 Usage/可用子命令。
func TestCacheASN_UnknownSubcommand(t *testing.T) {
	clearGlobalCache(t)
	out, _ := runCacheCmd(t, "asn", "nope")
	// cobra 未识别子命令时输出 Usage（含 list/clear 建议）
	assert.True(t, strings.Contains(out, "list") || strings.Contains(out, "Usage"))
}
