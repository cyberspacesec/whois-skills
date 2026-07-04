package whois

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

// TestLocalFileStorageSaveLoad 验证本地存储的存取往返。
func TestLocalFileStorageSaveLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalFileStorage(dir)
	if err != nil {
		t.Fatalf("NewLocalFileStorage 失败: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
	}
	key := "whois:example.com"
	if err := s.Save(ctx, key, info); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	var got whoisparser.WhoisInfo
	if err := s.Load(ctx, key, &got); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if got.Domain == nil || got.Domain.Domain != "example.com" {
		t.Errorf("往返数据不匹配: %+v", got)
	}
}

// TestLocalFileStorageLoadNotFound 验证读取不存在 key 返回错误。
func TestLocalFileStorageLoadNotFound(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	defer s.Close()
	var got whoisparser.WhoisInfo
	if err := s.Load(context.Background(), "missing", &got); err == nil {
		t.Error("期望返回 not found 错误，得到 nil")
	}
}

// TestLocalFileStorageExistsDelete 验证 Exists 与 Delete。
func TestLocalFileStorageExistsDelete(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	defer s.Close()
	ctx := context.Background()
	key := "whois:exists.example"

	if ok, _ := s.Exists(ctx, key); ok {
		t.Error("未存储前不应存在")
	}
	_ = s.Save(ctx, key, &whoisparser.WhoisInfo{})
	if ok, _ := s.Exists(ctx, key); !ok {
		t.Error("存储后应存在")
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("Delete 失败: %v", err)
	}
	if ok, _ := s.Exists(ctx, key); ok {
		t.Error("删除后不应存在")
	}
	// 删除不存在 key 不应报错
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("删除不存在 key 不应报错: %v", err)
	}
}

// TestLocalFileStorageList 验证按前缀列出 key。
func TestLocalFileStorageList(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	defer s.Close()
	ctx := context.Background()

	_ = s.Save(ctx, "whois:a.com", &whoisparser.WhoisInfo{})
	_ = s.Save(ctx, "whois:b.com", &whoisparser.WhoisInfo{})
	_ = s.Save(ctx, "ip:1.2.3.4", &IPWhoisInfo{})

	keys, err := s.List(ctx, "whois:")
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("whois: 前缀应有 2 个 key，得到 %d: %v", len(keys), keys)
	}

	ipKeys, _ := s.List(ctx, "ip:")
	if len(ipKeys) != 1 {
		t.Errorf("ip: 前缀应有 1 个 key，得到 %d", len(ipKeys))
	}
}

// TestLocalFileStorageAtomicWrite 验证写入是原子写（无残留 .tmp）。
func TestLocalFileStorageAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	defer s.Close()
	_ = s.Save(context.Background(), "whois:atomic.com", &whoisparser.WhoisInfo{})

	// 应只有 .json 文件，无 .tmp 残留
	entries, _ := os.ReadDir(filepath.Join(dir, "whois"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("发现残留 .tmp 文件: %s", e.Name())
		}
	}
}

// TestStorageProviderGlobalInjection 验证全局 StorageProvider 注入与恢复。
func TestStorageProviderGlobalInjection(t *testing.T) {
	original := globalStorageProvider
	defer func() { globalStorageProvider = original }()

	s, _ := NewLocalFileStorage(t.TempDir())
	SetStorageProvider(s)
	if GetStorageProvider() != s {
		t.Error("注入后 GetStorageProvider 应返回注入实例")
	}
	SetStorageProvider(nil)
	if GetStorageProvider() != nil {
		t.Error("Set(nil) 后应恢复为 nil（不启用持久化）")
	}
}

// TestInitStorageFromConfigLocal 验证从配置初始化本地存储。
func TestInitStorageFromConfigLocal(t *testing.T) {
	original := globalStorageProvider
	defer func() { globalStorageProvider = original }()

	dir := t.TempDir()
	if err := InitStorageFromConfig(&StorageConfig{
		Enabled:   true,
		Type:      "local",
		Directory: dir,
	}); err != nil {
		t.Fatalf("InitStorageFromConfig 失败: %v", err)
	}
	if _, ok := GetStorageProvider().(*LocalFileStorage); !ok {
		t.Errorf("应初始化 LocalFileStorage，得到 %T", GetStorageProvider())
	}
}

// TestInitStorageFromConfigDisabled 验证禁用时清空全局 provider。
func TestInitStorageFromConfigDisabled(t *testing.T) {
	original := globalStorageProvider
	defer func() { globalStorageProvider = original }()

	SetStorageProvider(&LocalFileStorage{})
	if err := InitStorageFromConfig(&StorageConfig{Enabled: false}); err != nil {
		t.Fatalf("InitStorageFromConfig 失败: %v", err)
	}
	if GetStorageProvider() != nil {
		t.Error("Enabled=false 时应清空全局 provider")
	}
}

// TestInitStorageFromConfigUnknownType 验证未知类型返回错误。
func TestInitStorageFromConfigUnknownType(t *testing.T) {
	original := globalStorageProvider
	defer func() { globalStorageProvider = original }()

	if err := InitStorageFromConfig(&StorageConfig{Enabled: true, Type: "unknown"}); err == nil {
		t.Error("未知类型应返回错误")
	}
}

// TestStorageEntryMetaTimestamps 验证存储条目带时间戳。
func TestStorageEntryMetaTimestamps(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	defer s.Close()

	before := time.Now()
	_ = s.Save(context.Background(), "whois:ts.com", &whoisparser.WhoisInfo{})
	after := time.Now()

	// 直接读取文件校验时间戳
	path := filepath.Join(dir, "whois", "ts.com.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}
	var entry WhoisStorageEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("解析 entry 失败: %v", err)
	}
	if entry.CreatedAt.Before(before) || entry.CreatedAt.After(after) {
		t.Errorf("CreatedAt 时间戳异常: %v", entry.CreatedAt)
	}
	if entry.Type != "whois" {
		t.Errorf("Type 应为 whois，得到 %s", entry.Type)
	}
}
