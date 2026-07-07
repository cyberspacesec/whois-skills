package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// 单例状态保存/恢复 helper
// ============================================================

// snapshotAPIKeyManager 保存并恢复全局 apiKeyManager 单例状态。
// 返回一个 restore 函数，调用后恢复原状态。
func snapshotAPIKeyManager(t *testing.T) func() {
	t.Helper()
	mgr := GetAPIKeyManager()
	mgr.mu.RLock()
	origConfigPath := mgr.configPath
	origKeys := make(map[string]*APIKey, len(mgr.keys))
	for k, v := range mgr.keys {
		cp := *v
		if v.ExpiresAt != nil {
			exp := *v.ExpiresAt
			cp.ExpiresAt = &exp
		}
		origKeys[k] = &cp
	}
	mgr.mu.RUnlock()
	return func() {
		mgr.mu.Lock()
		mgr.configPath = origConfigPath
		mgr.keys = origKeys
		mgr.mu.Unlock()
	}
}

// snapshotRateLimiters 保存并恢复全局 rateLimiters map
func snapshotRateLimiters(t *testing.T) func() {
	t.Helper()
	limiterMu.Lock()
	orig := make(map[string]*rateLimiter, len(rateLimiters))
	for k, v := range rateLimiters {
		cp := *v
		orig[k] = &cp
	}
	limiterMu.Unlock()
	return func() {
		limiterMu.Lock()
		rateLimiters = orig
		limiterMu.Unlock()
	}
}

// ============================================================
// InitAPIKeys 测试（操作单例，需保存恢复）
// ============================================================

func TestInitAPIKeys_Success(t *testing.T) {
	restore := snapshotAPIKeyManager(t)
	defer restore()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "apikeys.json")

	keys := []*APIKey{
		{
			ID:          "id1",
			Key:         "key1",
			Permissions: []string{"admin"},
			RateLimit:   60,
			CreatedAt:   time.Now(),
		},
		{
			ID:          "id2",
			Key:         "key2",
			Permissions: []string{"read"},
			RateLimit:   30,
			CreatedAt:   time.Now(),
		},
	}
	data, err := json.Marshal(keys)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(configPath, data, 0644))

	err = InitAPIKeys(configPath)
	assert.NoError(t, err)

	mgr := GetAPIKeyManager()
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	assert.Len(t, mgr.keys, 2)
	assert.NotNil(t, mgr.keys["key1"])
	assert.Equal(t, "id2", mgr.keys["key2"].ID)
}

func TestInitAPIKeys_FileNotExist(t *testing.T) {
	restore := snapshotAPIKeyManager(t)
	defer restore()

	err := InitAPIKeys("/nonexistent/path/apikeys.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestInitAPIKeys_BadJSON(t *testing.T) {
	restore := snapshotAPIKeyManager(t)
	defer restore()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "apikeys.json")
	assert.NoError(t, os.WriteFile(configPath, []byte("{invalid json}"), 0644))

	err := InitAPIKeys(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config")
}

// ============================================================
// LoadConfig 成功路径
// ============================================================

func TestAPIKeyManager_LoadConfig_Success(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "apikeys.json")

	keys := []*APIKey{
		{ID: "id1", Key: "key1", Permissions: []string{"read"}, RateLimit: 60},
	}
	data, err := json.Marshal(keys)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(configPath, data, 0644))

	err = mgr.LoadConfig(configPath)
	assert.NoError(t, err)
	assert.Equal(t, configPath, mgr.configPath)
	assert.Len(t, mgr.keys, 1)
	assert.Equal(t, "id1", mgr.keys["key1"].ID)
}

// ============================================================
// SaveConfig 各分支
// ============================================================

func TestAPIKeyManager_SaveConfig_EmptyConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	// configPath 为空 -> 默认 "config/apikeys.json"，但我们改用 tmpDir 防止污染
	// 通过先设置一个相对路径在 tmpDir 下工作目录来测试空分支
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	// 切换工作目录到临时目录，使默认 "config/apikeys.json" 写在临时目录下
	origWd, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(origWd)
	assert.NoError(t, os.Chdir(tmpDir))

	// configPath 为空，触发默认路径分支
	err = mgr.SaveConfig()
	assert.NoError(t, err)
	// configPath 应被设为默认值
	assert.Equal(t, "config/apikeys.json", mgr.configPath)

	// 文件应存在
	_, err = os.Stat(filepath.Join(tmpDir, "config", "apikeys.json"))
	assert.NoError(t, err)
}

func TestAPIKeyManager_SaveConfig_MkdirAllFail(t *testing.T) {
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: "/proc/cannot-create-dir/apikeys.json",
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	err := mgr.SaveConfig()
	assert.Error(t, err)
	// MkdirAll 失败 或 WriteFile 失败
	assert.True(t,
		strings.Contains(err.Error(), "创建配置目录失败") || strings.Contains(err.Error(), "保存API密钥配置失败"),
		"err should mention dir/file failure, got: %v", err)
}

func TestAPIKeyManager_SaveConfig_WriteFileFail(t *testing.T) {
	// 目标路径是已存在的目录（不是文件），WriteFile 会失败
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: tmpDir, // 这是一个目录，写入会失败
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	err := mgr.SaveConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "保存API密钥配置失败")
}

func TestAPIKeyManager_SaveConfig_NoPathSeparator(t *testing.T) {
	// configPath 不含路径分隔符（纯文件名），触发 dir="config" 分支
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	assert.NoError(t, err)
	defer os.Chdir(origWd)
	assert.NoError(t, os.Chdir(tmpDir))

	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: "apikeys.json", // 无分隔符
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	err = mgr.SaveConfig()
	assert.NoError(t, err)
	// 文件应写到当前目录(tmpDir)下的 apikeys.json
	_, err = os.Stat(filepath.Join(tmpDir, "apikeys.json"))
	assert.NoError(t, err)
	// config 目录也应被创建
	_, err = os.Stat(filepath.Join(tmpDir, "config"))
	assert.NoError(t, err)
}

func TestAPIKeyManager_SaveConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "sub", "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1", Permissions: []string{"read"}}
	mgr.keys["k2"] = &APIKey{ID: "id2", Key: "k2"}

	err := mgr.SaveConfig()
	assert.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "sub", "apikeys.json"))
	assert.NoError(t, err)
	var loaded []*APIKey
	assert.NoError(t, json.Unmarshal(data, &loaded))
	assert.Len(t, loaded, 2)
}

// ============================================================
// GenerateAPIKey 成功 SaveConfig 路径
// ============================================================

func TestAPIKeyManager_GenerateAPIKey_SaveConfigSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}

	key, err := mgr.GenerateAPIKey("desc", []string{"read", "write"}, 100)
	assert.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 100, key.RateLimit)
	assert.Equal(t, []string{"read", "write"}, key.Permissions)

	// SaveConfig 应已写入文件
	_, err = os.Stat(filepath.Join(tmpDir, "apikeys.json"))
	assert.NoError(t, err)
}

func TestAPIKeyManager_GenerateAPIKey_SaveConfigFail(t *testing.T) {
	// configPath 指向不可写位置，SaveConfig 失败但 GenerateAPIKey 仍返回 key（仅 Warn）
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: "/nonexistent-dir/apikeys.json",
	}
	key, err := mgr.GenerateAPIKey("desc", nil, 0)
	assert.NoError(t, err, "GenerateAPIKey 不应因 SaveConfig 失败而返回错误")
	assert.NotNil(t, key)
	assert.Equal(t, []string{"admin"}, key.Permissions)
	assert.Equal(t, 60, key.RateLimit)
}

// ============================================================
// EnableAPIKey / SetKeyExpiration / UpdateKeyPermissions / UpdateKeyRateLimit
// 不存在 err 分支 + 成功路径
// ============================================================

func TestAPIKeyManager_EnableAPIKey_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.EnableAPIKey("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API密钥不存在")
}

func TestAPIKeyManager_EnableAPIKey_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1", Permissions: []string{}}

	err := mgr.EnableAPIKey("k1")
	assert.NoError(t, err)
	assert.Equal(t, []string{"admin"}, mgr.keys["k1"].Permissions)
}

func TestAPIKeyManager_SetKeyExpiration_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.SetKeyExpiration("nonexistent", time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API密钥不存在")
}

func TestAPIKeyManager_SetKeyExpiration_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	exp := time.Now().Add(24 * time.Hour)
	err := mgr.SetKeyExpiration("k1", exp)
	assert.NoError(t, err)
	assert.NotNil(t, mgr.keys["k1"].ExpiresAt)
}

func TestAPIKeyManager_UpdateKeyPermissions_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.UpdateKeyPermissions("nonexistent", []string{"read"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API密钥不存在")
}

func TestAPIKeyManager_UpdateKeyPermissions_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1", Permissions: []string{"read"}}

	err := mgr.UpdateKeyPermissions("k1", []string{"read", "write"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"read", "write"}, mgr.keys["k1"].Permissions)
}

func TestAPIKeyManager_UpdateKeyRateLimit_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.UpdateKeyRateLimit("nonexistent", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API密钥不存在")
}

func TestAPIKeyManager_UpdateKeyRateLimit_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1", RateLimit: 60}

	err := mgr.UpdateKeyRateLimit("k1", 200)
	assert.NoError(t, err)
	assert.Equal(t, 200, mgr.keys["k1"].RateLimit)
}

func TestAPIKeyManager_DisableAPIKey_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1", Permissions: []string{"read"}}

	err := mgr.DisableAPIKey("k1")
	assert.NoError(t, err)
	assert.Len(t, mgr.keys["k1"].Permissions, 0)
}

func TestAPIKeyManager_DeleteAPIKey_Success(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := &APIKeyManager{
		keys:       make(map[string]*APIKey),
		configPath: filepath.Join(tmpDir, "apikeys.json"),
	}
	mgr.keys["k1"] = &APIKey{ID: "id1", Key: "k1"}

	err := mgr.DeleteAPIKey("k1")
	assert.NoError(t, err)
	_, exists := mgr.GetAPIKey("k1")
	assert.False(t, exists)
}

// ============================================================
// middleware: respondWithError / checkRateLimit 重置 / cleanup / Start
// ============================================================

func TestRespondWithError(t *testing.T) {
	w := httptest.NewRecorder()
	respondWithError(w, http.StatusForbidden, "forbidden")

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp APIResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "forbidden", resp.Error)
}

func TestCheckRateLimit_ResetWindow(t *testing.T) {
	restore := snapshotRateLimiters(t)
	defer restore()

	key := &APIKey{
		ID:         "reset-id",
		Key:        "reset-key",
		RateLimit:  2,
		Permissions: []string{"read"},
		CreatedAt:  time.Now(),
	}
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:1111"

	// 塞入一个过期的 limiter（lastReset 在 windowSize 之前），触发重置分支
	limiterMu.Lock()
	rateLimiters["reset-id:127.0.0.1:1111"] = &rateLimiter{
		lastReset:  time.Now().Add(-2 * time.Minute), // 超过 1 分钟窗口
		count:      999,                               // 已达上限
		rateLimit:  2,
		windowSize: time.Minute,
	}
	limiterMu.Unlock()

	// 由于窗口过期，应重置 count 并允许通过
	assert.True(t, checkRateLimit(key, req))
	// 重置后 count 应为 1
	limiterMu.RLock()
	l := rateLimiters["reset-id:127.0.0.1:1111"]
	limiterMu.RUnlock()
	assert.Equal(t, 1, l.count)
}

func TestCheckRateLimit_NewLimiter(t *testing.T) {
	restore := snapshotRateLimiters(t)
	defer restore()

	key := &APIKey{
		ID:         "new-id",
		Key:        "new-key",
		RateLimit:  5,
		Permissions: []string{"read"},
	}
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:2222"

	// 不存在的 limiter，应创建新的
	assert.True(t, checkRateLimit(key, req))
	limiterMu.RLock()
	l, exists := rateLimiters["new-id:10.0.0.1:2222"]
	limiterMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, 5, l.rateLimit)
	assert.Equal(t, time.Minute, l.windowSize)
}

func TestCleanupRateLimiters_RemovesExpired(t *testing.T) {
	restore := snapshotRateLimiters(t)
	defer restore()

	limiterMu.Lock()
	// 过期的（lastReset 超过 1 小时）
	rateLimiters["expired"] = &rateLimiter{
		lastReset:  time.Now().Add(-2 * time.Hour),
		rateLimit:  10,
		windowSize: time.Minute,
	}
	// 未过期的
	rateLimiters["fresh"] = &rateLimiter{
		lastReset:  time.Now(),
		rateLimit:  10,
		windowSize: time.Minute,
	}
	limiterMu.Unlock()

	cleanupRateLimiters()

	limiterMu.RLock()
	_, expiredExists := rateLimiters["expired"]
	_, freshExists := rateLimiters["fresh"]
	limiterMu.RUnlock()

	assert.False(t, expiredExists, "expired limiter should be removed")
	assert.True(t, freshExists, "fresh limiter should remain")
}

func TestCleanupRateLimiters_Empty(t *testing.T) {
	restore := snapshotRateLimiters(t)
	defer restore()

	// 清空 rateLimiters
	limiterMu.Lock()
	rateLimiters = make(map[string]*rateLimiter)
	limiterMu.Unlock()

	// 不应 panic
	assert.NotPanics(t, func() {
		cleanupRateLimiters()
	})
}

func TestStartRateLimitCleanup(t *testing.T) {
	// 不应 panic
	assert.NotPanics(t, func() {
		StartRateLimitCleanup()
	})
	// 让 goroutine 启动
	time.Sleep(50 * time.Millisecond)
}

// ============================================================
// AuthMiddleware 完整路径测试（操作单例）
// ============================================================

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	restore := snapshotAPIKeyManager(t)
	defer restore()

	mgr := GetAPIKeyManager()
	mgr.mu.Lock()
	mgr.keys["valid-key"] = &APIKey{
		ID:          "valid-id",
		Key:         "valid-key",
		Permissions: []string{"read"},
		RateLimit:   100,
		CreatedAt:   time.Now(),
	}
	mgr.mu.Unlock()

	handler := AuthMiddleware("read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RateLimitExceeded(t *testing.T) {
	restoreMgr := snapshotAPIKeyManager(t)
	defer restoreMgr()
	restoreRL := snapshotRateLimiters(t)
	defer restoreRL()

	mgr := GetAPIKeyManager()
	mgr.mu.Lock()
	mgr.keys["rl-key"] = &APIKey{
		ID:          "rl-id",
		Key:         "rl-key",
		Permissions: []string{"read"},
		RateLimit:   2,
		CreatedAt:   time.Now(),
	}
	mgr.mu.Unlock()

	handler := AuthMiddleware("read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 前 2 次通过
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Key", "rl-key")
		req.RemoteAddr = "127.0.0.1:3333"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 第 3 次应被限流
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "rl-key")
	req.RemoteAddr = "127.0.0.1:3333"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	restoreMgr := snapshotAPIKeyManager(t)
	defer restoreMgr()
	restoreRL := snapshotRateLimiters(t)
	defer restoreRL()

	mgr := GetAPIKeyManager()
	mgr.mu.Lock()
	mgr.keys["ok-key"] = &APIKey{
		ID:          "ok-id",
		Key:         "ok-key",
		Permissions: []string{"read"},
		RateLimit:   100,
		CreatedAt:   time.Now(),
	}
	mgr.mu.Unlock()

	called := false
	handler := AuthMiddleware("read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "ok-key")
	req.RemoteAddr = "127.0.0.1:4444"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)

	// 验证请求日志被记录（APIKeyID 应被填充）
	logs := GetRequestLogger().GetRecentLogs()
	assert.NotEmpty(t, logs)
	// 最后一条日志应包含 ok-id
	last := logs[len(logs)-1]
	assert.Equal(t, "ok-id", last.APIKeyID)
	assert.Equal(t, http.StatusOK, last.StatusCode)
}

func TestAuthMiddleware_LogsFillProcessTime(t *testing.T) {
	restoreMgr := snapshotAPIKeyManager(t)
	defer restoreMgr()
	restoreRL := snapshotRateLimiters(t)
	defer restoreRL()

	mgr := GetAPIKeyManager()
	mgr.mu.Lock()
	mgr.keys["log-key"] = &APIKey{
		ID:          "log-id",
		Key:         "log-key",
		Permissions: []string{"read"},
		RateLimit:   100,
		CreatedAt:   time.Now(),
	}
	mgr.mu.Unlock()

	handler := AuthMiddleware("read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/create", nil)
	req.Header.Set("X-API-Key", "log-key")
	req.RemoteAddr = "127.0.0.1:5555"
	// 同时验证 getClientIP 在中间件中的使用
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	logs := GetRequestLogger().GetRecentLogs()
	last := logs[len(logs)-1]
	assert.Equal(t, "POST", last.Method)
	assert.Equal(t, "/create", last.Path)
	assert.Equal(t, "9.9.9.9", last.ClientIP) // 来自 X-Forwarded-For
	assert.Equal(t, http.StatusCreated, last.StatusCode)
	assert.GreaterOrEqual(t, last.ProcessTime, int64(0))
}

// ============================================================
// GetRequestLogger 单例
// ============================================================

func TestGetRequestLogger_Singleton(t *testing.T) {
	l1 := GetRequestLogger()
	l2 := GetRequestLogger()
	assert.Same(t, l1, l2)
}

// ============================================================
// 并发安全 smoke test（rateLimiters + AddLog）
// ============================================================

func TestRateLimiters_ConcurrentAccess(t *testing.T) {
	restore := snapshotRateLimiters(t)
	defer restore()

	var wg sync.WaitGroup
	key := &APIKey{
		ID:         "conc-id",
		Key:        "conc-key",
		RateLimit:  10000,
		Permissions: []string{"read"},
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "127.0.0.1:" + string(rune('0'+idx))
			checkRateLimit(key, req)
		}(i)
	}
	wg.Wait()
	// 不 panic 即可
}
