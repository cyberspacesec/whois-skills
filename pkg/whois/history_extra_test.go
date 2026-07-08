package whois

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- Close ----

func TestLocalHistoryStorage_Close(t *testing.T) {
	h := NewLocalHistoryStorage(nil)
	assert.NoError(t, h.Close())
}

// ---- formatBool ----

func TestFormatBool(t *testing.T) {
	assert.Equal(t, "true", formatBool(true))
	assert.Equal(t, "false", formatBool(false))
}

// ---- QueryHistorySnapshots: 全局便捷函数 ----

func TestQueryHistorySnapshots_NoProvider(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	globalHistoryProvider = nil
	_, err := QueryHistorySnapshots(context.Background(), "x.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "历史快照未启用")
}

func TestQueryHistorySnapshots_WithProvider(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalHistoryProvider = NewLocalHistoryStorage(sp)
	globalHistoryProvider.SaveSnapshot(context.Background(), &WhoisSnapshot{
		Domain:    "x.com",
		Timestamp: 1,
		Info:      whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}},
	})
	got, err := QueryHistorySnapshots(context.Background(), "x.com")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

// ---- SaveHistorySnapshot: info 为 nil ----

func TestSaveHistorySnapshot_NilInfo(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalHistoryProvider = NewLocalHistoryStorage(sp)
	err := SaveHistorySnapshot(context.Background(), "x.com", nil, "raw", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WHOIS 信息为空")
}

// ---- SaveHistorySnapshot: 成功（含反向索引）----

func TestSaveHistorySnapshot_Success(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalHistoryProvider = NewLocalHistoryStorage(sp)
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}
	err := SaveHistorySnapshot(context.Background(), "x.com", info, "raw", "note")
	assert.NoError(t, err)
}

// ---- SaveHistorySnapshot: SaveSnapshot 失败 ----

func TestSaveHistorySnapshot_SaveFail(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	// 用关闭的 Redis storage 让 Save 失败
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	globalHistoryProvider = NewLocalHistoryStorage(sp)
	cleanup()
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}
	err = SaveHistorySnapshot(context.Background(), "x.com", info, "raw", "")
	assert.Error(t, err)
}

// ---- SaveSnapshot: 域名为空 ----

func TestLocalHistoryStorage_SaveSnapshot_EmptyDomain(t *testing.T) {
	h := NewLocalHistoryStorage(mustNewLocalFileStorage(t))
	err := h.SaveSnapshot(context.Background(), &WhoisSnapshot{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}

// ---- SaveSnapshot: Timestamp=0 自动填充 ----

func TestLocalHistoryStorage_SaveSnapshot_AutoTimestamp(t *testing.T) {
	h := NewLocalHistoryStorage(mustNewLocalFileStorage(t))
	snap := &WhoisSnapshot{Domain: "x.com", Info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}}
	err := h.SaveSnapshot(context.Background(), snap)
	assert.NoError(t, err)
	assert.NotZero(t, snap.Timestamp)
}

func mustNewLocalFileStorage(t *testing.T) StorageProvider {
	t.Helper()
	sp, err := NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalFileStorage: %v", err)
	}
	return sp
}

// ---- QuerySnapshots: List 失败 ----

func TestLocalHistoryStorage_QuerySnapshots_ListFail(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	h := NewLocalHistoryStorage(sp)
	cleanup()
	_, err = h.QuerySnapshots(context.Background(), "x.com")
	assert.Error(t, err)
}

// ---- QuerySnapshots: 含坏快照（Load 失败跳过）----

func TestLocalHistoryStorage_QuerySnapshots_BadSnapshotSkipped(t *testing.T) {
	h := NewLocalHistoryStorage(mustNewLocalFileStorage(t))
	ctx := context.Background()
	// 存两个合法快照
	h.SaveSnapshot(ctx, &WhoisSnapshot{Domain: "x.com", Timestamp: 1, Info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}})
	h.SaveSnapshot(ctx, &WhoisSnapshot{Domain: "x.com", Timestamp: 2, Info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}})
	// 手动写入一个坏 key
	ls := h.storage
	ls.Save(ctx, historyDomainPrefix("x.com")+"bad", make(chan int)) // chan 无法序列化 → 但 Save 可能失败
	// 直接写文件坏 JSON
	if lfs, ok := ls.(*LocalFileStorage); ok {
		path := lfs.keyPath(historyDomainPrefix("x.com") + "bad")
		writeFile(path, "not-json")
	}
	got, err := h.QuerySnapshots(ctx, "x.com")
	assert.NoError(t, err)
	assert.Len(t, got, 2) // 坏的跳过，返回 2 个合法的
	assert.Equal(t, int64(1), got[0].Timestamp) // 升序
}

// ---- QuerySnapshotsInRange ----

func TestLocalHistoryStorage_QuerySnapshotsInRange(t *testing.T) {
	h := NewLocalHistoryStorage(mustNewLocalFileStorage(t))
	ctx := context.Background()
	for _, ts := range []int64{10, 20, 30, 40} {
		h.SaveSnapshot(ctx, &WhoisSnapshot{Domain: "x.com", Timestamp: ts, Info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}})
	}
	got, err := h.QuerySnapshotsInRange(ctx, "x.com", 20, 30)
	assert.NoError(t, err)
	assert.Len(t, got, 2)
}

// ---- QuerySnapshotsInRange: QuerySnapshots 失败 ----

func TestLocalHistoryStorage_QuerySnapshotsInRange_ListFail(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	h := NewLocalHistoryStorage(sp)
	cleanup()
	_, err = h.QuerySnapshotsInRange(context.Background(), "x.com", 0, 100)
	assert.Error(t, err)
}

// ---- InitHistoryFromConfig: 禁用 ----

func TestInitHistoryFromConfig_Disabled(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: false})
	assert.NoError(t, err)
	assert.Nil(t, globalHistoryProvider)
}

// ---- InitHistoryFromConfig: 未知类型 ----

func TestInitHistoryFromConfig_UnknownType(t *testing.T) {
	orig := globalHistoryProvider
	defer func() { globalHistoryProvider = orig }()
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: true, Type: "weird"})
	assert.Error(t, err)
}

// ---- InitHistoryFromConfig: local 新建目录 ----

func TestInitHistoryFromConfig_LocalNewDir(t *testing.T) {
	orig := globalHistoryProvider
	origSP := globalStorageProvider
	defer func() { globalHistoryProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	dir := t.TempDir() + "/hist"
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: true, Type: "local", Directory: dir})
	assert.NoError(t, err)
	assert.NotNil(t, globalHistoryProvider)
	assert.NotNil(t, globalStorageProvider)
}

// ---- InitHistoryFromConfig: local 目录创建失败 ----

func TestInitHistoryFromConfig_LocalFail(t *testing.T) {
	orig := globalHistoryProvider
	origSP := globalStorageProvider
	defer func() { globalHistoryProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	f := t.TempDir() + "/afile"
	writeFile(f, "x")
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: true, Type: "local", Directory: f})
	assert.Error(t, err)
}

// ---- InitHistoryFromConfig: local 复用已存在 StorageProvider ----

func TestInitHistoryFromConfig_LocalReuseSP(t *testing.T) {
	orig := globalHistoryProvider
	origSP := globalStorageProvider
	defer func() { globalHistoryProvider = orig; globalStorageProvider = origSP }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalStorageProvider = sp
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalHistoryProvider)
}

// ---- InitHistoryFromConfig: Directory 空默认 ----

func TestInitHistoryFromConfig_LocalEmptyDir(t *testing.T) {
	orig := globalHistoryProvider
	origSP := globalStorageProvider
	defer func() { globalHistoryProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	err := InitHistoryFromConfig(&HistoryConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalHistoryProvider)
	// 清理默认目录
	os.RemoveAll("data/history")
}

// ---- CompareSnapshots: nil 参数 ----

func TestCompareSnapshots_NilArgs(t *testing.T) {
	assert.Nil(t, CompareSnapshots(nil, &WhoisSnapshot{}))
	assert.Nil(t, CompareSnapshots(&WhoisSnapshot{}, nil))
}

// ---- compareWhoisFields: 各字段的 Added/Removed 分支 ----

func TestCompareWhoisFields_AddedRemovedBranches(t *testing.T) {
	// from 有 Registrar/Registrant/Administrative/Technical/Billing，to 都没有 → Removed
	from := &whoisparser.WhoisInfo{
		Domain:         &whoisparser.Domain{Domain: "x.com"},
		Registrar:      &whoisparser.Contact{Name: "r"},
		Registrant:     &whoisparser.Contact{Name: "reg"},
		Administrative: &whoisparser.Contact{Name: "a"},
		Technical:      &whoisparser.Contact{Name: "t"},
		Billing:        &whoisparser.Contact{Name: "b"},
	}
	to := &whoisparser.WhoisInfo{}
	diff := &WhoisSnapshotDiff{}
	compareWhoisFields(from, to, diff)
	assert.Contains(t, diff.RemovedFields, "domain")
	assert.Contains(t, diff.RemovedFields, "registrar")
	assert.Contains(t, diff.RemovedFields, "registrant")
	assert.Contains(t, diff.RemovedFields, "administrative")
	assert.Contains(t, diff.RemovedFields, "technical")
	assert.Contains(t, diff.RemovedFields, "billing")

	// 反向：from 无，to 有 → Added
	diff2 := &WhoisSnapshotDiff{}
	compareWhoisFields(to, from, diff2)
	assert.Contains(t, diff2.AddedFields, "domain")
	assert.Contains(t, diff2.AddedFields, "registrar")
	assert.Contains(t, diff2.AddedFields, "registrant")
	assert.Contains(t, diff2.AddedFields, "administrative")
	assert.Contains(t, diff2.AddedFields, "technical")
	assert.Contains(t, diff2.AddedFields, "billing")
}

// ---- compareStructFields: bool 字段变更 ----
// whoisparser.Contact 无 bool 字段，但 Domain 有 Dnssec(bool) 等。
// 构造两个 Domain 仅 Dnssec 不同。

func TestCompareStructFields_BoolChange(t *testing.T) {
	from := &whoisparser.Domain{Domain: "x.com", DNSSec: false}
	to := &whoisparser.Domain{Domain: "x.com", DNSSec: true}
	diff := &WhoisSnapshotDiff{}
	compareStructFields(from, to, "domain", diff)
	// 应有 bool 变更
	found := false
	for _, c := range diff.ChangedFields {
		if c.NewValue == "true" || c.OldValue == "false" {
			found = true
		}
	}
	assert.True(t, found)
}

// ---- compareWhoisFields: 双方均有各 Contact → compareStructFields 路径 ----

func TestCompareWhoisFields_BothHaveContacts(t *testing.T) {
	from := &whoisparser.WhoisInfo{
		Domain:         &whoisparser.Domain{Domain: "x.com"},
		Registrar:      &whoisparser.Contact{Name: "old-r"},
		Registrant:     &whoisparser.Contact{Name: "old-reg"},
		Administrative: &whoisparser.Contact{Name: "old-a"},
		Technical:      &whoisparser.Contact{Name: "old-t"},
		Billing:        &whoisparser.Contact{Name: "old-b"},
	}
	to := &whoisparser.WhoisInfo{
		Domain:         &whoisparser.Domain{Domain: "x.com"},
		Registrar:      &whoisparser.Contact{Name: "new-r"},
		Registrant:     &whoisparser.Contact{Name: "new-reg"},
		Administrative: &whoisparser.Contact{Name: "new-a"},
		Technical:      &whoisparser.Contact{Name: "new-t"},
		Billing:        &whoisparser.Contact{Name: "new-b"},
	}
	diff := &WhoisSnapshotDiff{}
	compareWhoisFields(from, to, diff)
	// 各 Contact 的 Name 变更都应记录
	fields := map[string]bool{}
	for _, c := range diff.ChangedFields {
		fields[c.Field] = true
	}
	assert.True(t, fields["registrar.Name"])
	assert.True(t, fields["registrant.Name"])
	assert.True(t, fields["administrative.Name"])
	assert.True(t, fields["technical.Name"])
	assert.True(t, fields["billing.Name"])
}

// ---- compareStructFields: nil 接口 → IsValid false ----

func TestCompareStructFields_NilInterface(t *testing.T) {
	diff := &WhoisSnapshotDiff{}
	// 传入 nil interface → reflectValue 返回空 Value → IsValid false → 直接 return
	assert.NotPanics(t, func() {
		compareStructFields(nil, nil, "x", diff)
	})
	assert.Empty(t, diff.ChangedFields)
}

// ---- reflectValue: nil 指针 ----

func TestReflectValue_NilPtr(t *testing.T) {
	var p *whoisparser.Domain
	v := reflectValue(p)
	assert.False(t, v.IsValid())
}

func TestReflectValue_NonPtr(t *testing.T) {
	v := reflectValue(whoisparser.Domain{Domain: "x"})
	assert.True(t, v.IsValid())
}
