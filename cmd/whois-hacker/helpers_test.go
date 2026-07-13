package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// execTestSelf 以子进程重跑指定测试函数，并设置给定环境变量。
// 用于隔离 os.Exit / 阻塞启动等场景。
// 子进程的 stdout/stderr 指向本地 *bytes.Buffer，而非继承父进程 os.Stdout/Stderr：
// 父 go test 进程的 stdout 是管道，子进程继承后写入会因管道缓冲满而阻塞，
// 进而导致 cmd.Run() 永久 Wait —— 此前该模式曾造成 TestErrExit_* 测试 669s 超时。
func execTestSelf(t *testing.T, envKV ...string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), envKV...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	return cmd, &stdout, &stderr
}

// stubQueryProvider 是注入到 whois.SetWhoisQueryProvider 的桩，避免真实联网。
type stubQueryProvider struct {
	raw       string
	info      whoisparser.WhoisInfo
	queryErr  error
	parseErr  error
	calls     int
	queryCalls int
	parseCalls int
}

func (s *stubQueryProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	s.queryCalls++
	if s.queryErr != nil {
		return "", s.queryErr
	}
	if s.raw != "" {
		return s.raw, nil
	}
	return "raw-response-for:" + domain, nil
}

func (s *stubQueryProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	s.parseCalls++
	if s.parseErr != nil {
		return whoisparser.WhoisInfo{}, s.parseErr
	}
	return s.info, nil
}

// newStubProvider 构造一个返回固定 info 的成功 stub。
func newStubProvider() *stubQueryProvider {
	return &stubQueryProvider{
		info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain: "example.com",
			},
			Registrar: &whoisparser.Contact{
				Name: "TestRegistrar",
			},
		},
	}
}

// withStubProvider 注入 stub provider 并在测试结束后恢复默认。
func withStubProvider(t *testing.T, p *stubQueryProvider) {
	t.Helper()
	whois.SetWhoisQueryProvider(p)
	t.Cleanup(func() { whois.SetWhoisQueryProvider(nil) })
}

// TestDurationOf 验证 durationOf 把秒数转 Duration。
func TestDurationOf(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "0s"},
		{10, "10s"},
		{120, "2m0s"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, durationOf(tt.seconds).String())
	}
}

// TestLoadConfigFromFile_EmptyFlag flagConfig 为空时应直接返回。
func TestLoadConfigFromFile_EmptyFlag(t *testing.T) {
	orig := flagConfig
	defer func() { flagConfig = orig }()
	flagConfig = ""
	rootCmd = newRootCmd()
	// 不应 panic
	loadConfigFromFile()
}

// TestLoadConfigFromFile_NotExist 文件不存在走 IsNotExist 分支。
func TestLoadConfigFromFile_NotExist(t *testing.T) {
	orig := flagConfig
	defer func() { flagConfig = orig }()
	rootCmd = newRootCmd()
	flagConfig = "/definitely/not/exists/config.yaml"
	loadConfigFromFile() // 内部仅 Debugf，不应 panic
}

// TestLoadConfigFromFile_BadYAML 坏 YAML 走 Warnf 分支。
func TestLoadConfigFromFile_BadYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/bad.yaml"
	assert.NoError(t, os.WriteFile(path, []byte("  : : not: valid: yaml: [unterminated"), 0644))

	orig := flagConfig
	defer func() { flagConfig = orig }()
	rootCmd = newRootCmd()
	flagConfig = path
	loadConfigFromFile() // 仅 Warnf，不应 panic
}

// TestLoadConfigFromFile_Valid 合法 YAML 覆盖 log/proxy 分支（命令行未显式设置时）。
func TestLoadConfigFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/valid.yaml"
	content := `
log:
  level: debug
  format: json
proxy:
  enabled: true
  file: config/proxies.json
`
	assert.NoError(t, os.WriteFile(path, []byte(content), 0644))

	origConfig := flagConfig
	origLevel := flagLogLevel
	origFormat := flagLogFormat
	origProxy := flagUseProxy
	origProxyFile := flagProxyFile
	defer func() {
		flagConfig = origConfig
		flagLogLevel = origLevel
		flagLogFormat = origFormat
		flagUseProxy = origProxy
		flagProxyFile = origProxyFile
	}()

	rootCmd = newRootCmd()
	flagConfig = path
	// 重置 flag 为默认值，且确保 Changed=false（模拟命令行未显式设置），
	// 使配置文件可覆盖。注意：Set 会标记 Changed=true，需手动重置为 false。
	pf := rootCmd.PersistentFlags()
	pf.Set("log-level", "info")
	pf.Set("log-format", "text")
	pf.Set("use-proxy", "false")
	pf.Set("proxy-file", "config/proxies.json")
	for _, name := range []string{"log-level", "log-format", "use-proxy", "proxy-file"} {
		if fl := pf.Lookup(name); fl != nil {
			fl.Changed = false
		}
	}
	loadConfigFromFile()

	assert.Equal(t, "debug", flagLogLevel)
	assert.Equal(t, "json", flagLogFormat)
	assert.True(t, flagUseProxy)
	assert.Equal(t, "config/proxies.json", flagProxyFile)
}

// TestLoadConfigFromFile_ValidButFlagChanged 命令行显式设置时配置文件不应覆盖。
func TestLoadConfigFromFile_ValidButFlagChanged(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/valid.yaml"
	content := "log:\n  level: debug\n  format: json\nproxy:\n  enabled: true\n"
	assert.NoError(t, os.WriteFile(path, []byte(content), 0644))

	origConfig := flagConfig
	origLevel := flagLogLevel
	origFormat := flagLogFormat
	origProxy := flagUseProxy
	defer func() {
		flagConfig = origConfig
		flagLogLevel = origLevel
		flagLogFormat = origFormat
		flagUseProxy = origProxy
	}()
	rootCmd = newRootCmd()
	flagConfig = path
	// 模拟命令行显式设置（Changed=true）
	rootCmd.PersistentFlags().Set("log-level", "info")
	rootCmd.PersistentFlags().Set("log-format", "text")
	rootCmd.PersistentFlags().Set("use-proxy", "false")
	// Changed() 返回 true
	loadConfigFromFile()
	assert.Equal(t, "info", flagLogLevel)   // 未被覆盖
	assert.False(t, flagUseProxy)            // 未被覆盖
}
