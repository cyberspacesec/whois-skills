package security

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// APIKey API密钥信息
type APIKey struct {
	// API密钥ID
	ID string `json:"id"`

	// API密钥
	Key string `json:"key"`

	// 权限列表
	Permissions []string `json:"permissions"`

	// 速率限制（每分钟请求数）
	RateLimit int `json:"rate_limit"`

	// 创建时间
	CreatedAt time.Time `json:"created_at"`

	// 过期时间（可选）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// APIKeyManager API密钥管理器
type APIKeyManager struct {
	mu sync.RWMutex

	// API密钥映射表
	keys map[string]*APIKey

	// 配置文件路径
	configPath string
}

var (
	defaultManager *APIKeyManager
	managerOnce    sync.Once
)

// GetAPIKeyManager 获取API密钥管理器实例
func GetAPIKeyManager() *APIKeyManager {
	managerOnce.Do(func() {
		defaultManager = &APIKeyManager{
			keys: make(map[string]*APIKey),
		}
	})
	return defaultManager
}

// InitAPIKeys 从文件初始化API密钥
func InitAPIKeys(configFile string) error {
	manager := GetAPIKeyManager()

	// 读取配置文件
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// 解析配置
	var keys []*APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	// 更新密钥映射表
	manager.mu.Lock()
	defer manager.mu.Unlock()

	manager.keys = make(map[string]*APIKey)
	for _, key := range keys {
		manager.keys[key.Key] = key
	}

	return nil
}

// ValidateKey 验证API密钥并检查权限
func (m *APIKeyManager) ValidateKey(apiKey string, requiredPermission string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找API密钥
	key, exists := m.keys[apiKey]
	if !exists {
		return nil, fmt.Errorf("invalid API key")
	}

	// 检查是否过期
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, fmt.Errorf("API key has expired")
	}

	// 检查权限
	hasPermission := false
	for _, perm := range key.Permissions {
		if perm == requiredPermission || perm == "admin" {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		return nil, fmt.Errorf("insufficient permissions")
	}

	return key, nil
}

// GenerateAPIKey 生成新的API密钥
func (m *APIKeyManager) GenerateAPIKey(description string, permissions []string, rateLimit int) (*APIKey, error) {
	// 生成随机密钥
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}
	key := base64.URLEncoding.EncodeToString(keyBytes)

	// 生成密钥ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("生成密钥ID失败: %w", err)
	}
	id := base64.URLEncoding.EncodeToString(idBytes)[:12]

	// 如果没有指定权限，使用默认权限
	if len(permissions) == 0 {
		permissions = []string{"admin"}
	}

	// 如果没有指定速率限制，使用默认值
	if rateLimit <= 0 {
		rateLimit = 60 // 默认每分钟60次请求
	}

	apiKey := &APIKey{
		ID:          id,
		Key:         key,
		Permissions: permissions,
		RateLimit:   rateLimit,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.keys[key] = apiKey
	m.mu.Unlock()

	// 保存到配置文件
	if err := m.SaveConfig(); err != nil {
		logrus.Warnf("保存API密钥配置失败: %v", err)
	}

	return apiKey, nil
}

// LoadConfig 从配置文件加载API密钥
func (m *APIKeyManager) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("配置文件不存在: %s", configPath)
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var keys []*APIKey
	if err := json.Unmarshal(data, &keys); err != nil {
		return fmt.Errorf("解析API密钥配置失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.configPath = configPath
	m.keys = make(map[string]*APIKey, len(keys))
	for _, key := range keys {
		m.keys[key.Key] = key
	}

	logrus.Infof("已加载 %d 个API密钥", len(keys))
	return nil
}

// SaveConfig 保存API密钥到配置文件
func (m *APIKeyManager) SaveConfig() error {
	m.mu.RLock()
	keys := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	configPath := m.configPath
	m.mu.RUnlock()

	if configPath == "" {
		configPath = "config/apikeys.json"
		m.mu.Lock()
		m.configPath = configPath
		m.mu.Unlock()
	}

	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化API密钥失败: %w", err)
	}

	// 确保目录存在
	dir := os.ExpandEnv(filepath.Dir(configPath))
	if !strings.ContainsRune(os.ExpandEnv(configPath), os.PathSeparator) {
		dir = "config"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("保存API密钥配置失败: %w", err)
	}

	logrus.Infof("已保存 %d 个API密钥到 %s", len(keys), configPath)
	return nil
}

// GetAPIKey 获取指定密钥的信息
func (m *APIKeyManager) GetAPIKey(key string) (*APIKey, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apiKey, exists := m.keys[key]
	return apiKey, exists
}

// ListAPIKeys 列出所有API密钥
func (m *APIKeyManager) ListAPIKeys() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*APIKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	return keys
}

// DisableAPIKey 禁用API密钥
func (m *APIKeyManager) DisableAPIKey(key string) error {
	m.mu.Lock()
	apiKey, exists := m.keys[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	apiKey.Permissions = []string{}
	m.mu.Unlock()
	return m.SaveConfig()
}

// EnableAPIKey 启用API密钥
func (m *APIKeyManager) EnableAPIKey(key string) error {
	m.mu.Lock()
	apiKey, exists := m.keys[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	apiKey.Permissions = []string{"admin"}
	m.mu.Unlock()
	return m.SaveConfig()
}

// DeleteAPIKey 删除API密钥
func (m *APIKeyManager) DeleteAPIKey(key string) error {
	m.mu.Lock()
	if _, exists := m.keys[key]; !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	delete(m.keys, key)
	m.mu.Unlock()
	return m.SaveConfig()
}

// SetKeyExpiration 设置API密钥过期时间
func (m *APIKeyManager) SetKeyExpiration(key string, expiresAt time.Time) error {
	m.mu.Lock()
	apiKey, exists := m.keys[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	apiKey.ExpiresAt = &expiresAt
	m.mu.Unlock()
	return m.SaveConfig()
}

// UpdateKeyPermissions 更新API密钥权限
func (m *APIKeyManager) UpdateKeyPermissions(key string, permissions []string) error {
	m.mu.Lock()
	apiKey, exists := m.keys[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	apiKey.Permissions = permissions
	m.mu.Unlock()
	return m.SaveConfig()
}

// UpdateKeyRateLimit 更新API密钥速率限制
func (m *APIKeyManager) UpdateKeyRateLimit(key string, rateLimit int) error {
	m.mu.Lock()
	apiKey, exists := m.keys[key]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("API密钥不存在")
	}
	apiKey.RateLimit = rateLimit
	m.mu.Unlock()
	return m.SaveConfig()
}
