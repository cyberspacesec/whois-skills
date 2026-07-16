package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// runProxyCmd 构造 proxy 命令树执行给定参数，返回 stdout 与 error。
func runProxyCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newProxyCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// writeProxyFile 写一份代理配置 JSON（[]*ProxyConfig）。
func writeProxyFile(t *testing.T, name string, configs []*whois.ProxyConfig) string {
	t.Helper()
	data, err := json.Marshal(configs)
	require.NoError(t, err)
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, data, 0644))
	return p
}

// reloadProxies 清空代理池后从文件加载，确保每个测试有可预测状态。
// GetProxyPool 是单例，测试间共享，故每个用例显式重置。
func reloadProxies(t *testing.T, path string) {
	t.Helper()
	// 直接通过 LoadProxiesFromFile 覆盖单例池内容。
	require.NoError(t, whois.LoadProxiesFromFile(path))
	t.Cleanup(func() {
		// 测试结束写一份空代理文件恢复池为空，避免污染后续测试。
		empty := filepath.Join(t.TempDir(), "empty.json")
		_ = os.WriteFile(empty, []byte("[]"), 0644)
		_ = whois.LoadProxiesFromFile(empty)
	})
}

// --- newProxyCmd ---

// TestProxyCmd_NoSub 无子命令输出 Usage。
func TestProxyCmd_NoSub(t *testing.T) {
	out, err := runProxyCmd(t)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(out, "list") || strings.Contains(out, "Usage"))
}

// --- newProxyListCmd ---

// TestProxyList_Empty 代理池为空分支。
func TestProxyList_Empty(t *testing.T) {
	// 用空数组重置池
	empty := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(empty, []byte("[]"), 0644))
	require.NoError(t, whois.LoadProxiesFromFile(empty))
	out, err := runProxyCmd(t, "list")
	assert.NoError(t, err)
	assert.Contains(t, out, "代理池为空")
}

// TestProxyList_WithProxies 有代理的文本输出（含 ✅/❌ 状态）。
func TestProxyList_WithProxies(t *testing.T) {
	p := writeProxyFile(t, "proxies.json", []*whois.ProxyConfig{
		{Address: "127.0.0.1:1080", Type: "socks5", Enabled: true, Timeout: 5},
		{Address: "127.0.0.1:8080", Type: "http", Enabled: true, Timeout: 5},
	})
	reloadProxies(t, p)
	out, err := runProxyCmd(t, "list")
	assert.NoError(t, err)
	assert.Contains(t, out, "代理池（共")
	// 至少出现一个代理地址
	assert.Contains(t, out, "127.0.0.1:1080")
}

// TestProxyList_JSON --json 输出结构化代理统计。
func TestProxyList_JSON(t *testing.T) {
	p := writeProxyFile(t, "proxies.json", []*whois.ProxyConfig{
		{Address: "127.0.0.1:1080", Type: "socks5", Enabled: true, Timeout: 5},
	})
	reloadProxies(t, p)
	out, err := runProxyCmd(t, "list", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"proxies"`)
}

// --- newProxyStatsCmd ---

// TestProxyStats_Text 文本统计。
func TestProxyStats_Text(t *testing.T) {
	p := writeProxyFile(t, "proxies.json", []*whois.ProxyConfig{
		{Address: "127.0.0.1:1080", Type: "socks5", Enabled: true, Timeout: 5},
	})
	reloadProxies(t, p)
	out, err := runProxyCmd(t, "stats")
	assert.NoError(t, err)
	assert.Contains(t, out, "代理总数")
	assert.Contains(t, out, "可用代理")
}

// TestProxyStats_JSON JSON 统计。
func TestProxyStats_JSON(t *testing.T) {
	p := writeProxyFile(t, "proxies.json", []*whois.ProxyConfig{
		{Address: "127.0.0.1:1080", Type: "socks5", Enabled: true, Timeout: 5},
	})
	reloadProxies(t, p)
	out, err := runProxyCmd(t, "stats", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"total"`)
}

// --- newProxySetCmd ---

// TestProxySet_Socks5 socks5 代理成功设置。
func TestProxySet_Socks5(t *testing.T) {
	out, err := runProxyCmd(t, "set", "127.0.0.1:1080", "--type", "socks5")
	assert.NoError(t, err)
	assert.Contains(t, out, "已设置全局代理")
	assert.Contains(t, out, "socks5://127.0.0.1:1080")
}

// TestProxySet_HTTP http 代理成功设置（带认证）。
func TestProxySet_HTTP(t *testing.T) {
	out, err := runProxyCmd(t, "set", "proxy.example.com:8080", "--type", "http", "--user", "alice", "--pass", "secret", "--timeout", "15")
	assert.NoError(t, err)
	assert.Contains(t, out, "http://proxy.example.com:8080")
}

// TestProxySet_UnsupportedType 不支持的代理类型应报错（GetDialer 失败）。
func TestProxySet_UnsupportedType(t *testing.T) {
	_, err := runProxyCmd(t, "set", "127.0.0.1:1080", "--type", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "创建代理拨号器失败")
}
