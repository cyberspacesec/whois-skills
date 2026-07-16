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

// runConfigCmd 构造 config 命令树执行给定参数，返回 stdout 与 error。
func runConfigCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newConfigCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// writeDefaultConfigFile 把默认库配置保存为 JSON 文件并返回路径。
func writeDefaultConfigFile(t *testing.T, name string) string {
	t.Helper()
	cfg := whois.DefaultWhoisLibraryConfig()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, whois.SaveWhoisLibraryConfigToFile(&cfg, p))
	return p
}

// writeInvalidConfigFile 写一份校验失败的配置（Query.Timeout=0）。
func writeInvalidConfigFile(t *testing.T, name string) string {
	t.Helper()
	cfg := whois.DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 0 // 触发 ValidateWhoisLibraryConfig "查询超时必须大于0"
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, whois.SaveWhoisLibraryConfigToFile(&cfg, p))
	return p
}

// writeOverrideConfigFile 写一份 override 配置（仅修改部分字段）。
func writeOverrideConfigFile(t *testing.T, name string) string {
	t.Helper()
	cfg := whois.DefaultWhoisLibraryConfig()
	cfg.Query.Timeout = 30
	cfg.Log.Level = "debug"
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, whois.SaveWhoisLibraryConfigToFile(&cfg, p))
	return p
}

// --- newConfigCmd ---

// TestConfigCmd_NoSub 无子命令时输出 Usage。
func TestConfigCmd_NoSub(t *testing.T) {
	out, err := runConfigCmd(t)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(out, "show") || strings.Contains(out, "Usage"))
}

// --- newConfigShowCmd ---

// TestConfigShow_Default --default 输出默认库配置摘要。
func TestConfigShow_Default(t *testing.T) {
	out, err := runConfigCmd(t, "show", "--default")
	assert.NoError(t, err)
	assert.Contains(t, out, "查询: ") // 摘要内容
}

// TestConfigShow_DefaultJSON --default --json 输出 JSON（同时输出摘要）。
func TestConfigShow_DefaultJSON(t *testing.T) {
	out, err := runConfigCmd(t, "show", "--default", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"query"`)
}

// TestConfigShow_DefaultSummary --default --summary 输出摘要。
func TestConfigShow_DefaultSummary(t *testing.T) {
	out, err := runConfigCmd(t, "show", "--default", "--summary")
	assert.NoError(t, err)
	assert.Contains(t, out, "查询: ")
}

// TestConfigShow_Global 无 flag 显示当前全局配置。
func TestConfigShow_Global(t *testing.T) {
	out, err := runConfigCmd(t, "show")
	assert.NoError(t, err)
	assert.Contains(t, out, "查询: ")
}

// TestConfigShow_FromFile --file 加载指定文件。
func TestConfigShow_FromFile(t *testing.T) {
	p := writeDefaultConfigFile(t, "lib.json")
	out, err := runConfigCmd(t, "show", "--file", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "查询: ")
}

// TestConfigShow_FromFileJSON --file + --json。
func TestConfigShow_FromFileJSON(t *testing.T) {
	p := writeDefaultConfigFile(t, "lib.json")
	out, err := runConfigCmd(t, "show", "--file", p, "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"query"`)
}

// TestConfigShow_FromFileNotExist --file 文件不存在应报错。
func TestConfigShow_FromFileNotExist(t *testing.T) {
	_, err := runConfigCmd(t, "show", "--file", "/no/such/lib.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "加载库配置")
}

// --- newConfigValidateCmd ---

// TestConfigValidate_OK 合法配置。
func TestConfigValidate_OK(t *testing.T) {
	p := writeDefaultConfigFile(t, "valid.json")
	out, err := runConfigCmd(t, "validate", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "配置合法")
}

// TestConfigValidate_NotExist 文件不存在应报错。
func TestConfigValidate_NotExist(t *testing.T) {
	_, err := runConfigCmd(t, "validate", "/no/such/valid.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "加载库配置")
}

// TestConfigValidate_Invalid 配置无效（Timeout=0）应报错并打印输出。
func TestConfigValidate_Invalid(t *testing.T) {
	p := writeInvalidConfigFile(t, "invalid.json")
	out, err := runConfigCmd(t, "validate", p)
	require.Error(t, err)
	assert.Contains(t, out, "配置无效")
	assert.Contains(t, err.Error(), "查询超时")
}

// --- newConfigSaveCmd ---

// TestConfigSave_Default --default 保存默认配置。
func TestConfigSave_Default(t *testing.T) {
	p := filepath.Join(t.TempDir(), "saved-default.json")
	out, err := runConfigCmd(t, "save", p, "--default")
	assert.NoError(t, err)
	assert.Contains(t, out, "已保存库配置")
	assert.FileExists(t, p)
}

// TestConfigSave_Current 保存当前全局配置。
func TestConfigSave_Current(t *testing.T) {
	p := filepath.Join(t.TempDir(), "saved-current.json")
	out, err := runConfigCmd(t, "save", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "已保存库配置")
	assert.FileExists(t, p)
}

// TestConfigSave_BadPath 保存到无法创建目录的路径应报错（MkdirAll 失败）。
func TestConfigSave_BadPath(t *testing.T) {
	// 先在临时目录建一个普通文件，再尝试在其“子目录”下保存，
	// os.MkdirAll 会因父路径是文件而非目录失败。
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0644))
	target := filepath.Join(blocker, "sub", "cfg.json")
	_, err := runConfigCmd(t, "save", target)
	require.Error(t, err)
}

// --- newConfigMergeCmd ---

// TestConfigMerge_Summary 合并输出摘要。
func TestConfigMerge_Summary(t *testing.T) {
	base := writeDefaultConfigFile(t, "base.json")
	ov := writeOverrideConfigFile(t, "override.json")
	out, err := runConfigCmd(t, "merge", base, ov)
	assert.NoError(t, err)
	assert.Contains(t, out, "查询: ")
}

// TestConfigMerge_JSON 合并输出 JSON。
func TestConfigMerge_JSON(t *testing.T) {
	base := writeDefaultConfigFile(t, "base.json")
	ov := writeOverrideConfigFile(t, "override.json")
	out, err := runConfigCmd(t, "merge", base, ov, "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"query"`)
}

// TestConfigMerge_BadBase base 加载失败应报错。
func TestConfigMerge_BadBase(t *testing.T) {
	ov := writeOverrideConfigFile(t, "override.json")
	_, err := runConfigCmd(t, "merge", "/no/such/base.json", ov)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base 配置")
}

// TestConfigMerge_BadOverride override 加载失败应报错。
func TestConfigMerge_BadOverride(t *testing.T) {
	base := writeDefaultConfigFile(t, "base.json")
	_, err := runConfigCmd(t, "merge", base, "/no/such/override.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "override 配置")
}

// TestConfigMerge_TooFewArgs 参数不足应被 cobra MinimumNArgs 拦截。
func TestConfigMerge_TooFewArgs(t *testing.T) {
	_, err := runConfigCmd(t, "merge", "only-one")
	require.Error(t, err)
}

// --- newConfigApplyCmd ---

// TestConfigApply_OK 应用合法配置。
func TestConfigApply_OK(t *testing.T) {
	p := writeDefaultConfigFile(t, "apply.json")
	out, err := runConfigCmd(t, "apply", p)
	assert.NoError(t, err)
	assert.Contains(t, out, "已应用库配置")
}

// TestConfigApply_NotExist 文件不存在应报错。
func TestConfigApply_NotExist(t *testing.T) {
	_, err := runConfigCmd(t, "apply", "/no/such/apply.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "加载库配置")
}

// TestConfigApply_Invalid 配置无效应报“配置校验失败”。
func TestConfigApply_Invalid(t *testing.T) {
	p := writeInvalidConfigFile(t, "apply-invalid.json")
	_, err := runConfigCmd(t, "apply", p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "配置校验失败")
}

// TestConfigSave_RoundtripJSON 验证保存的 JSON 可被解析（覆盖 SaveWhoisLibraryConfigToFile 输出可读）。
func TestConfigSave_RoundtripJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "roundtrip.json")
	_, err := runConfigCmd(t, "save", p, "--default")
	require.NoError(t, err)
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	var cfg whois.WhoisLibraryConfig
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.Equal(t, 10, cfg.Query.Timeout)
}
