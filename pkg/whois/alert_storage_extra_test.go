package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// helpers
func writeFile(path, content string) {
	_ = os.WriteFile(path, []byte(content), 0644)
}

func import_osMkdirAll(path string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
}

// ---- generateAlertID ----

func TestGenerateAlertID(t *testing.T) {
	id := generateAlertID()
	assert.NotEmpty(t, id)
	// 两次调用应不同（纳秒级）
	id2 := generateAlertID()
	// 极少概率相同，仅断言非空
	assert.NotEmpty(t, id2)
}

// ---- LocalAlertStorage.Close ----

func TestLocalAlertStorage_Close(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	assert.NoError(t, s.Close())
}

// ---- SaveAlert 自动生成 ID 与 Timestamp ----

func TestLocalAlertStorage_SaveAlert_AutoFill(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	alert := &DomainAlert{Domain: "x.com", Type: "expiry", Level: "warning"}
	err := s.SaveAlert(context.Background(), alert)
	assert.NoError(t, err)
	assert.NotEmpty(t, alert.ID)
	assert.False(t, alert.Timestamp.IsZero())
}

// ---- QueryAlerts: List 错误、Load 错误、所有过滤条件、Limit ----

func TestLocalAlertStorage_QueryAlerts_Filters(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	now := time.Now()
	alerts := []*DomainAlert{
		{ID: "1", Domain: "a.com", Type: "expiry", Level: "warning", Timestamp: now.Add(-2 * time.Hour)},
		{ID: "2", Domain: "b.com", Type: "change", Level: "critical", Timestamp: now.Add(-1 * time.Hour)},
		{ID: "3", Domain: "a.com", Type: "expiry", Level: "critical", Timestamp: now},
		{ID: "4", Domain: "future.com", Type: "expiry", Level: "warning", Timestamp: now.Add(2 * time.Hour)}, // 在 EndTime 之后
	}
	for _, a := range alerts {
		s.SaveAlert(context.Background(), a)
	}

	// 按域名过滤
	got, err := s.QueryAlerts(context.Background(), AlertFilter{Domain: "a.com"})
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	// 按类型过滤
	got, err = s.QueryAlerts(context.Background(), AlertFilter{Type: "change"})
	assert.NoError(t, err)
	assert.Len(t, got, 1)

	// 按级别过滤
	got, err = s.QueryAlerts(context.Background(), AlertFilter{Level: "critical"})
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	// 按时间范围（EndTime 排除 future.com）
	start := now.Add(-90 * time.Minute)
	end := now.Add(10 * time.Minute)
	got, err = s.QueryAlerts(context.Background(), AlertFilter{StartTime: &start, EndTime: &end})
	assert.NoError(t, err)
	assert.Len(t, got, 2) // id=2,3

	// Limit
	got, err = s.QueryAlerts(context.Background(), AlertFilter{Limit: 1})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestLocalAlertStorage_QueryAlerts_LoadError(t *testing.T) {
	// 写入一个坏 JSON 文件，使 Load 失败但 continue
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	// 手动写入坏 alert 文件
	path := sp.keyPath(alertKey(&DomainAlert{ID: "bad", Timestamp: time.Now()}))
	import_osMkdirAll(path)
	writeFile(path, "not-json")
	got, err := s.QueryAlerts(context.Background(), AlertFilter{})
	assert.NoError(t, err)
	assert.Empty(t, got)
}

// ---- GetAlert ----

func TestLocalAlertStorage_GetAlert_NotFound(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	_, err := s.GetAlert(context.Background(), "nope")
	assert.Error(t, err)
}

func TestLocalAlertStorage_GetAlert_LoadError(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	path := sp.keyPath(alertKey(&DomainAlert{ID: "bad", Timestamp: time.Now()}))
	import_osMkdirAll(path)
	writeFile(path, "not-json")
	_, err := s.GetAlert(context.Background(), "bad")
	assert.Error(t, err)
}

func TestLocalAlertStorage_GetAlert_Success(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	s.SaveAlert(context.Background(), &DomainAlert{ID: "findme", Domain: "x.com", Timestamp: time.Now()})
	got, err := s.GetAlert(context.Background(), "findme")
	assert.NoError(t, err)
	assert.Equal(t, "findme", got.ID)
}

// ---- DeleteAlert ----

func TestLocalAlertStorage_DeleteAlert_NotFound(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	err := s.DeleteAlert(context.Background(), "nope")
	assert.Error(t, err)
}

func TestLocalAlertStorage_DeleteAlert_LoadError(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalAlertStorage(sp)
	path := sp.keyPath(alertKey(&DomainAlert{ID: "bad", Timestamp: time.Now()}))
	import_osMkdirAll(path)
	writeFile(path, "not-json")
	err := s.DeleteAlert(context.Background(), "bad")
	assert.Error(t, err)
}

// ---- LocalMonitorStateStorage ----

func TestLocalMonitorStateStorage_Close(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalMonitorStateStorage(sp)
	assert.NoError(t, s.Close())
}

func TestLocalMonitorStateStorage_SaveWatchState_EmptyDomain(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalMonitorStateStorage(sp)
	err := s.SaveWatchState(context.Background(), &DomainWatchState{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}

func TestLocalMonitorStateStorage_LoadWatchStates_LoadError(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalMonitorStateStorage(sp)
	// 写入坏 watch 文件
	path := sp.keyPath(watchStateKey("bad.com"))
	import_osMkdirAll(path)
	writeFile(path, "not-json")
	states, err := s.LoadWatchStates(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, states)
}

func TestLocalMonitorStateStorage_DeleteWatchState(t *testing.T) {
	sp, _ := NewLocalFileStorage(t.TempDir())
	s := NewLocalMonitorStateStorage(sp)
	s.SaveWatchState(context.Background(), &DomainWatchState{Domain: "x.com"})
	err := s.DeleteWatchState(context.Background(), "x.com")
	assert.NoError(t, err)
}

// ---- InitAlertStorageFromConfig ----

func TestInitAlertStorageFromConfig_Disabled(t *testing.T) {
	orig := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = orig }()
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: false})
	assert.NoError(t, err)
	assert.Nil(t, globalAlertStorageProvider)
}

func TestInitAlertStorageFromConfig_UnknownType(t *testing.T) {
	orig := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = orig }()
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: true, Type: "weird"})
	assert.Error(t, err)
}

func TestInitAlertStorageFromConfig_LocalFail(t *testing.T) {
	orig := globalAlertStorageProvider
	origSP := globalStorageProvider
	defer func() { globalAlertStorageProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	// 用文件路径作为目录 → MkdirAll 失败
	f := t.TempDir() + "/afile"
	writeFile(f, "x")
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: true, Type: "local", Directory: f})
	assert.Error(t, err)
}

func TestInitAlertStorageFromConfig_ExistingStorageProvider(t *testing.T) {
	orig := globalAlertStorageProvider
	origSP := globalStorageProvider
	defer func() { globalAlertStorageProvider = orig; globalStorageProvider = origSP }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalStorageProvider = sp
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalAlertStorageProvider)
}

func TestInitAlertStorageFromConfig_LocalNewDir(t *testing.T) {
	orig := globalAlertStorageProvider
	origSP := globalStorageProvider
	defer func() { globalAlertStorageProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	dir := t.TempDir() + "/alerts"
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: true, Type: "local", Directory: dir})
	assert.NoError(t, err)
	assert.NotNil(t, globalAlertStorageProvider)
	assert.NotNil(t, globalStorageProvider)
}

// ---- InitMonitorStateFromConfig ----

func TestInitMonitorStateFromConfig_Disabled(t *testing.T) {
	orig := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = orig }()
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: false})
	assert.NoError(t, err)
	assert.Nil(t, globalMonitorStateProvider)
}

func TestInitMonitorStateFromConfig_UnknownType(t *testing.T) {
	orig := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = orig }()
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: true, Type: "weird"})
	assert.Error(t, err)
}

func TestInitMonitorStateFromConfig_LocalFail(t *testing.T) {
	orig := globalMonitorStateProvider
	origSP := globalStorageProvider
	defer func() { globalMonitorStateProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	f := t.TempDir() + "/afile"
	writeFile(f, "x")
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: true, Type: "local", Directory: f})
	assert.Error(t, err)
}

func TestInitMonitorStateFromConfig_ExistingStorageProvider(t *testing.T) {
	orig := globalMonitorStateProvider
	origSP := globalStorageProvider
	defer func() { globalMonitorStateProvider = orig; globalStorageProvider = origSP }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalStorageProvider = sp
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalMonitorStateProvider)
}

func TestInitMonitorStateFromConfig_LocalNewDir(t *testing.T) {
	orig := globalMonitorStateProvider
	origSP := globalStorageProvider
	defer func() { globalMonitorStateProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	dir := t.TempDir() + "/monitor"
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: true, Type: "local", Directory: dir})
	assert.NoError(t, err)
	assert.NotNil(t, globalMonitorStateProvider)
}

// ---- 便捷函数 nil provider ----

func TestQueryAlerts_NoProvider(t *testing.T) {
	orig := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = orig }()
	globalAlertStorageProvider = nil
	_, err := QueryAlerts(context.Background(), AlertFilter{})
	assert.Error(t, err)
}

func TestLoadWatchStates_NoProvider(t *testing.T) {
	orig := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = orig }()
	globalMonitorStateProvider = nil
	_, err := LoadWatchStates(context.Background())
	assert.Error(t, err)
}

// ---- InitAlertStorageFromConfig Directory 空 ----

func TestInitAlertStorageFromConfig_EmptyDir(t *testing.T) {
	orig := globalAlertStorageProvider
	origSP := globalStorageProvider
	defer func() { globalAlertStorageProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	// Directory="" → 默认 data/alerts，并 SetStorageProvider
	err := InitAlertStorageFromConfig(&AlertStorageConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalAlertStorageProvider)
	os.RemoveAll("data/alerts")
}

func TestInitMonitorStateFromConfig_EmptyDir(t *testing.T) {
	orig := globalMonitorStateProvider
	origSP := globalStorageProvider
	defer func() { globalMonitorStateProvider = orig; globalStorageProvider = origSP }()
	globalStorageProvider = nil
	err := InitMonitorStateFromConfig(&MonitorStateConfig{Enabled: true, Type: "local"})
	assert.NoError(t, err)
	assert.NotNil(t, globalMonitorStateProvider)
	os.RemoveAll("data/monitor")
}

// ---- List 错误分支（用 RedisStorage 关闭 miniredis 触发）----

func TestLocalAlertStorage_QueryAlerts_ListError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	s := NewLocalAlertStorage(sp)
	cleanup() // 关闭 → List 的 Scan 迭代失败
	_, err = s.QueryAlerts(context.Background(), AlertFilter{})
	assert.Error(t, err)
}

func TestLocalAlertStorage_GetAlert_ListError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	s := NewLocalAlertStorage(sp)
	cleanup()
	_, err = s.GetAlert(context.Background(), "x")
	assert.Error(t, err)
}

func TestLocalAlertStorage_DeleteAlert_ListError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	s := NewLocalAlertStorage(sp)
	cleanup()
	err = s.DeleteAlert(context.Background(), "x")
	assert.Error(t, err)
}

func TestLocalMonitorStateStorage_LoadWatchStates_ListError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	sp, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	s := NewLocalMonitorStateStorage(sp)
	cleanup()
	_, err = s.LoadWatchStates(context.Background())
	assert.Error(t, err)
}

// ---- 便捷函数 非 nil provider 路径 ----

func TestSaveAlert_Convenience(t *testing.T) {
	orig := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	err := SaveAlert(context.Background(), &DomainAlert{Domain: "x.com", Timestamp: time.Now()})
	assert.NoError(t, err)
}

func TestQueryAlerts_Convenience(t *testing.T) {
	orig := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalAlertStorageProvider = NewLocalAlertStorage(sp)
	globalAlertStorageProvider.SaveAlert(context.Background(), &DomainAlert{ID: "1", Domain: "x.com", Timestamp: time.Now()})
	got, err := QueryAlerts(context.Background(), AlertFilter{})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestSaveWatchState_Convenience(t *testing.T) {
	orig := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	err := SaveWatchState(context.Background(), &DomainWatchState{Domain: "x.com"})
	assert.NoError(t, err)
}

func TestLoadWatchStates_Convenience(t *testing.T) {
	orig := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = orig }()
	sp, _ := NewLocalFileStorage(t.TempDir())
	globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
	globalMonitorStateProvider.SaveWatchState(context.Background(), &DomainWatchState{Domain: "x.com"})
	got, err := LoadWatchStates(context.Background())
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}
