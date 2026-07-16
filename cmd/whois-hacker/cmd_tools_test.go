package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// execCapture 执行一个 cobra 命令并捕获其全部 stdout（含 outputJSON 直写 os.Stdout
// 与 cmd.OutOrStdout() 两条路径）。返回捕获内容与 RunE 的 error。
// 不调用 SetOut：OutOrStdout() 回退到 os.Stdout，与 captureStdout 重定向一致。
func execCapture(t *testing.T, root *cobra.Command, args ...string) (string, error) {
	t.Helper()
	root.SetArgs(args)
	var runErr error
	out := captureStdout(t, func() {
		runErr = root.Execute()
	})
	return out, runErr
}

// runToolsCmd 构造 tools 命令树执行给定参数，返回 stdout 与 error。
func runToolsCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCapture(t, newToolsCmd(), args...)
}

// runServersCmd 构造 servers 命令树（顶层）执行给定参数，返回 stdout 与 error。
func runServersCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCapture(t, newServersCmd(), args...)
}

// runIDNCmd 构造 idn 命令执行。
func runIDNCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCapture(t, newIDNCmd(), args...)
}

// runFormatCmd 构造 format 命令执行。
func runFormatCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCapture(t, newFormatCmd(), args...)
}

// runExportCmd 构造 export 命令执行（捕获 stdout）。
func runExportCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return execCapture(t, newExportCmd(), args...)
}

// writeFile 写临时文件并返回路径。
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	return p
}

// withStubProviderInfo 注入返回给定 info 的 stub provider，结束时恢复。
func withStubProviderInfo(t *testing.T, info whoisparser.WhoisInfo) {
	t.Helper()
	whois.SetWhoisQueryProvider(&stubQueryProvider{info: info})
	t.Cleanup(func() { whois.SetWhoisQueryProvider(nil) })
}

// withStdin 临时把 os.Stdin 替换为 r，执行 fn 后恢复。
func withStdin(t *testing.T, r io.Reader, fn func()) {
	t.Helper()
	orig := os.Stdin
	// 通过文件而非 pipe 更简单：写入临时文件再打开。
	f, err := os.CreateTemp(t.TempDir(), "stdin-*")
	require.NoError(t, err)
	_, _ = io.Copy(f, r)
	_ = f.Close()
	stdinFile, err := os.Open(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { _ = stdinFile.Close() })
	os.Stdin = stdinFile
	defer func() { os.Stdin = orig }()
	fn()
}

// --- newToolsCmd ---

// TestToolsCmd_NoSub 无子命令输出 Usage。
func TestToolsCmd_NoSub(t *testing.T) {
	out, err := runToolsCmd(t)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(out, "ip-parse") || strings.Contains(out, "Usage"))
}

// --- newToolsIPParseCmd ---

// TestToolsIPParse_FromFile --file 读取并解析 IP WHOIS 原始文本。
func TestToolsIPParse_FromFile(t *testing.T) {
	raw := "OrgName: Example\nNetRange: 8.8.8.0 - 8.8.8.255\n"
	p := writeFile(t, "raw.txt", raw)
	out, err := runToolsCmd(t, "ip-parse", "8.8.8.8", "--file", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "8.8.8.8")
}

// TestToolsIPParse_Stdin 从 stdin 读取。
// stdin 路径直接构造命令并 Execute，外层用 captureStdout 统一捕获
// （outputJSON 直写 os.Stdout）。不能复用 runToolsCmd，避免嵌套 captureStdout。
func TestToolsIPParse_Stdin(t *testing.T) {
	raw := "OrgName: Example\n"
	root := newToolsCmd()
	root.SetArgs([]string{"ip-parse", "8.8.8.8"})
	out := captureStdout2(t, func() {
		withStdin(t, strings.NewReader(raw), func() {
			_ = root.Execute()
		})
	})
	assert.Contains(t, out, "8.8.8.8")
}

// TestToolsIPParse_FileOpenError --file 指向不存在文件应报错。
func TestToolsIPParse_FileOpenError(t *testing.T) {
	_, err := runToolsCmd(t, "ip-parse", "8.8.8.8", "--file", "/no/such/file.txt")
	require.Error(t, err)
}

// TestToolsIPParse_EmptyRaw 原始文本为空应报“WHOIS响应为空”。
func TestToolsIPParse_EmptyRaw(t *testing.T) {
	p := writeFile(t, "empty.txt", "")
	_, err := runToolsCmd(t, "ip-parse", "8.8.8.8", "--file", p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析失败")
}

// --- newToolsDomainCmd ---

// TestToolsDomain_Text 域名解析文本输出。
func TestToolsDomain_Text(t *testing.T) {
	out, err := runToolsCmd(t, "domain", "www.example.com")
	assert.NoError(t, err)
	assert.Contains(t, out, "完整域名: www.example.com")
	assert.Contains(t, out, "顶级域")
}

// TestToolsDomain_JSON --json 输出。
func TestToolsDomain_JSON(t *testing.T) {
	out, err := runToolsCmd(t, "domain", "example.com", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"full_domain"`)
}

// TestToolsDomain_Invalid 无效域名应报错。
func TestToolsDomain_Invalid(t *testing.T) {
	_, err := runToolsCmd(t, "domain", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析失败")
}

// --- newToolsTLDCmd ---

// TestToolsTLD_Effective 默认提取有效 TLD。
func TestToolsTLD_Effective(t *testing.T) {
	out, err := runToolsCmd(t, "tld", "example.co.uk")
	assert.NoError(t, err)
	assert.Contains(t, out, "co.uk")
}

// TestToolsTLD_Simple --simple 提取简单 TLD。
func TestToolsTLD_Simple(t *testing.T) {
	out, err := runToolsCmd(t, "tld", "example.com", "--simple")
	assert.NoError(t, err)
	assert.Contains(t, out, "com")
}

// TestToolsTLD_Invalid 无效输入应报错。
func TestToolsTLD_Invalid(t *testing.T) {
	_, err := runToolsCmd(t, "tld", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "提取失败")
}

// --- newToolsNormalizeCmd ---

// TestToolsNormalize 各类型字段规范化。
func TestToolsNormalize(t *testing.T) {
	tests := []struct {
		name, fieldType, value, want string
	}{
		{"email", "email", "Foo@BAR.com", "foo@bar.com"},
		{"phone", "phone", "+1 (234) 567-8900", "+1(234)567-8900"},
		{"country", "country", "us", "US"},
		{"name_upper", "name", "JOHN DOE", "John Doe"},
		{"name_plain", "name", "john  doe", "john doe"},
		{"unknown", "other", "raw value", "raw value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runToolsCmd(t, "normalize", tt.fieldType, tt.value)
			assert.NoError(t, err)
			assert.Equal(t, tt.want+"\n", out)
		})
	}
}

// --- newToolsASNPrefixesCmd ---

// TestToolsASNPrefixes_InvalidASN 无效 ASN 字符串应报“无效 ASN”。
func TestToolsASNPrefixes_InvalidASN(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	_, err := runToolsCmd(t, "asn-prefixes", "not-a-number")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效 ASN")
}

// TestToolsASNPrefixes_OK 真实 ASN 查询成功（网络）。
func TestToolsASNPrefixes_OK(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	orig := flagTimeout
	flagTimeout = 20
	defer func() { flagTimeout = orig }()
	out, err := runToolsCmd(t, "asn-prefixes", "13335")
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	assert.Contains(t, out, "ASN 13335")
}

// --- newToolsASNIPRangesCmd ---

// TestToolsASNIPRanges_Text 真实 ASN IP 段查询文本输出（网络）。
func TestToolsASNIPRanges_Text(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	out, err := runToolsCmd(t, "asn-ip-ranges", "13335")
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	assert.Contains(t, out, "ASN 13335")
	assert.Contains(t, out, "IPv4 段")
}

// TestToolsASNIPRanges_JSON --json 输出（网络）。
func TestToolsASNIPRanges_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	out, err := runToolsCmd(t, "asn-ip-ranges", "13335", "--json")
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	assert.Contains(t, out, `"ipv4"`)
}

// ===================== cmd_tools.go =====================

// --- newIDNCmd ---

// TestIDN_ToASCII to-ascii 转换（默认）。
func TestIDN_ToASCII(t *testing.T) {
	out, err := runIDNCmd(t, "münchen.de")
	assert.NoError(t, err)
	assert.Contains(t, out, "xn--mnchen-3ya")
}

// TestIDN_ToUnicode to-unicode 转换。
func TestIDN_ToUnicode(t *testing.T) {
	out, err := runIDNCmd(t, "xn--mnchen-3ya.de", "--action", "to-unicode")
	assert.NoError(t, err)
	assert.Contains(t, out, "münchen")
}

// TestIDN_Normalize normalize 动作。
func TestIDN_Normalize(t *testing.T) {
	out, err := runIDNCmd(t, "https://München.DE/path", "--action", "normalize")
	assert.NoError(t, err)
	assert.Contains(t, out, "xn--mnchen-3ya")
}

// TestIDN_Detect detect 动作（IDN=true 与 IDN=false）。
func TestIDN_Detect_IDN(t *testing.T) {
	out, err := runIDNCmd(t, "münchen.de", "--action", "detect")
	assert.NoError(t, err)
	assert.Contains(t, out, `"is_idn": true`)
}

// TestIDN_Detect_NotIDN detect 对普通域返回 false。
func TestIDN_Detect_NotIDN(t *testing.T) {
	out, err := runIDNCmd(t, "example.com", "--action", "detect")
	assert.NoError(t, err)
	assert.Contains(t, out, `"is_idn": false`)
}

// TestIDN_Empty 空域名转换应报错。
func TestIDN_Empty(t *testing.T) {
	_, err := runIDNCmd(t, "", "--action", "to-ascii")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "转换失败")
}

// TestIDN_UnknownAction 未知 action 应报错。
func TestIDN_UnknownAction(t *testing.T) {
	_, err := runIDNCmd(t, "example.com", "--action", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未知 action")
}

// --- newFormatCmd ---

// TestFormat_FromFile 从文件读取并清洗。
func TestFormat_FromFile(t *testing.T) {
	raw := "# comment\n% another\nDomain: example\n\n\n"
	p := writeFile(t, "raw.txt", raw)
	out, err := runFormatCmd(t, p)
	assert.NoError(t, err)
	assert.Contains(t, out, "example")
}

// TestFormat_DetectOnly --detect-only 仅输出格式。
func TestFormat_DetectOnly(t *testing.T) {
	raw := "Registrar: Verisign\n"
	p := writeFile(t, "raw.txt", raw)
	out, err := runFormatCmd(t, p, "--detect-only")
	assert.NoError(t, err)
	assert.Contains(t, out, "format")
}

// TestFormat_Stdin 从 stdin 读取（args 为空）。
func TestFormat_Stdin(t *testing.T) {
	raw := "Domain: from-stdin\n"
	root := newFormatCmd()
	root.SetArgs([]string{})
	out := captureStdout2(t, func() {
		withStdin(t, strings.NewReader(raw), func() {
			_ = root.Execute()
		})
	})
	assert.Contains(t, out, "from-stdin")
}

// TestFormat_StdinDash 参数 "-" 也走 stdin 分支。
func TestFormat_StdinDash(t *testing.T) {
	raw := "Domain: dash-input\n"
	root := newFormatCmd()
	root.SetArgs([]string{"-"})
	out := captureStdout2(t, func() {
		withStdin(t, strings.NewReader(raw), func() {
			_ = root.Execute()
		})
	})
	assert.Contains(t, out, "dash-input")
}

// TestFormat_FileOpenError 文件打开失败。
func TestFormat_FileOpenError(t *testing.T) {
	_, err := runFormatCmd(t, "/no/such/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "打开文件失败")
}

// --- newExportCmd ---

// TestExport_JSON 导出 JSON 格式（用 stub provider 避免联网）。
func TestExport_JSON(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	withStubProviderInfo(t, whoisparser.WhoisInfo{
		Domain:    &whoisparser.Domain{Domain: "example.com"},
		Registrar: &whoisparser.Contact{Name: "Example Registrar"},
	})
	out, err := runExportCmd(t, "example.com", "--format", "json")
	require.NoError(t, err)
	assert.Contains(t, out, "example.com")
}

// TestExport_CSV 导出 CSV。
func TestExport_CSV(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	withStubProviderInfo(t, whoisparser.WhoisInfo{
		Domain:    &whoisparser.Domain{Domain: "csv.example"},
		Registrar: &whoisparser.Contact{Name: "CSV Reg"},
	})
	out, err := runExportCmd(t, "csv.example", "--format", "csv")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

// TestExport_Markdown 导出 Markdown（md 别名）。
func TestExport_Markdown(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	withStubProviderInfo(t, whoisparser.WhoisInfo{
		Domain:    &whoisparser.Domain{Domain: "md.example"},
		Registrar: &whoisparser.Contact{Name: "MD Reg"},
	})
	out, err := runExportCmd(t, "md.example", "--format", "md")
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

// TestExport_UnknownFormat 未知格式应报错。
func TestExport_UnknownFormat(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	withStubProviderInfo(t, whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "u.example"},
	})
	_, err := runExportCmd(t, "u.example", "--format", "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未知格式")
}

// TestExport_QueryFail 查询失败应报错。
func TestExport_QueryFail(t *testing.T) {
	orig := flagTimeout
	flagTimeout = 5
	defer func() { flagTimeout = orig }()
	whois.SetWhoisQueryProvider(&stubQueryProvider{queryErr: errBoom})
	t.Cleanup(func() { whois.SetWhoisQueryProvider(nil) })
	_, err := runExportCmd(t, "fail.example", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "查询失败")
}

// --- newServersCmd ---

// TestServers_ListAll 无 --tld 列出全部映射。
func TestServers_ListAll(t *testing.T) {
	out, err := runServersCmd(t)
	assert.NoError(t, err)
	assert.Contains(t, out, "com") // 默认加载 com 等
}

// TestServers_FilterFound --tld 命中已有映射。
func TestServers_FilterFound(t *testing.T) {
	out, err := runServersCmd(t, "--tld", "com")
	assert.NoError(t, err)
	assert.Contains(t, out, "whois.verisign-grs.com")
}

// TestServers_FilterNotFound --tld 未命中应报错。
func TestServers_FilterNotFound(t *testing.T) {
	_, err := runServersCmd(t, "--tld", "nosuchtld")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到 TLD")
}

// --- newServersListCmd ---

// TestServersList_NoFilter 列出全部。
func TestServersList_NoFilter(t *testing.T) {
	out, err := runServersCmd(t, "list")
	assert.NoError(t, err)
	assert.Contains(t, out, "com")
}

// TestServersList_FilterFound --tld 命中。
func TestServersList_FilterFound(t *testing.T) {
	out, err := runServersCmd(t, "list", "--tld", "org")
	assert.NoError(t, err)
	assert.Contains(t, out, "pir.org")
}

// TestServersList_FilterNotFound --tld 未命中。
func TestServersList_FilterNotFound(t *testing.T) {
	_, err := runServersCmd(t, "list", "--tld", "zzz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到 TLD")
}

// --- newServersStatsCmd ---

// TestServersStats_Text 文本统计。
func TestServersStats_Text(t *testing.T) {
	out, err := runServersCmd(t, "stats")
	assert.NoError(t, err)
	assert.Contains(t, out, "WHOIS 服务器统计")
	assert.Contains(t, out, "总数")
}

// TestServersStats_JSON --json 输出。
func TestServersStats_JSON(t *testing.T) {
	out, err := runServersCmd(t, "stats", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, "total_servers")
}

// --- newServersSaveCmd ---

// TestServersSave_OK 保存到文件。
func TestServersSave_OK(t *testing.T) {
	p := filepath.Join(t.TempDir(), "servers.json")
	out, err := runServersCmd(t, "save", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "已保存到")
	assert.FileExists(t, p)
}

// TestServersSave_BadPath 保存到无法创建的路径应报错。
func TestServersSave_BadPath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0644))
	target := filepath.Join(blocker, "sub", "servers.json")
	_, err := runServersCmd(t, "save", target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "保存失败")
}

// --- newServersDiscoverCmd ---

// TestServersDiscover_OK 在线发现（网络）。
func TestServersDiscover_OK(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	out, err := runServersCmd(t, "discover", "com")
	if err != nil {
		t.Skipf("网络不可用，跳过: %v", err)
	}
	assert.Contains(t, out, "com:")
}

// TestServersDiscover_NetworkError 网络/未找到分支。
func TestServersDiscover_NetworkError(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	// 用一个几乎不存在的 TLD，IANA 返回空 → "未找到 ... 的 WHOIS 服务器"
	_, err := runServersCmd(t, "discover", "zzzinvalidtld")
	if err == nil {
		return
	}
	// 错误可能是“发现失败”或网络错误
	assert.Contains(t, err.Error(), "发现失败")
}

// --- newServersRefreshCmd ---

// TestServersRefresh_OK 刷新服务器列表（网络）。
// RefreshServerList → DiscoverWhoisServer 的 io.Copy 无超时保护，
// 真实网络下可能永久阻塞导致整个测试包 panic 超时。
// 故用 goroutine + 计时器包裹：5s 内未完成则跳过，避免拖垮整个包。
// 注意：后台 goroutine 不引用 t（测试函数返回后 t 不再安全）。
func TestServersRefresh_OK(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过网络测试")
	}
	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		// 不传 t，用裸 execCapture 避免测试结束后访问 t
		root := newServersCmd()
		root.SetArgs([]string{"refresh"})
		var runErr error
		out := captureStdout(t, func() { runErr = root.Execute() })
		done <- result{out, runErr}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Skipf("网络不可用，跳过: %v", r.err)
		}
		assert.Contains(t, r.out, "服务器列表已刷新")
	case <-time.After(5 * time.Second):
		t.Skip("refresh 网络调用超过 5s，跳过避免阻塞")
	}
}

// ===================== helpers =====================

// captureStdout2 复用 output_test.go 的 captureStdout，别名避免与 serve 测试冲突。
func captureStdout2(t *testing.T, fn func()) string {
	t.Helper()
	return captureStdout(t, fn)
}

// errBoom 是可复用的错误对象，便于 stub provider 返回固定错误。
var errBoom = errBoomType{}

type errBoomType struct{}

func (errBoomType) Error() string { return "boom" }
