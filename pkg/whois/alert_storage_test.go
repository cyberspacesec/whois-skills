package whois

import (
	"context"
	"testing"
	"time"
)

// TestLocalAlertStorageSaveQuery 验证告警存取。
func TestLocalAlertStorageSaveQuery(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	a := NewLocalAlertStorage(storage)

	ctx := context.Background()
	alert := &DomainAlert{
		ID:        "test-123",
		Domain:    "example.com",
		Type:      AlertExpiryWarning,
		Level:     AlertLevelWarning,
		Message:   "即将过期",
		Timestamp: time.Now(),
	}
	if err := a.SaveAlert(ctx, alert); err != nil {
		t.Fatalf("SaveAlert 失败: %v", err)
	}

	got, err := a.GetAlert(ctx, "test-123")
	if err != nil {
		t.Fatalf("GetAlert 失败: %v", err)
	}
	if got.Domain != "example.com" || got.Message != "即将过期" {
		t.Errorf("告警内容不匹配: %+v", got)
	}
}

// TestLocalAlertStorageQueryFilter 验证告警查询过滤。
func TestLocalAlertStorageQueryFilter(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	a := NewLocalAlertStorage(storage)

	ctx := context.Background()
	now := time.Now()
	// 存多个告警
	_ = a.SaveAlert(ctx, &DomainAlert{ID: "1", Domain: "a.com", Type: AlertExpiryWarning, Level: AlertLevelWarning, Timestamp: now})
	_ = a.SaveAlert(ctx, &DomainAlert{ID: "2", Domain: "a.com", Type: AlertRegistrantChange, Level: AlertLevelCritical, Timestamp: now.Add(time.Hour)})
	_ = a.SaveAlert(ctx, &DomainAlert{ID: "3", Domain: "b.com", Type: AlertExpiryWarning, Level: AlertLevelInfo, Timestamp: now})

	// 按域名过滤
	alerts, err := a.QueryAlerts(ctx, AlertFilter{Domain: "a.com"})
	if err != nil {
		t.Fatalf("QueryAlerts 失败: %v", err)
	}
	if len(alerts) != 2 {
		t.Errorf("a.com 应有 2 条告警，得到 %d", len(alerts))
	}

	// 按类型过滤
	alertsType, _ := a.QueryAlerts(ctx, AlertFilter{Type: AlertExpiryWarning})
	if len(alertsType) != 2 {
		t.Errorf("ExpiryWarning 应有 2 条，得到 %d", len(alertsType))
	}

	// 按级别过滤
	alertsLevel, _ := a.QueryAlerts(ctx, AlertFilter{Level: AlertLevelCritical})
	if len(alertsLevel) != 1 {
		t.Errorf("Critical 应有 1 条，得到 %d", len(alertsLevel))
	}

	// 限制数量
	alertsLimit, _ := a.QueryAlerts(ctx, AlertFilter{Limit: 1})
	if len(alertsLimit) != 1 {
		t.Errorf("Limit=1 应返回 1 条，得到 %d", len(alertsLimit))
	}
}

// TestLocalAlertStorageDelete 验证删除告警。
func TestLocalAlertStorageDelete(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	a := NewLocalAlertStorage(storage)

	ctx := context.Background()
	_ = a.SaveAlert(ctx, &DomainAlert{ID: "del-1", Domain: "x.com", Timestamp: time.Now()})

	if err := a.DeleteAlert(ctx, "del-1"); err != nil {
		t.Fatalf("DeleteAlert 失败: %v", err)
	}
	_, err := a.GetAlert(ctx, "del-1")
	if err == nil {
		t.Error("删除后不应能获取告警")
	}
}

// TestAlertStorageGlobalInjection 验证全局注入。
func TestAlertStorageGlobalInjection(t *testing.T) {
	original := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = original }()

	storage, _ := NewLocalFileStorage(t.TempDir())
	a := NewLocalAlertStorage(storage)
	SetAlertStorageProvider(a)
	if GetAlertStorageProvider() != a {
		t.Error("注入后应返回注入实例")
	}
	SetAlertStorageProvider(nil)
	if GetAlertStorageProvider() != nil {
		t.Error("Set(nil) 后应为 nil")
	}
}

// TestLocalMonitorStateStorageSaveLoad 验证监控状态存取。
func TestLocalMonitorStateStorageSaveLoad(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	m := NewLocalMonitorStateStorage(storage)

	ctx := context.Background()
	state := &DomainWatchState{
		Domain:        "example.com",
		LastCheck:     time.Now(),
		Status:        WatchStatusActive,
		DaysRemaining: 30,
	}
	if err := m.SaveWatchState(ctx, state); err != nil {
		t.Fatalf("SaveWatchState 失败: %v", err)
	}

	states, err := m.LoadWatchStates(ctx)
	if err != nil {
		t.Fatalf("LoadWatchStates 失败: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("应有 1 个状态，得到 %d", len(states))
	}
	if states["example.com"].DaysRemaining != 30 {
		t.Errorf("状态内容不匹配: %+v", states["example.com"])
	}
}

// TestLocalMonitorStateStorageDelete 验证删除监控状态。
func TestLocalMonitorStateStorageDelete(t *testing.T) {
	dir := t.TempDir()
	storage, _ := NewLocalFileStorage(dir)
	m := NewLocalMonitorStateStorage(storage)

	ctx := context.Background()
	_ = m.SaveWatchState(ctx, &DomainWatchState{Domain: "x.com", Status: WatchStatusActive})

	if err := m.DeleteWatchState(ctx, "x.com"); err != nil {
		t.Fatalf("DeleteWatchState 失败: %v", err)
	}
	states, _ := m.LoadWatchStates(ctx)
	if len(states) != 0 {
		t.Errorf("删除后应有 0 个状态，得到 %d", len(states))
	}
}

// TestMonitorStateGlobalInjection 验证全局注入。
func TestMonitorStateGlobalInjection(t *testing.T) {
	original := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = original }()

	storage, _ := NewLocalFileStorage(t.TempDir())
	m := NewLocalMonitorStateStorage(storage)
	SetMonitorStateProvider(m)
	if GetMonitorStateProvider() != m {
		t.Error("注入后应返回注入实例")
	}
	SetMonitorStateProvider(nil)
	if GetMonitorStateProvider() != nil {
		t.Error("Set(nil) 后应为 nil")
	}
}

// TestSaveAlertNoProvider 验证未注入 provider 时静默跳过。
func TestSaveAlertNoProvider(t *testing.T) {
	original := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = original }()
	globalAlertStorageProvider = nil

	alert := &DomainAlert{ID: "test", Domain: "x.com"}
	err := SaveAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("未注入 provider 时应静默跳过，得到错误: %v", err)
	}
}

// TestSaveWatchStateNoProvider 验证未注入 provider 时静默跳过。
func TestSaveWatchStateNoProvider(t *testing.T) {
	original := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = original }()
	globalMonitorStateProvider = nil

	state := &DomainWatchState{Domain: "x.com"}
	err := SaveWatchState(context.Background(), state)
	if err != nil {
		t.Errorf("未注入 provider 时应静默跳过，得到错误: %v", err)
	}
}

// TestInitAlertStorageFromConfigLocal 验证从配置初始化。
func TestInitAlertStorageFromConfigLocal(t *testing.T) {
	original := globalAlertStorageProvider
	originalStorage := globalStorageProvider
	defer func() {
		globalAlertStorageProvider = original
		globalStorageProvider = originalStorage
	}()

	dir := t.TempDir()
	if err := InitAlertStorageFromConfig(&AlertStorageConfig{
		Enabled:   true,
		Type:      "local",
		Directory: dir,
	}); err != nil {
		t.Fatalf("InitAlertStorageFromConfig 失败: %v", err)
	}
	if _, ok := GetAlertStorageProvider().(*LocalAlertStorage); !ok {
		t.Errorf("应初始化 LocalAlertStorage，得到 %T", GetAlertStorageProvider())
	}
}

// TestInitMonitorStateFromConfigLocal 验证从配置初始化。
func TestInitMonitorStateFromConfigLocal(t *testing.T) {
	original := globalMonitorStateProvider
	originalStorage := globalStorageProvider
	defer func() {
		globalMonitorStateProvider = original
		globalStorageProvider = originalStorage
	}()

	dir := t.TempDir()
	if err := InitMonitorStateFromConfig(&MonitorStateConfig{
		Enabled:   true,
		Type:      "local",
		Directory: dir,
	}); err != nil {
		t.Fatalf("InitMonitorStateFromConfig 失败: %v", err)
	}
	if _, ok := GetMonitorStateProvider().(*LocalMonitorStateStorage); !ok {
		t.Errorf("应初始化 LocalMonitorStateStorage，得到 %T", GetMonitorStateProvider())
	}
}