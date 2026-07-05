package whois

import (
	"context"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

// TestLocalReverseWhoisIndexSearch 验证按邮箱/姓名/组织反查。
func TestLocalReverseWhoisIndexSearch(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	ctx := context.Background()

	// 索引两个快照
	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "a.com",
		Info: whoisparser.WhoisInfo{
			Registrant: &whoisparser.Contact{
				Name:         "Alice",
				Email:        "alice@example.com",
				Organization: "Acme Corp",
			},
		},
	})
	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "b.com",
		Info: whoisparser.WhoisInfo{
			Registrant: &whoisparser.Contact{
				Name:         "Bob",
				Email:        "alice@example.com", // 同邮箱
				Organization: "Other Corp",
			},
		},
	})

	// 按邮箱查：应返回两个域名
	results, err := idx.SearchByEmail(ctx, "alice@example.com", nil)
	if err != nil {
		t.Fatalf("SearchByEmail 失败: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("alice@example.com 应匹配 2 个域名，得到 %d", len(results))
	}
	domains := map[string]bool{}
	for _, r := range results {
		domains[r.Domain] = true
	}
	if !domains["a.com"] || !domains["b.com"] {
		t.Errorf("应匹配 a.com 和 b.com，得到 %v", domains)
	}

	// 按姓名查 Alice：应只返回 a.com
	results, _ = idx.SearchByRegistrant(ctx, "Alice", nil)
	if len(results) != 1 || results[0].Domain != "a.com" {
		t.Errorf("Alice 应只匹配 a.com，得到 %+v", results)
	}

	// 按组织查 Acme Corp
	results, _ = idx.SearchByOrganization(ctx, "Acme Corp", nil)
	if len(results) != 1 || results[0].Domain != "a.com" {
		t.Errorf("Acme Corp 应只匹配 a.com，得到 %+v", results)
	}
}

// TestLocalReverseWhoisIndexFuzzy 验证模糊匹配（子串）。
func TestLocalReverseWhoisIndexFuzzy(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	ctx := context.Background()
	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "a.com",
		Info: whoisparser.WhoisInfo{
			Registrant: &whoisparser.Contact{Email: "alice@acme.com"},
		},
	})

	// 用部分字符串模糊匹配
	results, _ := idx.SearchByEmail(ctx, "alice", nil)
	if len(results) != 1 {
		t.Errorf("模糊匹配 alice 应返回 1 个，得到 %d", len(results))
	}
}

// TestLocalReverseWhoisIndexLimit 验证限制数量。
func TestLocalReverseWhoisIndexLimit(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	ctx := context.Background()
	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "a.com",
		Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "shared@x.com"}},
	})
	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "b.com",
		Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "shared@x.com"}},
	})

	results, _ := idx.SearchByEmail(ctx, "shared@x.com", &ReverseWhoisOptions{Limit: 1})
	if len(results) != 1 {
		t.Errorf("Limit=1 应返回 1 个，得到 %d", len(results))
	}
}

// TestLocalReverseWhoisIndexRebuild 验证重建索引。
func TestLocalReverseWhoisIndexRebuild(t *testing.T) {
	idx := NewLocalReverseWhoisIndex(nil)
	ctx := context.Background()

	_ = idx.IndexSnapshot(ctx, &WhoisSnapshot{
		Domain: "a.com",
		Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "a@x.com"}},
	})
	// 重建应清空旧索引
	count, err := idx.RebuildFromSnapshots(ctx, []WhoisSnapshot{
		{Domain: "b.com", Info: whoisparser.WhoisInfo{Registrant: &whoisparser.Contact{Email: "b@x.com"}}},
	})
	if err != nil {
		t.Fatalf("RebuildFromSnapshots 失败: %v", err)
	}
	if count != 1 {
		t.Errorf("应索引 1 个快照，得到 %d", count)
	}
	// a.com 应已不存在
	results, _ := idx.SearchByEmail(ctx, "a@x.com", nil)
	if len(results) != 0 {
		t.Errorf("重建后 a.com 应已清空，得到 %d", len(results))
	}
	// b.com 应存在
	results, _ = idx.SearchByEmail(ctx, "b@x.com", nil)
	if len(results) != 1 {
		t.Errorf("b.com 应存在，得到 %d", len(results))
	}
}

// TestReverseWhoisGlobalInjection 验证全局注入与恢复。
func TestReverseWhoisGlobalInjection(t *testing.T) {
	original := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = original }()

	idx := NewLocalReverseWhoisIndex(nil)
	SetReverseWhoisProvider(idx)
	if GetReverseWhoisProvider() != idx {
		t.Error("注入后应返回注入实例")
	}
	SetReverseWhoisProvider(nil)
	if GetReverseWhoisProvider() != nil {
		t.Error("Set(nil) 后应为 nil")
	}
}

// TestInitReverseWhoisFromConfig 验证从配置初始化。
func TestInitReverseWhoisFromConfig(t *testing.T) {
	original := globalReverseWhoisProvider
	originalHistory := globalHistoryProvider
	defer func() {
		globalReverseWhoisProvider = original
		globalHistoryProvider = originalHistory
	}()

	// 需先启用 HistoryProvider
	storage, _ := NewLocalFileStorage(t.TempDir())
	SetHistoryProvider(NewLocalHistoryStorage(storage))

	if err := InitReverseWhoisFromConfig(&ReverseWhoisConfig{Enabled: true, Type: "local"}); err != nil {
		t.Fatalf("InitReverseWhoisFromConfig 失败: %v", err)
	}
	if _, ok := GetReverseWhoisProvider().(*LocalReverseWhoisIndex); !ok {
		t.Errorf("应初始化 LocalReverseWhoisIndex，得到 %T", GetReverseWhoisProvider())
	}
}

// TestInitReverseWhoisFromConfigNoHistory 验证无 HistoryProvider 时返回错误。
func TestInitReverseWhoisFromConfigNoHistory(t *testing.T) {
	original := globalReverseWhoisProvider
	originalHistory := globalHistoryProvider
	defer func() {
		globalReverseWhoisProvider = original
		globalHistoryProvider = originalHistory
	}()
	globalHistoryProvider = nil

	if err := InitReverseWhoisFromConfig(&ReverseWhoisConfig{Enabled: true, Type: "local"}); err == nil {
		t.Error("无 HistoryProvider 时应返回错误")
	}
}

// TestInitReverseWhoisFromConfigDisabled 验证禁用时清空。
func TestInitReverseWhoisFromConfigDisabled(t *testing.T) {
	original := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = original }()

	SetReverseWhoisProvider(NewLocalReverseWhoisIndex(nil))
	if err := InitReverseWhoisFromConfig(&ReverseWhoisConfig{Enabled: false}); err != nil {
		t.Fatalf("InitReverseWhoisFromConfig 失败: %v", err)
	}
	if GetReverseWhoisProvider() != nil {
		t.Error("Enabled=false 时应清空全局 provider")
	}
}

// TestInitReverseWhoisFromConfigUnknownType 验证未知类型返回错误。
func TestInitReverseWhoisFromConfigUnknownType(t *testing.T) {
	original := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = original }()

	if err := InitReverseWhoisFromConfig(&ReverseWhoisConfig{Enabled: true, Type: "unknown"}); err == nil {
		t.Error("未知类型应返回错误")
	}
}

// TestIndexWhoisSnapshotNoProvider 验证未注入 provider 时静默跳过。
func TestIndexWhoisSnapshotNoProvider(t *testing.T) {
	original := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = original }()
	globalReverseWhoisProvider = nil

	err := IndexWhoisSnapshot(context.Background(), &WhoisSnapshot{Domain: "x.com"})
	if err != nil {
		t.Errorf("未注入 provider 时应静默跳过，得到错误: %v", err)
	}
}