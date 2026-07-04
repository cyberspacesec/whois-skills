package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// 写一份样例 AS 关系文件（RouteViews 格式：from|to|rel|source，rel -1=provider→customer，0=peer）。
// 构造关系：
//   100 向 200 付费 Transit  → 100 的 upstream 含 200；200 的 downstream 含 100
//   300 与 400 对等          → 300/400 的 peer 互含
func writeSampleASRelFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "as-rel.txt")
	content := `# RouteViews AS relationships
# format: from_as|to_as|relationship|source
100|200|-1|routeviews
300|400|0|routeviews
# 这条会让 200 也有上游
200|500|-1|routeviews
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入样例文件失败: %v", err)
	}
	return path
}

// TestLocalASNRelationProviderQuery 验证本地文件解析与关系查询。
func TestLocalASNRelationProviderQuery(t *testing.T) {
	path := writeSampleASRelFile(t)
	p, err := NewLocalASNRelationProvider(path)
	if err != nil {
		t.Fatalf("NewLocalASNRelationProvider 失败: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	// 查 ASN 100：上游应为 200
	rel, err := p.QueryRelations(ctx, 100)
	if err != nil {
		t.Fatalf("QueryRelations(100) 失败: %v", err)
	}
	if !containsInt(rel.UpstreamASNs, 200) {
		t.Errorf("ASN 100 的 upstream 应含 200，得到 %v", rel.UpstreamASNs)
	}
	if len(rel.DownstreamASNs) != 0 {
		t.Errorf("ASN 100 不应有 downstream，得到 %v", rel.DownstreamASNs)
	}

	// 查 ASN 200：下游应为 100，上游应为 500
	rel200, _ := p.QueryRelations(ctx, 200)
	if !containsInt(rel200.DownstreamASNs, 100) {
		t.Errorf("ASN 200 的 downstream 应含 100，得到 %v", rel200.DownstreamASNs)
	}
	if !containsInt(rel200.UpstreamASNs, 500) {
		t.Errorf("ASN 200 的 upstream 应含 500，得到 %v", rel200.UpstreamASNs)
	}

	// 查 ASN 300：对等应为 400
	rel300, _ := p.QueryRelations(ctx, 300)
	if !containsInt(rel300.PeerASNs, 400) {
		t.Errorf("ASN 300 的 peer 应含 400，得到 %v", rel300.PeerASNs)
	}

	// 查 ASN 400：对等应为 300（来自 toIndex）
	rel400, _ := p.QueryRelations(ctx, 400)
	if !containsInt(rel400.PeerASNs, 300) {
		t.Errorf("ASN 400 的 peer 应含 300，得到 %v", rel400.PeerASNs)
	}

	// 查不存在的 ASN：应返回空关系
	relMiss, _ := p.QueryRelations(ctx, 99999)
	if len(relMiss.UpstreamASNs) != 0 || len(relMiss.DownstreamASNs) != 0 || len(relMiss.PeerASNs) != 0 {
		t.Errorf("不存在的 ASN 应返回空关系，得到 %+v", relMiss)
	}
}

// TestASNRelationProviderGlobalInjection 验证全局 provider 注入与恢复。
func TestASNRelationProviderGlobalInjection(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()

	path := writeSampleASRelFile(t)
	p, _ := NewLocalASNRelationProvider(path)
	defer p.Close()

	SetASNRelationProvider(p)
	if GetASNRelationProvider() != p {
		t.Error("注入后应返回注入实例")
	}
	SetASNRelationProvider(nil)
	if GetASNRelationProvider() != nil {
		t.Error("Set(nil) 后应为 nil")
	}
}

// TestInitASNRelationFromConfigLocal 验证从配置初始化本地 provider。
func TestInitASNRelationFromConfigLocal(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()

	path := writeSampleASRelFile(t)
	if err := InitASNRelationFromConfig(&ASNRelationConfig{
		Enabled:  true,
		Type:     "local",
		FilePath: path,
	}); err != nil {
		t.Fatalf("InitASNRelationFromConfig 失败: %v", err)
	}
	if _, ok := GetASNRelationProvider().(*LocalASNRelationProvider); !ok {
		t.Errorf("应初始化 LocalASNRelationProvider，得到 %T", GetASNRelationProvider())
	}
}

// TestInitASNRelationFromConfigDisabled 验证禁用时清空。
func TestInitASNRelationFromConfigDisabled(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()

	path := writeSampleASRelFile(t)
	p, _ := NewLocalASNRelationProvider(path)
	defer p.Close()
	SetASNRelationProvider(p)

	if err := InitASNRelationFromConfig(&ASNRelationConfig{Enabled: false}); err != nil {
		t.Fatalf("InitASNRelationFromConfig 失败: %v", err)
	}
	if GetASNRelationProvider() != nil {
		t.Error("Enabled=false 时应清空全局 provider")
	}
}

// TestInitASNRelationFromConfigUnknownType 验证未知类型返回错误。
func TestInitASNRelationFromConfigUnknownType(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()

	if err := InitASNRelationFromConfig(&ASNRelationConfig{Enabled: true, Type: "unknown"}); err == nil {
		t.Error("未知类型应返回错误")
	}
}

// TestQueryASNWithBGPRelations 验证 IncludeBGP=true 时 ASNDetail 被填充关系。
func TestQueryASNWithBGPRelations(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()
	// 清缓存避免上轮测试残留
	asnDetailCache.mu.Lock()
	asnDetailCache.items = make(map[int]*ASNDetail)
	asnDetailCache.mu.Unlock()

	path := writeSampleASRelFile(t)
	p, _ := NewLocalASNRelationProvider(path)
	defer p.Close()
	SetASNRelationProvider(p)

	// 不真正联网：用 stub provider 已在上层填充。这里直接调 queryASNRelations。
	info := &ASNDetail{ASN: 100}
	queryASNRelations(context.Background(), &ASNQueryOptions{ASN: 100, IncludeBGP: true}, info)
	if !containsInt(info.UpstreamASNs, 200) {
		t.Errorf("IncludeBGP=true 时 UpstreamASNs 应含 200，得到 %v", info.UpstreamASNs)
	}

	// IncludeBGP=false 时不填充
	info2 := &ASNDetail{ASN: 100}
	queryASNRelations(context.Background(), &ASNQueryOptions{ASN: 100, IncludeBGP: false}, info2)
	if len(info2.UpstreamASNs) != 0 {
		t.Errorf("IncludeBGP=false 时不应填充，得到 %v", info2.UpstreamASNs)
	}
}

// TestQueryASNRelationsNoProvider 验证未注入 provider 时不报错（静默跳过）。
func TestQueryASNRelationsNoProvider(t *testing.T) {
	original := globalASNRelationProvider
	defer func() { globalASNRelationProvider = original }()
	globalASNRelationProvider = nil

	info := &ASNDetail{ASN: 100}
	// 不应 panic
	queryASNRelations(context.Background(), &ASNQueryOptions{ASN: 100, IncludeBGP: true}, info)
	if len(info.UpstreamASNs) != 0 {
		t.Errorf("未注入 provider 时不应填充，得到 %v", info.UpstreamASNs)
	}
}

// TestLocalASNRelationProviderMissingFile 验证文件不存在时不 panic（返回空查询）。
func TestLocalASNRelationProviderMissingFile(t *testing.T) {
	p, err := NewLocalASNRelationProvider("/nonexistent/as-rel.txt")
	if err != nil {
		t.Fatalf("文件不存在时构造不应失败: %v", err)
	}
	defer p.Close()
	rel, err := p.QueryRelations(context.Background(), 100)
	if err != nil {
		t.Fatalf("查询不应失败: %v", err)
	}
	if len(rel.UpstreamASNs) != 0 {
		t.Errorf("文件不存在时应返回空关系，得到 %v", rel.UpstreamASNs)
	}
}