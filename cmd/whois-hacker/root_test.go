package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNeedsServerManager 覆盖 needsServerManager 的 true/false 两个分支。
func TestNeedsServerManager(t *testing.T) {
	need := []string{
		"whois", "ip", "asn", "availability", "diff", "quality", "correlation", "batch",
		"asn-prefixes", "asn-ip-ranges", "analyze", "profile", "registrars",
	}
	for _, name := range need {
		assert.Truef(t, needsServerManager(name), "%s 应需要 server manager", name)
	}
	notNeed := []string{"version", "serve", "config", "cache", "proxy", "metrics", "tools",
		"idn", "format", "export", "servers", "rdap", "completion", "help", "unknown"}
	for _, name := range notNeed {
		assert.Falsef(t, needsServerManager(name), "%s 不应需要 server manager", name)
	}
}

// TestNewRootCmd_PersistentPreRun_VersionShortCircuit 覆盖 PersistentPreRunE 对 version 的早返回分支。
func TestNewRootCmd_PersistentPreRun_VersionShortCircuit(t *testing.T) {
	root := newRootCmd()
	// version 子命令触发 PersistentPreRunE，应跳过日志/代理/server 初始化直接返回 nil
	cmd, _, err := root.Find([]string{"version"})
	assert.NoError(t, err)
	assert.NotNil(t, cmd)
	// 直接调用 PersistentPreRunE（root 上的），传入 version cmd
	preRun := root.PersistentPreRunE
	assert.NotNil(t, preRun)
	assert.NoError(t, preRun(cmd, []string{}))
}

// TestNewRootCmd_PersistentPreRun_CompletionShortCircuit 覆盖 completion/help 早返回分支。
func TestNewRootCmd_PersistentPreRun_CompletionShortCircuit(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"completion", "help"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == nil {
			// help/completion 可能为特殊命令，Find 失败则跳过
			continue
		}
		assert.NoErrorf(t, root.PersistentPreRunE(cmd, []string{}), "%s 应早返回 nil", name)
	}
}

// TestNewRootCmd_DefaultFlags 覆盖 newRootCmd 注册的全部持久 flag 默认值。
func TestNewRootCmd_DefaultFlags(t *testing.T) {
	root := newRootCmd()
	pf := root.PersistentFlags()

	tests := []struct {
		name string
		want interface{}
	}{
		{"config", "config/config.yaml"},
		{"log-level", "info"},
		{"log-format", "text"},
		{"timeout", 10},
		{"use-proxy", false},
		{"proxy-file", "config/proxies.json"},
		{"format", "json"},
	}
	for _, tt := range tests {
		fl := pf.Lookup(tt.name)
		assert.NotNilf(t, fl, "flag %s 应已注册", tt.name)
		if fl == nil {
			continue
		}
		// fl.DefValue 始终为 string，统一用字符串比较
		assert.Equalf(t, fmt.Sprintf("%v", tt.want), fl.DefValue, "flag %s 默认值", tt.name)
	}
}
