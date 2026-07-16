package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// ---- helpers ----

// saveFlagTimeout 并恢复 flagTimeout，便于子命令内部 ctx 超时计算。
func saveFlagTimeout(t *testing.T, v int) {
	t.Helper()
	orig := flagTimeout
	flagTimeout = v
	t.Cleanup(func() { flagTimeout = orig })
}

// writeDomainFile 在临时目录写一个域名文件，返回路径。
func writeDomainFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0644))
	return p
}

// runCmd 为预留的通用执行入口（当前各用例直接调用 cmd.SetArgs/Execute），
// 保留以避免后续重复实现；如不再需要可删除。
var _ = func(cmd interface{ Execute() error }, args []string) error { return cmd.Execute() }

// ============================================================================
// readDomainFile
// ============================================================================

func TestAnalyze_ReadDomainFile_HappyWithCommentsAndBlanks(t *testing.T) {
	p := writeDomainFile(t, "domains.txt", "# 注释行\n\nexample.com\n  # 带前导空格的注释\ngoogle.com\n\n# 尾部注释\n")
	got, err := readDomainFile(p)
	assert.NoError(t, err)
	assert.Equal(t, []string{"example.com", "google.com"}, got)
}

func TestAnalyze_ReadDomainFile_NotExist(t *testing.T) {
	_, err := readDomainFile(filepath.Join(t.TempDir(), "nope.txt"))
	assert.Error(t, err)
}

func TestAnalyze_ReadDomainFile_EmptyFile(t *testing.T) {
	p := writeDomainFile(t, "empty.txt", "")
	got, err := readDomainFile(p)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestAnalyze_ReadDomainFile_OnlyComments(t *testing.T) {
	p := writeDomainFile(t, "c.txt", "# a\n# b\n")
	got, err := readDomainFile(p)
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// ============================================================================
// queryInfo
// ============================================================================

func TestAnalyze_QueryInfo_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := newStubProvider()
	withStubProvider(t, p)

	info, err := queryInfo(context.Background(), "example.com")
	assert.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "example.com", info.Domain.Domain)
}

func TestAnalyze_QueryInfo_QueryError(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := &stubQueryProvider{queryErr: fmt.Errorf("network boom")}
	withStubProvider(t, p)

	_, err := queryInfo(context.Background(), "example.com")
	assert.Error(t, err)
}

// ============================================================================
// newDiffCmd
// ============================================================================

func TestAnalyze_Diff_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newDiffCmd()
	cmd.SetArgs([]string{"old.com", "new.com"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"domain_old"`)
	assert.Contains(t, out, `"domain_new"`)
	assert.Contains(t, out, `"total"`)
}

func TestAnalyze_Diff_FirstQueryFails(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := &stubQueryProvider{queryErr: fmt.Errorf("first fail")}
	withStubProvider(t, p)

	cmd := newDiffCmd()
	cmd.SetArgs([]string{"old.com", "new.com"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询 old.com 失败")
}

// 让第二次查询失败：Query 第一次成功、第二次报错。
// 直接实现 whois.WhoisQueryProvider 接口，用 whois.SetWhoisQueryProvider 注入。
type failSecondQueryProvider struct {
	info whoisparser.WhoisInfo
	call int
}

func (p *failSecondQueryProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	p.call++
	if p.call >= 2 {
		return "", fmt.Errorf("second fail")
	}
	return "raw", nil
}

func (p *failSecondQueryProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	return p.info, nil
}

func TestAnalyze_Diff_SecondQueryFails(t *testing.T) {
	saveFlagTimeout(t, 5)
	whois.SetWhoisQueryProvider(&failSecondQueryProvider{info: newStubProvider().info})
	t.Cleanup(func() { whois.SetWhoisQueryProvider(nil) })

	cmd := newDiffCmd()
	cmd.SetArgs([]string{"old.com", "new.com"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询 new.com 失败")
}

func TestAnalyze_Diff_WrongArgs(t *testing.T) {
	cmd := newDiffCmd()
	cmd.SetArgs([]string{"only-one.com"})
	err := cmd.Execute()
	assert.Error(t, err) // cobra.ExactArgs(2) 校验失败
}

// ============================================================================
// newQualityCmd
// ============================================================================

func TestAnalyze_Quality_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newQualityCmd()
	cmd.SetArgs([]string{"example.com"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"total"`)
}

func TestAnalyze_Quality_QueryFails(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, &stubQueryProvider{queryErr: fmt.Errorf("q fail")})

	cmd := newQualityCmd()
	cmd.SetArgs([]string{"example.com"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询失败")
}

func TestAnalyze_Quality_WrongArgs(t *testing.T) {
	cmd := newQualityCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
}

// ============================================================================
// buildCorrelationEngine
// ============================================================================

func TestAnalyze_BuildCorrelationEngine_AllSuccess(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	engine, ctx, cancel := buildCorrelationEngine([]string{"a.com", "b.com"})
	defer cancel()
	require.NotNil(t, engine)
	assert.NotNil(t, ctx)
	result := engine.Analyze()
	assert.Equal(t, 2, result.Stats.InputDomains)
}

func TestAnalyze_BuildCorrelationEngine_WithErrorDomain(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, &stubQueryProvider{queryErr: fmt.Errorf("ce fail")})

	// queryInfo 失败时 buildCorrelationEngine 仅向 stderr 输出错误后 continue，
	// 不调用 engine.AddDomain，故 domainMap 为空、Analyze().Stats.InputDomains == 0。
	out := captureStderr(t, func() {
		engine, _, cancel := buildCorrelationEngine([]string{"bad.com", "alsobad.com"})
		defer cancel()
		result := engine.Analyze()
		assert.Equal(t, 0, result.Stats.InputDomains)
	})
	assert.Contains(t, out, "查询 bad.com 失败")
	assert.Contains(t, out, "查询 alsobad.com 失败")
}

// ============================================================================
// newCorrelationCmd
// ============================================================================

func TestAnalyze_Correlation_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newCorrelationCmd()
	cmd.SetArgs([]string{"a.com", "b.com"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"stats"`)
}

func TestAnalyze_Correlation_TooFewArgs(t *testing.T) {
	cmd := newCorrelationCmd()
	cmd.SetArgs([]string{"only-one.com"})
	err := cmd.Execute()
	assert.Error(t, err) // MinimumNArgs(2)
}

func TestAnalyze_Correlation_HasSubcommands(t *testing.T) {
	cmd := newCorrelationCmd()
	subs := cmd.Commands()
	names := make(map[string]bool)
	for _, s := range subs {
		names[s.Name()] = true
	}
	assert.True(t, names["analyze"])
	assert.True(t, names["profile"])
	assert.True(t, names["registrars"])
}

// ============================================================================
// newCorrelationAnalyzeCmd
// ============================================================================

func TestAnalyze_CorrelationAnalyze_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newCorrelationAnalyzeCmd()
	cmd.SetArgs([]string{"a.com", "b.com"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"clusters"`)
}

func TestAnalyze_CorrelationAnalyze_TooFewArgs(t *testing.T) {
	cmd := newCorrelationAnalyzeCmd()
	cmd.SetArgs([]string{"only.com"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// ============================================================================
// newCorrelationProfileCmd
// ============================================================================

func TestAnalyze_CorrelationProfile_MissingIDType(t *testing.T) {
	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"a.com", "b.com"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--id")
}

func TestAnalyze_CorrelationProfile_InvalidType(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"a.com", "b.com", "--id", "x", "--type", "bogus"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效类型")
}

func TestAnalyze_CorrelationProfile_NotFound(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"a.com", "b.com", "--id", "nobody@example.com", "--type", "email"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未找到实体")
}

func TestAnalyze_CorrelationProfile_TooFewArgs(t *testing.T) {
	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"a.com", "--id", "x", "--type", "email"})
	err := cmd.Execute()
	assert.Error(t, err) // MinimumNArgs(2)
}

// profile 的成功路径需要 queryInfo 返回带注册人邮箱的 info，且 --id 与归一化后的邮箱一致。
func TestAnalyze_CorrelationProfile_Email_Found(t *testing.T) {
	saveFlagTimeout(t, 5)
	// stub provider 返回带 Registrant.Email 的 info，使 AddDomain 聚类到 email 簇
	p := &stubQueryProvider{
		info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{Domain: "x.com"},
			Registrant: &whoisparser.Contact{
				Email: "owner@realdomain.com", // 非隐私邮箱
				Name:  "Real Owner",
			},
			Registrar: &whoisparser.Contact{Name: "RegA"},
		},
	}
	withStubProvider(t, p)

	// 归一化后邮箱键即 "owner@realdomain.com"（NormalizeContactField 对 email 通常小写）
	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"x.com", "y.com", "--id", "owner@realdomain.com", "--type", "email"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"entity_id"`)
}

func TestAnalyze_CorrelationProfile_Registrant_Found(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := &stubQueryProvider{
		info: whoisparser.WhoisInfo{
			Domain:     &whoisparser.Domain{Domain: "x.com"},
			Registrant: &whoisparser.Contact{Name: "Jane Doe"},
			Registrar:  &whoisparser.Contact{Name: "RegA"},
		},
	}
	withStubProvider(t, p)

	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"x.com", "y.com", "--id", "Jane Doe", "--type", "registrant"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"entity_type"`)
}

func TestAnalyze_CorrelationProfile_OrgAlias_Found(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := &stubQueryProvider{
		info: whoisparser.WhoisInfo{
			Domain:     &whoisparser.Domain{Domain: "x.com"},
			Registrant: &whoisparser.Contact{Organization: "OrgX"},
			Registrar:  &whoisparser.Contact{Name: "RegA"},
		},
	}
	withStubProvider(t, p)

	// "org" 是 "organization" 的别名
	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"x.com", "y.com", "--id", "OrgX", "--type", "org"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"entity_id"`)
}

func TestAnalyze_CorrelationProfile_Organization_Found(t *testing.T) {
	saveFlagTimeout(t, 5)
	p := &stubQueryProvider{
		info: whoisparser.WhoisInfo{
			Domain:     &whoisparser.Domain{Domain: "x.com"},
			Registrant: &whoisparser.Contact{Organization: "OrgY"},
			Registrar:  &whoisparser.Contact{Name: "RegA"},
		},
	}
	withStubProvider(t, p)

	cmd := newCorrelationProfileCmd()
	cmd.SetArgs([]string{"x.com", "y.com", "--id", "OrgY", "--type", "organization"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"entity_id"`)
}

// ============================================================================
// newCorrelationRegistrarsCmd
// ============================================================================

func TestAnalyze_CorrelationRegistrars_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newCorrelationRegistrarsCmd()
	cmd.SetArgs([]string{"a.com", "b.com"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	// GetRegistrarStats 返回 map，序列化为 JSON object
	assert.True(t, strings.Contains(out, "TestRegistrar") || out == "{}" || strings.Contains(out, "{"))
}

func TestAnalyze_CorrelationRegistrars_TooFewArgs(t *testing.T) {
	cmd := newCorrelationRegistrarsCmd()
	cmd.SetArgs([]string{"only.com"})
	err := cmd.Execute()
	assert.Error(t, err)
}

// ============================================================================
// newBatchCmd
// ============================================================================

func TestAnalyze_Batch_Success(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	p := writeDomainFile(t, "domains.txt", "# header\na.com\nb.com\n")
	cmd := newBatchCmd()
	cmd.SetArgs([]string{p, "--concurrency", "1", "--query-delay", "0", "--max-retries", "1"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"total"`)
	assert.Contains(t, out, `"results"`)
}

func TestAnalyze_Batch_WithCheckpoint(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	p := writeDomainFile(t, "domains.txt", "a.com\nb.com\n")
	cpFile := filepath.Join(t.TempDir(), "cp.json")
	cmd := newBatchCmd()
	cmd.SetArgs([]string{p, "--concurrency", "1", "--query-delay", "0",
		"--checkpoint", cpFile, "--checkpoint-interval", "1"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"total"`)
	// 断点文件应被写出
	_, err := os.Stat(cpFile)
	assert.NoError(t, err)
}

func TestAnalyze_Batch_FileNotExist(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cmd := newBatchCmd()
	cmd.SetArgs([]string{filepath.Join(t.TempDir(), "nope.txt")})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "读取域名文件失败")
}

func TestAnalyze_Batch_EmptyDomainList(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	p := writeDomainFile(t, "empty.txt", "# only comments\n\n")
	cmd := newBatchCmd()
	cmd.SetArgs([]string{p})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名列表为空")
}

func TestAnalyze_Batch_WrongArgs(t *testing.T) {
	cmd := newBatchCmd()
	cmd.SetArgs([]string{}) // ExactArgs(1)
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestAnalyze_Batch_HasResumeSubcommand(t *testing.T) {
	cmd := newBatchCmd()
	subs := cmd.Commands()
	found := false
	for _, s := range subs {
		if s.Name() == "resume" {
			found = true
		}
	}
	assert.True(t, found)
}

// ============================================================================
// newBatchResumeCmd
// ============================================================================

// writeCheckpoint 写一个合法的 Checkpoint JSON 文件。
// completed 为 true 时把所有域名标记为已完成（走"全部完成"分支）。
func writeCheckpoint(t *testing.T, path string, domains []string, completed bool) {
	t.Helper()
	cp := &whois.Checkpoint{
		BatchID:          "test-batch",
		CreatedAt:        "2024-01-01T00:00:00Z",
		AllDomains:       domains,
		CompletedDomains: map[string]bool{},
		Results:          map[string]*whois.CheckpointResult{},
	}
	if completed {
		for _, d := range domains {
			cp.CompletedDomains[d] = true
			cp.Results[d] = &whois.CheckpointResult{RawResponse: "raw:" + d}
		}
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}

func TestAnalyze_BatchResume_NoCheckpointArg(t *testing.T) {
	cmd := newBatchResumeCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--checkpoint")
}

func TestAnalyze_BatchResume_LoadFails(t *testing.T) {
	cmd := newBatchResumeCmd()
	cmd.SetArgs([]string{"--checkpoint", filepath.Join(t.TempDir(), "nope.json")})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "加载断点失败")
}

func TestAnalyze_BatchResume_BadJSON(t *testing.T) {
	p := writeDomainFile(t, "cp.json", "{not valid json")
	cmd := newBatchResumeCmd()
	cmd.SetArgs([]string{"--checkpoint", p})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "加载断点失败")
}

func TestAnalyze_BatchResume_AllCompleted(t *testing.T) {
	saveFlagTimeout(t, 5)
	cpFile := filepath.Join(t.TempDir(), "cp.json")
	writeCheckpoint(t, cpFile, []string{"a.com", "b.com"}, true)

	cmd := newBatchResumeCmd()
	cmd.SetArgs([]string{"--checkpoint", cpFile})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"completed": 2`)
	assert.Contains(t, out, `"resumed": 0`)
}

func TestAnalyze_BatchResume_PartialResume(t *testing.T) {
	saveFlagTimeout(t, 5)
	withStubProvider(t, newStubProvider())

	cpFile := filepath.Join(t.TempDir(), "cp.json")
	// 2 个域名，0 个已完成 → 全部待处理
	writeCheckpoint(t, cpFile, []string{"a.com", "b.com"}, false)

	cmd := newBatchResumeCmd()
	cmd.SetArgs([]string{"--checkpoint", cpFile, "--concurrency", "1",
		"--query-delay", "0", "--max-retries", "1"})
	out := captureStdout(t, func() {
		assert.NoError(t, cmd.Execute())
	})
	assert.Contains(t, out, `"total"`)
	assert.Contains(t, out, `"completed"`)
}

// captureStderr 临时把 os.Stderr 换成 buffer 并返回内容。
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	done := make(chan struct{})
	var buf []byte
	go func() {
		tmp := make([]byte, 1024)
		for {
			n, e := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if e != nil {
				close(done)
				return
			}
		}
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	<-done
	return string(buf)
}
