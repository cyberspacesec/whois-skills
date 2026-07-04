package whois

import (
	"context"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

// TestLocalHistoryStorageSaveQuery 验证快照存取。
func TestLocalHistoryStorageSaveQuery(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	h := NewLocalHistoryStorage(storage)

	ctx := context.Background()
	now := time.Now().Unix()
	snap := &WhoisSnapshot{
		Domain:    "example.com",
		Timestamp: now,
		Info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{Domain: "example.com"},
		},
		Note: "test",
	}
	if err := h.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot 失败: %v", err)
	}

	got, err := h.GetSnapshot(ctx, "example.com", now)
	if err != nil {
		t.Fatalf("GetSnapshot 失败: %v", err)
	}
	if got.Domain != "example.com" || got.Note != "test" {
		t.Errorf("快照内容不匹配: %+v", got)
	}
}

// TestLocalHistoryStorageQuerySnapshots 验证列出与时间范围查询。
func TestLocalHistoryStorageQuerySnapshots(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	h := NewLocalHistoryStorage(storage)

	ctx := context.Background()
	now := time.Now().Unix()
	// 存 3 个快照
	for i := 0; i < 3; i++ {
		_ = h.SaveSnapshot(ctx, &WhoisSnapshot{
			Domain:    "example.com",
			Timestamp: now + int64(i*3600), // 每隔 1 小时
			Info:      whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "example.com"}},
		})
	}

	snaps, err := h.QuerySnapshots(ctx, "example.com")
	if err != nil {
		t.Fatalf("QuerySnapshots 失败: %v", err)
	}
	if len(snaps) != 3 {
		t.Errorf("应有 3 个快照，得到 %d", len(snaps))
	}
	// 验证按时间升序
	for i := 1; i < len(snaps); i++ {
		if snaps[i].Timestamp < snaps[i-1].Timestamp {
			t.Error("快照未按时间升序")
		}
	}

	// 时间范围查询（取中间）
	rangeSnaps, _ := h.QuerySnapshotsInRange(ctx, "example.com", now+3600, now+3600)
	if len(rangeSnaps) != 1 {
		t.Errorf("范围查询应有 1 个快照，得到 %d", len(rangeSnaps))
	}
}

// TestLocalHistoryStorageDelete 验证删除快照。
func TestLocalHistoryStorageDelete(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	h := NewLocalHistoryStorage(storage)

	ctx := context.Background()
	now := time.Now().Unix()
	_ = h.SaveSnapshot(ctx, &WhoisSnapshot{Domain: "example.com", Timestamp: now})

	if err := h.DeleteSnapshot(ctx, "example.com", now); err != nil {
		t.Fatalf("DeleteSnapshot 失败: %v", err)
	}
	_, err := h.GetSnapshot(ctx, "example.com", now)
	if err == nil {
		t.Error("删除后不应能获取快照")
	}
}

// TestHistoryProviderGlobalInjection 验证全局 provider 注入。
func TestHistoryProviderGlobalInjection(t *testing.T) {
	original := globalHistoryProvider
	defer func() { globalHistoryProvider = original }()

	storage, _ := NewLocalFileStorage(t.TempDir())
	h := NewLocalHistoryStorage(storage)
	SetHistoryProvider(h)
	if GetHistoryProvider() != h {
		t.Error("注入后应返回注入实例")
	}
	SetHistoryProvider(nil)
	if GetHistoryProvider() != nil {
		t.Error("Set(nil) 后应为 nil")
	}
}

// TestInitHistoryFromConfigLocal 验证从配置初始化。
func TestInitHistoryFromConfigLocal(t *testing.T) {
	original := globalHistoryProvider
	originalStorage := globalStorageProvider
	defer func() {
		globalHistoryProvider = original
		globalStorageProvider = originalStorage
	}()

	dir := t.TempDir()
	if err := InitHistoryFromConfig(&HistoryConfig{
		Enabled:   true,
		Type:      "local",
		Directory: dir,
	}); err != nil {
		t.Fatalf("InitHistoryFromConfig 失败: %v", err)
	}
	if _, ok := GetHistoryProvider().(*LocalHistoryStorage); !ok {
		t.Errorf("应初始化 LocalHistoryStorage，得到 %T", GetHistoryProvider())
	}
}

// TestSaveHistorySnapshotNoProvider 验证未注入 provider 时静默跳过。
func TestSaveHistorySnapshotNoProvider(t *testing.T) {
	original := globalHistoryProvider
	defer func() { globalHistoryProvider = original }()
	globalHistoryProvider = nil

	err := SaveHistorySnapshot(context.Background(), "example.com", &whoisparser.WhoisInfo{}, "", "test")
	if err != nil {
		t.Errorf("未注入 provider 时应静默跳过，得到错误: %v", err)
	}
}

// TestCompareSnapshots 验证快照对比。
func TestCompareSnapshots(t *testing.T) {
	now := time.Now().Unix()
	from := &WhoisSnapshot{
		Domain:    "example.com",
		Timestamp: now,
		Info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:         "example.com",
				ExpirationDate: "2025-01-01",
				NameServers:    []string{"ns1.old.com", "ns2.old.com"},
			},
			Registrar: &whoisparser.Contact{Name: "Old Registrar"},
		},
	}
	to := &WhoisSnapshot{
		Domain:    "example.com",
		Timestamp: now + 86400,
		Info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{
				Domain:         "example.com",
				ExpirationDate: "2026-01-01", // 变更
				NameServers:    []string{"ns1.new.com", "ns2.new.com"},
			},
			Registrar: &whoisparser.Contact{Name: "New Registrar"}, // 变更
		},
	}

	diff := CompareSnapshots(from, to)
	if diff == nil {
		t.Fatal("对比结果不应为空")
	}
	if diff.FromTime != from.Timestamp || diff.ToTime != to.Timestamp {
		t.Errorf("时间戳不匹配: %+v", diff)
	}
	// 检查字段变更
	changedFields := make(map[string]bool)
	for _, fc := range diff.ChangedFields {
		changedFields[fc.Field] = true
	}
	// 应检测到 registrar.Name 变更
	if !hasFieldPrefix(changedFields, "registrar.Name") {
		t.Errorf("应检测到 registrar.Name 变更，得到 %v", changedFields)
	}
	// 应检测到 domain.ExpirationDate 变更
	if !hasFieldPrefix(changedFields, "domain.ExpirationDate") {
		t.Errorf("应检测到 domain.ExpirationDate 变更，得到 %v", changedFields)
	}
}

func hasFieldPrefix(m map[string]bool, prefix string) bool {
	for k := range m {
		if k == prefix {
			return true
		}
	}
	return false
}