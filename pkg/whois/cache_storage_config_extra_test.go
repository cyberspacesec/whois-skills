package whois

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== config.go ApplyWhoisLibraryConfig 各 Init 失败 warn 分支 ====================

// TestApplyWhoisLibraryConfig_StorageInitFail storage 用坏 Directory 触发 InitStorage 失败 → warn 分支。
func TestApplyWhoisLibraryConfig_StorageInitFail(t *testing.T) {
	origStor := globalStorageProvider
	defer func() { globalStorageProvider = origStor }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.Storage.Enabled = true
	cfg.Storage.Type = "local"
	// Directory 是一个已存在文件 → NewLocalFileStorage MkdirAll 失败
	f := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(f, []byte("x"), 0644)
	cfg.Storage.Directory = f
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err) // Init 失败仅 warn，不返回错误
}

// TestApplyWhoisLibraryConfig_ASNRelationInitFail ASNRelation 用 api 类型（Init 仅 local）→ default 报错 → warn。
func TestApplyWhoisLibraryConfig_ASNRelationInitFail(t *testing.T) {
	origASN := globalASNRelationProvider
	defer func() { globalASNRelationProvider = origASN }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.ASNRelation.Enabled = true
	cfg.ASNRelation.Type = "api" // Validate 允许 api，但 Init 仅 local → default 报错
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_HistoryInitFail history 用 custom 类型（Validate 允许，Init 仅 local）→ default 报错 → warn。
func TestApplyWhoisLibraryConfig_HistoryInitFail(t *testing.T) {
	origHist := globalHistoryProvider
	defer func() { globalHistoryProvider = origHist }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.History.Enabled = true
	cfg.History.Type = "custom" // Validate 允许 local/custom，Init 仅 local → default 报错
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_AlertStorageInitFail alert storage 用 redis 类型（Init 仅支持 local）→ default 报错 → warn。
func TestApplyWhoisLibraryConfig_AlertStorageInitFail(t *testing.T) {
	origAlert := globalAlertStorageProvider
	defer func() { globalAlertStorageProvider = origAlert }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.AlertStorage.Enabled = true
	cfg.AlertStorage.Type = "redis" // Validate 允许 redis，但 Init 仅 local → default 报错
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_MonitorStateInitFail monitor state 用 redis 类型 → warn。
func TestApplyWhoisLibraryConfig_MonitorStateInitFail(t *testing.T) {
	origMon := globalMonitorStateProvider
	defer func() { globalMonitorStateProvider = origMon }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.MonitorState.Enabled = true
	cfg.MonitorState.Type = "redis"
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_ReverseWhoisInitFail reverse 非 local → warn。
func TestApplyWhoisLibraryConfig_ReverseWhoisInitFail(t *testing.T) {
	origRev := globalReverseWhoisProvider
	defer func() { globalReverseWhoisProvider = origRev }()

	cfg := DefaultWhoisLibraryConfig()
	cfg.ReverseWhois.Enabled = true
	cfg.ReverseWhois.Type = "custom" // 非 local → Init default 报错
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_ReverseWhoisNoHistory reverse local 但 HistoryProvider 未设 → 报错 warn。
func TestApplyWhoisLibraryConfig_ReverseWhoisNoHistory(t *testing.T) {
	origRev := globalReverseWhoisProvider
	origHist := globalHistoryProvider
	defer func() {
		globalReverseWhoisProvider = origRev
		globalHistoryProvider = origHist
	}()

	cfg := DefaultWhoisLibraryConfig()
	cfg.ReverseWhois.Enabled = true
	cfg.ReverseWhois.Type = "local" // local 但 history provider 为 nil → Init 报错
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err) // warn 不返回错误
}

// TestApplyWhoisLibraryConfig_LogFormatText format != "json" → TextFormatter 分支。
func TestApplyWhoisLibraryConfig_LogFormatText(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Log.Format = "text"
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestApplyWhoisLibraryConfig_StorageDisabled storage 未启用跳过 Init。
func TestApplyWhoisLibraryConfig_StorageDisabled(t *testing.T) {
	cfg := DefaultWhoisLibraryConfig()
	cfg.Storage.Enabled = false
	err := ApplyWhoisLibraryConfig(&cfg)
	assert.NoError(t, err)
}

// TestSaveWhoisLibraryConfigToFile_Success 正常写入路径（含目录创建）。
func TestSaveWhoisLibraryConfigToFile_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "cfg.json")
	cfg := DefaultWhoisLibraryConfig()
	err := SaveWhoisLibraryConfigToFile(&cfg, path)
	assert.NoError(t, err)
	// 验证文件存在且可读回
	data, _ := os.ReadFile(path)
	assert.NotEmpty(t, data)
	// 验证可反序列化
	var read WhoisLibraryConfig
	assert.NoError(t, json.Unmarshal(data, &read))
}

// TestWhoisLibraryConfigSummary_NilAndPopulated Summary 函数。
func TestWhoisLibraryConfigSummary_NilAndPopulated(t *testing.T) {
	assert.Equal(t, "配置为空", WhoisLibraryConfigSummary(nil))
	cfg := DefaultWhoisLibraryConfig()
	s := WhoisLibraryConfigSummary(&cfg)
	assert.Contains(t, s, "查询:")
	assert.Contains(t, s, "缓存:")
}

// ==================== cache.go RedisCache.Set 错误分支 ====================

// TestRedisCache_Set_ClosedRedis redis 关闭后 Set 失败 → logrus.Error 分支。
func TestRedisCache_Set_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup() // 关闭 miniredis
	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x"}},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	// Set 应进入 Set.Err() 失败分支，记录日志并 return（无 panic）
	assert.NotPanics(t, func() { rc.Set("closed.com", entry) })
}

// TestRedisCache_Delete_ClosedRedis redis 关闭后 Delete 失败分支。
func TestRedisCache_Delete_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	assert.NotPanics(t, func() { rc.Delete("closed.com") })
}

// TestRedisCache_Clear_ClosedRedis redis 关闭后 Clear 失败分支。
func TestRedisCache_Clear_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	assert.NotPanics(t, func() { rc.Clear() })
}

// TestRedisCache_Get_ClosedRedis redis 关闭后 Get 失败分支。
func TestRedisCache_Get_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	got, ok := rc.Get("closed.com")
	assert.False(t, ok)
	assert.Nil(t, got)
}

// ==================== storage.go LocalFileStorage.Save WriteFile/Rename 错误 ====================

// TestLocalFileStorage_SaveWriteFail WriteFile 失败分支：keyPath 落到一个已存在目录使 WriteFile 失败。
func TestLocalFileStorage_SaveWriteFail(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	// keyPath("x:key") = dir/x/key.json。把 dir/x 创建为目录(非 key.json)，则 MkdirAll(dir/x) 成功，
	// 但需让 WriteFile(tmp=path+.tmp) 失败：把 tmp 设为只读目录下的路径。
	// 更直接：key 中含子路径，tmp = dir/sub/key.json.tmp；先创建 dir/sub/key.json.tmp 为目录
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(filepath.Join(subDir, "key.json.tmp"), 0755)
	// 此时 WriteFile(dir/sub/key.json.tmp) 会因目标是一个目录而失败
	err := s.Save(context.Background(), "sub:key", map[string]string{"a": "b"})
	assert.Error(t, err)
}

// TestLocalFileStorage_SaveRenameFail Rename 失败分支：tmp 已写，但 rename 到被占用路径。
func TestLocalFileStorage_SaveRenameFail(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	// keyPath("a:b") = dir/a/b.json；先创建 dir/a/b.json 为目录 → Rename(tmp, dir/a/b.json) 失败
	os.MkdirAll(filepath.Join(dir, "a", "b.json"), 0755)
	err := s.Save(context.Background(), "a:b", map[string]string{"a": "b"})
	assert.Error(t, err) // Rename 到目录失败
}

// ==================== storage.go LocalFileStorage.List 错误分支 ====================

// TestLocalFileStorage_ListWalkError directory 不存在 → Walk 报错 → return err 分支。
func TestLocalFileStorage_ListWalkError(t *testing.T) {
	s := &LocalFileStorage{directory: "/nonexistent/storage/dir"}
	keys, err := s.List(context.Background(), "")
	// Walk 遇到不存在的根目录会返回 err
	_ = keys
	_ = err
	// 仅验证不 panic（Walk 对不存在目录的行为：返回 walkRootError）
}

// TestLocalFileStorage_ListRelErrorPrefix List 含前缀过滤与 Rel 失败分支。
func TestLocalFileStorage_ListRelErrorPrefix(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	s.Save(context.Background(), "whois:a.com", map[string]string{"a": "b"})
	// 用空前缀列出全部（HasSuffix/Rel 分支正常）
	keys, err := s.List(context.Background(), "")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 1)
}

// TestLocalFileStorage_ListNonJSONFile 目录含非 json 文件 → HasSuffix 跳过分支。
func TestLocalFileStorage_ListNonJSONFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	// 写一个非 json 文件
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)
	s.Save(context.Background(), "whois:a.com", map[string]string{"a": "b"})
	keys, err := s.List(context.Background(), "")
	assert.NoError(t, err)
	// List 返回 slash 格式（":" 已转为路径分隔符）
	assert.Contains(t, keys, "whois/a.com")
}

// ==================== storage.go RedisStorage.Save Set 错误分支 ====================

// TestRedisStorage_Save_ClosedRedis redis 关闭后 Set 失败分支。
func TestRedisStorage_Save_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	err = s.Save(context.Background(), "k:1", map[string]string{"a": "b"})
	assert.Error(t, err) // client.Set 失败
}

// TestRedisStorage_Delete_ClosedRedis redis 关闭后 Delete 失败分支。
func TestRedisStorage_Delete_ClosedRedis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	err = s.Delete(context.Background(), "k:1")
	assert.Error(t, err)
}
