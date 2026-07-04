package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	whoisparser "github.com/likexian/whois-parser"
)

// ============================================================================
// Storage 抽象层
//
// 与 Cache（内存缓存）互补，Storage 是持久化存储抽象，让上层可把 WHOIS 数据
// 落本地文件、Redis、或接入 ES/ClickHouse/图数据库。上层通过 SetStorageProvider
// 注入自定义实现。
//
// 内置实现：
// - LocalFileStorage：本地 JSON 文件存储（按 key 分文件）
// - RedisStorage：Redis 存储（key → JSON value）
// ============================================================================

// StorageProvider 持久化存储提供者接口。
// key 通常是 "whois:<domain>" / "ip:<ip>" / "asn:<asn>" / "history:<domain>:<ts>"
type StorageProvider interface {
	// Save 存储 data 到 key，data 为结构化对象（会被 JSON 序列化）。
	Save(ctx context.Context, key string, data interface{}) error
	// Load 从 key 读取数据到 out（out 需为指针）。
	Load(ctx context.Context, key string, out interface{}) error
	// Delete 删除 key。
	Delete(ctx context.Context, key string) error
	// Exists 检查 key 是否存在。
	Exists(ctx context.Context, key string) (bool, error)
	// List 按 prefix 列出所有 key（用于批量查询）。
	List(ctx context.Context, prefix string) ([]string, error)
	// Close 关闭存储连接（如有）。
	Close() error
}

// WhoisStorageEntry 存储条目（包含元信息）。
type WhoisStorageEntry struct {
	Key       string                 `json:"key"`
	Type      string                 `json:"type"` // whois/ip/asn/history
	Data      json.RawMessage        `json:"data"` // 原始 JSON（WhoisInfo/ASNDetail 等）
	Meta      map[string]interface{} `json:"meta,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// StorageConfig 存储配置。
type StorageConfig struct {
	// 是否启用持久化存储
	Enabled bool `json:"enabled"`

	// 存储类型 (local/redis)
	Type string `json:"type"`

	// 本地存储目录（type=local）
	Directory string `json:"directory,omitempty"`

	// Redis 配置（type=redis）
	RedisConfig *RedisConfig `json:"redis,omitempty"`
}

// ---- 全局 Storage Provider ----

var globalStorageProvider StorageProvider

// GetStorageProvider 返回全局 StorageProvider（懒加载默认实现）。
// 默认为 nil（不启用持久化），上层需显式 SetStorageProvider。
func GetStorageProvider() StorageProvider {
	return globalStorageProvider
}

// SetStorageProvider 注入自定义 StorageProvider。
// 传 nil 表示禁用持久化。
func SetStorageProvider(p StorageProvider) {
	globalStorageProvider = p
}

// InitStorageFromConfig 从配置初始化全局 StorageProvider。
func InitStorageFromConfig(cfg *StorageConfig) error {
	if !cfg.Enabled {
		globalStorageProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		p, err := NewLocalFileStorage(cfg.Directory)
		if err != nil {
			return err
		}
		globalStorageProvider = p
		return nil
	case "redis":
		p, err := NewRedisStorage(cfg.RedisConfig)
		if err != nil {
			return err
		}
		globalStorageProvider = p
		return nil
	default:
		return fmt.Errorf("未知存储类型: %s", cfg.Type)
	}
}

// ---- LocalFileStorage 本地文件存储 ----

// LocalFileStorage 本地 JSON 文件存储。
type LocalFileStorage struct {
	mu        sync.RWMutex
	directory string
}

// NewLocalFileStorage 创建本地文件存储。
func NewLocalFileStorage(directory string) (*LocalFileStorage, error) {
	if directory == "" {
		directory = "data/storage"
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	return &LocalFileStorage{directory: directory}, nil
}

func (s *LocalFileStorage) keyPath(key string) string {
	// key 中的 ":" 替换为 "/" 形成子目录，避免单目录文件过多
	safeKey := strings.ReplaceAll(key, ":", string(filepath.Separator))
	return filepath.Join(s.directory, safeKey+".json")
}

// Save 存储数据到文件。
func (s *LocalFileStorage) Save(ctx context.Context, key string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.keyPath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	entry := WhoisStorageEntry{
		Key:       key,
		Data:      json.RawMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// 判断 data 类型，设置 Type 字段
	switch data.(type) {
	case *whoisparser.WhoisInfo:
		entry.Type = "whois"
	case *ASNDetail:
		entry.Type = "asn"
	case *IPWhoisInfo:
		entry.Type = "ip"
	default:
		entry.Type = "unknown"
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	entry.Data = b

	raw, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	// 原子写：tmp + rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load 从文件读取数据。
func (s *LocalFileStorage) Load(ctx context.Context, key string, out interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.keyPath(key)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key not found: %s", key)
		}
		return err
	}
	var entry WhoisStorageEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return err
	}
	return json.Unmarshal(entry.Data, out)
}

// Delete 删除文件。
func (s *LocalFileStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := s.keyPath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Exists 检查文件是否存在。
func (s *LocalFileStorage) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path := s.keyPath(key)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// List 按前缀列出所有 key（扫描目录）。
func (s *LocalFileStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 前缀中的冒号转换为路径分隔符，与 keyPath 一致
	prefixPath := strings.ReplaceAll(prefix, ":", string(filepath.Separator))
	var keys []string
	err := filepath.Walk(s.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		// 转换路径为 key
		rel, err := filepath.Rel(s.directory, path)
		if err != nil {
			return nil
		}
		// rel 是路径格式，直接与 prefixPath 比较；返回时转为 slash 格式
		if strings.HasPrefix(rel, prefixPath) {
			// 去掉 .json 后转回 slash 格式（用 / 替换路径分隔符）
			key := strings.TrimSuffix(rel, ".json")
			keys = append(keys, filepath.ToSlash(key))
		}
		return nil
	})
	return keys, err
}

// Close 无操作。
func (s *LocalFileStorage) Close() error { return nil }

// ---- RedisStorage Redis 存储 ----

// RedisStorage Redis 存储。
type RedisStorage struct {
	client *redis.Client
	mu     sync.RWMutex
}

// NewRedisStorage 创建 Redis 存储。
func NewRedisStorage(cfg *RedisConfig) (*RedisStorage, error) {
	if cfg == nil {
		cfg = &RedisConfig{Addr: "localhost:6379"}
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}
	return &RedisStorage{client: client}, nil
}

func redisKey(key string) string {
	return "whois:" + key
}

// Save 存储到 Redis。
func (s *RedisStorage) Save(ctx context.Context, key string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := WhoisStorageEntry{
		Key:       key,
		Data:      json.RawMessage{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	switch data.(type) {
	case *whoisparser.WhoisInfo:
		entry.Type = "whois"
	case *ASNDetail:
		entry.Type = "asn"
	case *IPWhoisInfo:
		entry.Type = "ip"
	default:
		entry.Type = "unknown"
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	entry.Data = b

	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, redisKey(key), raw, 0).Err()
}

// Load 从 Redis 读取。
func (s *RedisStorage) Load(ctx context.Context, key string, out interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := s.client.Get(ctx, redisKey(key)).Bytes()
	if err == redis.Nil {
		return fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return err
	}
	var entry WhoisStorageEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return err
	}
	return json.Unmarshal(entry.Data, out)
}

// Delete 删除 Redis key。
func (s *RedisStorage) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, redisKey(key)).Err()
}

// Exists 检查 Redis key 是否存在。
func (s *RedisStorage) Exists(ctx context.Context, key string) (bool, error) {
	n, err := s.client.Exists(ctx, redisKey(key)).Result()
	return n > 0, err
}

// List 按前缀扫描 Redis keys。
func (s *RedisStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	iter := s.client.Scan(ctx, 0, "whois:"+prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		k := iter.Val()
		keys = append(keys, strings.TrimPrefix(k, "whois:"))
	}
	return keys, iter.Err()
}

// Close 关闭 Redis 连接。
func (s *RedisStorage) Close() error {
	return s.client.Close()
}