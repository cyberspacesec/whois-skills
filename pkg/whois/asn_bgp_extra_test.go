package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- NewLocalASNRelationProvider: filePath 空默认 ----

func TestNewLocalASNRelationProvider_EmptyPath(t *testing.T) {
	orig := globalASNRelationProvider
	defer func() { globalASNRelationProvider = orig }()
	// filePath="" → 默认 "data/as-rel.txt"（不存在）→ loadFile 失败但仅 Warn，返回 p 非 nil
	p, err := NewLocalASNRelationProvider("")
	assert.NoError(t, err)
	assert.NotNil(t, p)
	// 清理可能的目录
	os.RemoveAll("data/as-rel.txt")
}

// ---- loadFile: 空行/注释/格式错误/解析错误 行跳过 ----

func TestLocalASNRelationProvider_loadFile_SkipBadLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "as-rel.txt")
	content := "" +
		"# 注释行\n" +
		"\n" +
		"123|456|-1|src\n" +
		"badline\n" +               // parts<3 → 跳过
		"abc|456|-1|src\n" +        // err1 → 跳过
		"123|xyz|-1|src\n" +        // err2 → 跳过
		"123|456|notanint|src\n" +  // err3 → 跳过
		"789|012|0|src\n"
	os.WriteFile(path, []byte(content), 0644)

	p, err := NewLocalASNRelationProvider(path)
	assert.NoError(t, err)
	// 正确解析两行：123|456|-1 与 789|012|0
	rel, err := p.QueryRelations(context.Background(), 123)
	assert.NoError(t, err)
	assert.Contains(t, rel.UpstreamASNs, 456) // -1 → provider → 上游
	rel2, _ := p.QueryRelations(context.Background(), 789)
	assert.Contains(t, rel2.PeerASNs, 12) // 0 → peer
}

// ---- Close: stopRefresh != nil（启动刷新后关闭）----
// NewLocalASNRelationProvider 不启动刷新；手动构造带 stopRefresh 的实例。

func TestLocalASNRelationProvider_Close_WithStopRefresh(t *testing.T) {
	p := &LocalASNRelationProvider{
		stopRefresh: make(chan struct{}),
	}
	assert.NoError(t, p.Close())
}

// ---- queryASNRelations: provider 查询失败分支 ----

type failingASNRelationProvider struct{}

func (failingASNRelationProvider) QueryRelations(ctx context.Context, asn int) (*ASNRelations, error) {
	return nil, assertError("bgp fail")
}
func (failingASNRelationProvider) Close() error { return nil }

func TestQueryASNRelations_ProviderFail(t *testing.T) {
	orig := globalASNRelationProvider
	defer func() { globalASNRelationProvider = orig }()
	SetASNRelationProvider(failingASNRelationProvider{})

	info := &ASNDetail{ASN: 13335}
	queryASNRelations(context.Background(), &ASNQueryOptions{
		ASN:          13335,
		IncludeBGP:   true,
		Source:       ASNSourceAll,
	}, info)
	// 失败分支仅 log.Warn，info 不被填充
	assert.Empty(t, info.UpstreamASNs)
}

// ---- queryASNRelations: IncludeBGP=false 直接返回 ----

func TestQueryASNRelations_NotIncluded(t *testing.T) {
	info := &ASNDetail{ASN: 1}
	queryASNRelations(context.Background(), &ASNQueryOptions{IncludeBGP: false}, info)
	assert.Empty(t, info.UpstreamASNs)
}

// ---- InitASNRelationFromConfig: local 成功（已有文件）----

func TestInitASNRelationFromConfig_LocalSuccess(t *testing.T) {
	orig := globalASNRelationProvider
	defer func() { globalASNRelationProvider = orig }()
	dir := t.TempDir()
	path := filepath.Join(dir, "as-rel.txt")
	os.WriteFile(path, []byte("1|2|0|src\n"), 0644)
	err := InitASNRelationFromConfig(&ASNRelationConfig{
		Enabled:  true,
		Type:     "local",
		FilePath: path,
	})
	assert.NoError(t, err)
	assert.NotNil(t, globalASNRelationProvider)
	if p, ok := globalASNRelationProvider.(*LocalASNRelationProvider); ok {
		p.Close()
	}
}

// ---- InitASNRelationFromConfig: local 文件不存在（仍返回 p，仅 Warn）----

func TestInitASNRelationFromConfig_LocalMissingFile(t *testing.T) {
	orig := globalASNRelationProvider
	defer func() { globalASNRelationProvider = orig }()
	err := InitASNRelationFromConfig(&ASNRelationConfig{
		Enabled:  true,
		Type:     "local",
		FilePath: "/nonexistent/as-rel.txt",
	})
	// NewLocalASNRelationProvider 文件加载失败仅 Warn，不返回 err
	assert.NoError(t, err)
	assert.NotNil(t, globalASNRelationProvider)
}
