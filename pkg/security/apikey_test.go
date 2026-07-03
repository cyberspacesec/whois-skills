package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetAPIKeyManager(t *testing.T) {
	mgr := GetAPIKeyManager()
	if mgr == nil {
		t.Fatal("GetAPIKeyManager() returned nil")
	}
}

func TestAPIKeyManager_ValidateKey(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{
		ID:          "test-id",
		Key:         "test-key",
		Permissions: []string{"read", "write"},
		RateLimit:   60,
		CreatedAt:   time.Now(),
	}

	// Valid key with correct permission
	key, err := mgr.ValidateKey("test-key", "read")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if key.ID != "test-id" {
		t.Errorf("Key ID = %s, want test-id", key.ID)
	}

	// Invalid key
	_, err = mgr.ValidateKey("invalid-key", "read")
	if err == nil {
		t.Error("Expected error for invalid key")
	}

	// Insufficient permissions
	_, err = mgr.ValidateKey("test-key", "admin")
	if err == nil {
		t.Error("Expected error for insufficient permissions")
	}
}

func TestAPIKeyManager_ValidateKey_Expired(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	expired := time.Now().Add(-time.Hour)
	mgr.keys["expired-key"] = &APIKey{
		ID:          "expired-id",
		Key:         "expired-key",
		Permissions: []string{"admin"},
		ExpiresAt:   &expired,
	}

	_, err := mgr.ValidateKey("expired-key", "admin")
	if err == nil {
		t.Error("Expected error for expired key")
	}
}

func TestAPIKeyManager_ValidateKey_AdminPermission(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["admin-key"] = &APIKey{
		ID:          "admin-id",
		Key:         "admin-key",
		Permissions: []string{"admin"},
		CreatedAt:   time.Now(),
	}

	// Admin should have access to any permission
	key, err := mgr.ValidateKey("admin-key", "any-permission")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if key.ID != "admin-id" {
		t.Errorf("Key ID = %s, want admin-id", key.ID)
	}
}

func TestAPIKeyManager_GenerateAPIKey(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	key, err := mgr.GenerateAPIKey("test", []string{"read"}, 30)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if key.Key == "" {
		t.Error("Key should not be empty")
	}
	if key.ID == "" {
		t.Error("ID should not be empty")
	}
	if key.RateLimit != 30 {
		t.Errorf("RateLimit = %d, want 30", key.RateLimit)
	}

	// Key should be stored in manager
	stored, exists := mgr.keys[key.Key]
	if !exists {
		t.Error("Key should be stored in manager")
	}
	if stored.ID != key.ID {
		t.Error("Stored key ID should match")
	}
}

func TestAPIKeyManager_GenerateAPIKey_Defaults(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	key, err := mgr.GenerateAPIKey("test", nil, 0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(key.Permissions) != 1 || key.Permissions[0] != "admin" {
		t.Errorf("Default permissions = %v, want [admin]", key.Permissions)
	}
	if key.RateLimit != 60 {
		t.Errorf("Default RateLimit = %d, want 60", key.RateLimit)
	}
}

func TestAPIKeyManager_GetAPIKey(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{
		ID:  "test-id",
		Key: "test-key",
	}

	key, exists := mgr.GetAPIKey("test-key")
	if !exists {
		t.Error("Key should exist")
	}
	if key.ID != "test-id" {
		t.Errorf("Key ID = %s, want test-id", key.ID)
	}

	_, exists = mgr.GetAPIKey("nonexistent")
	if exists {
		t.Error("Nonexistent key should not exist")
	}
}

func TestAPIKeyManager_ListAPIKeys(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["key1"] = &APIKey{ID: "id1", Key: "key1"}
	mgr.keys["key2"] = &APIKey{ID: "id2", Key: "key2"}

	keys := mgr.ListAPIKeys()
	if len(keys) != 2 {
		t.Errorf("Key count = %d, want 2", len(keys))
	}
}

func TestAPIKeyManager_DisableAPIKey(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{
		ID:          "test-id",
		Key:         "test-key",
		Permissions: []string{"read"},
	}

	_ = mgr.DisableAPIKey("test-key")
	// May fail on SaveConfig if no config path, but key permissions should be cleared
	key, _ := mgr.GetAPIKey("test-key")
	if len(key.Permissions) != 0 {
		t.Errorf("Permissions = %v, want empty after disable", key.Permissions)
	}
}

func TestAPIKeyManager_DisableAPIKey_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.DisableAPIKey("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestAPIKeyManager_DeleteAPIKey(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{ID: "test-id", Key: "test-key"}

	_ = mgr.DeleteAPIKey("test-key")
	// Key should be removed from map
	_, exists := mgr.GetAPIKey("test-key")
	if exists {
		t.Error("Key should be deleted")
	}
}

func TestAPIKeyManager_DeleteAPIKey_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.DeleteAPIKey("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestAPIKeyManager_SetKeyExpiration(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{ID: "test-id", Key: "test-key"}

	expiresAt := time.Now().Add(24 * time.Hour)
	_ = mgr.SetKeyExpiration("test-key", expiresAt)

	key, _ := mgr.GetAPIKey("test-key")
	if key.ExpiresAt == nil {
		t.Error("ExpiresAt should be set")
	}
}

func TestAPIKeyManager_UpdateKeyPermissions(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{ID: "test-id", Key: "test-key", Permissions: []string{"read"}}

	_ = mgr.UpdateKeyPermissions("test-key", []string{"read", "write", "admin"})

	key, _ := mgr.GetAPIKey("test-key")
	if len(key.Permissions) != 3 {
		t.Errorf("Permissions count = %d, want 3", len(key.Permissions))
	}
}

func TestAPIKeyManager_UpdateKeyRateLimit(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}

	mgr.keys["test-key"] = &APIKey{ID: "test-id", Key: "test-key", RateLimit: 60}

	_ = mgr.UpdateKeyRateLimit("test-key", 120)

	key, _ := mgr.GetAPIKey("test-key")
	if key.RateLimit != 120 {
		t.Errorf("RateLimit = %d, want 120", key.RateLimit)
	}
}

func TestAPIKeyManager_LoadConfig_NotFound(t *testing.T) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.LoadConfig("/nonexistent/apikeys.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAPIKeyManager_LoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/apikeys.json"
	os.WriteFile(configPath, []byte("invalid json"), 0644)

	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	err := mgr.LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestAPIKey_Fields(t *testing.T) {
	now := time.Now()
	key := &APIKey{
		ID:          "test-id",
		Key:         "test-key",
		Permissions: []string{"read", "write"},
		RateLimit:   120,
		CreatedAt:   now,
	}

	if key.ID != "test-id" {
		t.Errorf("ID = %s, want test-id", key.ID)
	}
	if key.RateLimit != 120 {
		t.Errorf("RateLimit = %d, want 120", key.RateLimit)
	}
	if len(key.Permissions) != 2 {
		t.Errorf("Permissions count = %d, want 2", len(key.Permissions))
	}
}

func TestAPIResponse_Fields(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Data:    "test data",
		Message: "ok",
	}
	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Data != "test data" {
		t.Errorf("Data = %v, want test data", resp.Data)
	}
}

func TestSendErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	SendErrorResponse(w, http.StatusBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Error("Success should be false for error")
	}
	if resp.Error != "invalid input" {
		t.Errorf("Error = %s, want invalid input", resp.Error)
	}
}

func TestSendSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	SendSuccessResponse(w, map[string]string{"key": "value"}, "success")

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Message != "success" {
		t.Errorf("Message = %s, want success", resp.Message)
	}
}

func TestNewRequestLogger(t *testing.T) {
	logger := NewRequestLogger(100)
	if logger == nil {
		t.Fatal("NewRequestLogger() returned nil")
	}
	if logger.maxLogs != 100 {
		t.Errorf("maxLogs = %d, want 100", logger.maxLogs)
	}
}

func TestRequestLogger_AddLog(t *testing.T) {
	logger := NewRequestLogger(5)

	for i := 0; i < 10; i++ {
		logger.AddLog(RequestLog{
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
		})
	}

	logs := logger.GetRecentLogs()
	if len(logs) > 5 {
		t.Errorf("Log count = %d, should be capped at 5", len(logs))
	}
}

func TestRequestLogger_GetRecentLogs(t *testing.T) {
	logger := NewRequestLogger(100)

	logger.AddLog(RequestLog{Method: "GET", Path: "/test1"})
	logger.AddLog(RequestLog{Method: "POST", Path: "/test2"})

	logs := logger.GetRecentLogs()
	if len(logs) != 2 {
		t.Errorf("Log count = %d, want 2", len(logs))
	}
}

func TestRequestLog_Fields(t *testing.T) {
	log := RequestLog{
		Method:     "GET",
		Path:       "/api/whois",
		ClientIP:   "127.0.0.1",
		StatusCode: 200,
	}

	if log.Method != "GET" {
		t.Errorf("Method = %s, want GET", log.Method)
	}
	if log.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", log.StatusCode)
	}
}

func TestAuthMiddleware_MissingKey(t *testing.T) {
	handler := AuthMiddleware("read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func BenchmarkGenerateAPIKey(b *testing.B) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.GenerateAPIKey("test", []string{"read"}, 60)
	}
}

func BenchmarkValidateKey(b *testing.B) {
	mgr := &APIKeyManager{keys: make(map[string]*APIKey)}
	mgr.keys["test-key"] = &APIKey{
		ID:          "test-id",
		Key:         "test-key",
		Permissions: []string{"admin"},
		CreatedAt:   time.Now(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.ValidateKey("test-key", "read")
	}
}

// ============================================================
// middleware 额外测试
// ============================================================

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")

	ip := getClientIP(req)
	assert.Equal(t, "10.0.0.1", ip)
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "172.16.0.1:12345"

	ip := getClientIP(req)
	assert.Equal(t, "172.16.0.1:12345", ip)
}

func TestGetClientIP_Priority(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "10.0.0.1")

	ip := getClientIP(req)
	assert.Equal(t, "192.168.1.1", ip) // X-Forwarded-For 优先
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	n, err := rw.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, http.StatusOK, rw.statusCode) // 默认200
}

func TestRequestLogger_MaxLogsOverflow(t *testing.T) {
	logger := NewRequestLogger(2)

	for i := 0; i < 5; i++ {
		logger.AddLog(RequestLog{Method: "GET", Path: "/api/test"})
	}

	logs := logger.GetRecentLogs()
	assert.Len(t, logs, 2)
}

func TestRequestLogger_GetRecentLogs_Copy(t *testing.T) {
	logger := NewRequestLogger(10)
	logger.AddLog(RequestLog{Method: "GET", Path: "/api/test"})

	logs := logger.GetRecentLogs()
	logs[0].Method = "MODIFIED"

	originalLogs := logger.GetRecentLogs()
	assert.Equal(t, "GET", originalLogs[0].Method)
}

func TestCheckRateLimit_Basic(t *testing.T) {
	key := &APIKey{
		ID:         "test-id",
		Key:        "test-key",
		RateLimit:  5,
		Permissions: []string{"read"},
		CreatedAt:  time.Now(),
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// 前5次应通过
	for i := 0; i < 5; i++ {
		assert.True(t, checkRateLimit(key, req))
	}

	// 第6次应被限流
	assert.False(t, checkRateLimit(key, req))
}
