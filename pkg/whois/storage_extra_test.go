package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- redisKey ----

func TestRedisKey(t *testing.T) {
	assert.Equal(t, "whois:example.com", redisKey("example.com"))
}

// ---- NewLocalFileStorage ----

func TestNewLocalFileStorage_EmptyDir(t *testing.T) {
	s, err := NewLocalFileStorage("")
	assert.NoError(t, err)
	assert.NotNil(t, s)
	// 默认目录 data/storage
	assert.Equal(t, "data/storage", s.directory)
	// 清理
	os.RemoveAll("data/storage")
}

func TestNewLocalFileStorage_MkdirFail(t *testing.T) {
	// 用一个文件路径作为目录 → MkdirAll 失败
	// 先创建一个文件
	f := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(f, []byte("x"), 0644)
	_, err := NewLocalFileStorage(f)
	assert.Error(t, err)
}

// ---- LocalFileStorage.Save type switch ----

func TestLocalFileStorage_SaveWhoisInfo(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x"}}
	err := s.Save(context.Background(), "whois:x.com", info)
	assert.NoError(t, err)
}

func TestLocalFileStorage_SaveASNDetail(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	err := s.Save(context.Background(), "asn:12345", &ASNDetail{ASN: 12345})
	assert.NoError(t, err)
}

func TestLocalFileStorage_SaveIPWhoisInfo(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	err := s.Save(context.Background(), "ip:1.2.3.4", &IPWhoisInfo{RIR: "arin"})
	assert.NoError(t, err)
}

func TestLocalFileStorage_SaveUnknownType(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	err := s.Save(context.Background(), "x:key", map[string]string{"a": "b"})
	assert.NoError(t, err)
}

func TestLocalFileStorage_SaveMarshalFail(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	// chan 无法序列化
	err := s.Save(context.Background(), "x:bad", make(chan int))
	assert.Error(t, err)
}

func TestLocalFileStorage_SaveMkdirFail(t *testing.T) {
	// directory 指向一个已存在文件，keyPath 子目录创建失败
	dir := t.TempDir()
	// 占用 keyPath 中的一级为文件，使 MkdirAll 失败
	s, _ := NewLocalFileStorage(dir)
	// 创建 dir/whois 为文件
	os.WriteFile(filepath.Join(dir, "whois"), []byte("x"), 0644)
	err := s.Save(context.Background(), "whois:x.com", map[string]string{"a": "b"})
	assert.Error(t, err)
}

// ---- LocalFileStorage.Load ----

func TestLocalFileStorage_LoadCorruptJSON(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	// 写入非法 JSON 文件
	path := s.keyPath("bad:key")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("not json"), 0644)
	var out map[string]string
	err := s.Load(context.Background(), "bad:key", &out)
	assert.Error(t, err)
}

func TestLocalFileStorage_LoadOtherError(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	// keyPath 指向目录（非文件）→ ReadFile 失败（非 IsNotExist）
	path := s.keyPath("dir:key")
	os.MkdirAll(path, 0755) // path 本身是目录
	var out map[string]string
	err := s.Load(context.Background(), "dir:key", &out)
	assert.Error(t, err)
}

// ---- LocalFileStorage.Delete 非存在 key（IsNotExist 分支）----

func TestLocalFileStorage_DeleteNonExistent(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	err := s.Delete(context.Background(), "nope:key")
	assert.NoError(t, err)
}

// ---- LocalFileStorage.Exists ----

func TestLocalFileStorage_Exists(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	exists, err := s.Exists(context.Background(), "k:x")
	assert.NoError(t, err)
	assert.False(t, exists)
	s.Save(context.Background(), "k:x", map[string]string{"a": "b"})
	exists, err = s.Exists(context.Background(), "k:x")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalFileStorage_Close(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	assert.NoError(t, s.Close())
}

// ---- LocalFileStorage.List Rel 失败分支 ----

func TestLocalFileStorage_ListPrefix(t *testing.T) {
	s, _ := NewLocalFileStorage(t.TempDir())
	s.Save(context.Background(), "whois:a.com", map[string]string{"a": "b"})
	s.Save(context.Background(), "whois:b.com", map[string]string{"a": "b"})
	s.Save(context.Background(), "ip:1.2.3.4", map[string]string{"a": "b"})
	keys, err := s.List(context.Background(), "whois:")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 2)
}

// ---- InitStorageFromConfig redis ----

func TestInitStorageFromConfig_Redis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	defer cleanup()
	orig := globalStorageProvider
	defer func() { globalStorageProvider = orig }()
	err := InitStorageFromConfig(&StorageConfig{Enabled: true, Type: "redis", RedisConfig: &RedisConfig{Addr: addr}})
	assert.NoError(t, err)
	assert.NotNil(t, globalStorageProvider)
}

func TestInitStorageFromConfig_RedisFail(t *testing.T) {
	orig := globalStorageProvider
	defer func() { globalStorageProvider = orig }()
	err := InitStorageFromConfig(&StorageConfig{Enabled: true, Type: "redis", RedisConfig: &RedisConfig{Addr: "127.0.0.1:1"}})
	assert.Error(t, err)
}

func TestInitStorageFromConfig_LocalFail(t *testing.T) {
	orig := globalStorageProvider
	defer func() { globalStorageProvider = orig }()
	f := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(f, []byte("x"), 0644)
	err := InitStorageFromConfig(&StorageConfig{Enabled: true, Type: "local", Directory: f})
	assert.Error(t, err)
}

// ---- RedisStorage 全方法 ----

func newTestRedisStorage(t *testing.T) *RedisStorage {
	t.Helper()
	addr, cleanup := newMiniredis(t)
	t.Cleanup(cleanup)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	return s
}

func TestNewRedisStorage_NilConfig(t *testing.T) {
	// nil config → 默认 localhost:6379，Ping 失败（无 redis）
	_, err := NewRedisStorage(nil)
	// 若本机无 redis，则连接失败；若有 redis 则成功。仅断言不 panic。
	_ = err
}

func TestNewRedisStorage_PingFail(t *testing.T) {
	_, err := NewRedisStorage(&RedisConfig{Addr: "127.0.0.1:1"})
	assert.Error(t, err)
}

func TestRedisStorage_SaveLoad(t *testing.T) {
	s := newTestRedisStorage(t)
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x"}}
	err := s.Save(context.Background(), "whois:x.com", info)
	assert.NoError(t, err)
	var out whoisparser.WhoisInfo
	err = s.Load(context.Background(), "whois:x.com", &out)
	assert.NoError(t, err)
	assert.Equal(t, "x", out.Domain.Domain)
}

func TestRedisStorage_SaveTypes(t *testing.T) {
	s := newTestRedisStorage(t)
	assert.NoError(t, s.Save(context.Background(), "asn:1", &ASNDetail{ASN: 1}))
	assert.NoError(t, s.Save(context.Background(), "ip:1", &IPWhoisInfo{RIR: "arin"}))
	assert.NoError(t, s.Save(context.Background(), "x:1", map[string]string{"a": "b"}))
}

func TestRedisStorage_SaveMarshalFail(t *testing.T) {
	s := newTestRedisStorage(t)
	err := s.Save(context.Background(), "x:bad", make(chan int))
	assert.Error(t, err)
}

func TestRedisStorage_LoadNotFound(t *testing.T) {
	s := newTestRedisStorage(t)
	var out map[string]string
	err := s.Load(context.Background(), "nope", &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestRedisStorage_LoadCorruptData(t *testing.T) {
	s := newTestRedisStorage(t)
	// 直接写入非法 JSON
	s.client.Set(context.Background(), redisKey("bad"), "not-json", 0)
	var out map[string]string
	err := s.Load(context.Background(), "bad", &out)
	assert.Error(t, err)
}

func TestRedisStorage_Delete(t *testing.T) {
	s := newTestRedisStorage(t)
	s.Save(context.Background(), "k:1", map[string]string{"a": "b"})
	err := s.Delete(context.Background(), "k:1")
	assert.NoError(t, err)
	exists, _ := s.Exists(context.Background(), "k:1")
	assert.False(t, exists)
}

func TestRedisStorage_Exists(t *testing.T) {
	s := newTestRedisStorage(t)
	exists, err := s.Exists(context.Background(), "k:none")
	assert.NoError(t, err)
	assert.False(t, exists)
	s.Save(context.Background(), "k:1", map[string]string{"a": "b"})
	exists, err = s.Exists(context.Background(), "k:1")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisStorage_List(t *testing.T) {
	s := newTestRedisStorage(t)
	s.Save(context.Background(), "whois:a.com", map[string]string{"a": "b"})
	s.Save(context.Background(), "whois:b.com", map[string]string{"a": "b"})
	s.Save(context.Background(), "ip:1.2.3.4", map[string]string{"a": "b"})
	keys, err := s.List(context.Background(), "whois:")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 2)
}

func TestRedisStorage_Close(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	assert.NoError(t, err)
	cleanup()
	// Close 在 miniredis 关闭后调用，可能成功或失败；仅验证不 panic
	_ = s.Close()
}

func TestRedisStorage_CloseNormal(t *testing.T) {
	s := newTestRedisStorage(t)
	// miniredis 仍存活
	err := s.Close()
	assert.NoError(t, err)
}

// ---- SetStorageProvider ----

func TestSetStorageProvider(t *testing.T) {
	orig := globalStorageProvider
	defer func() { globalStorageProvider = orig }()
	s, _ := NewLocalFileStorage(t.TempDir())
	SetStorageProvider(s)
	assert.Equal(t, s, GetStorageProvider())
	SetStorageProvider(nil)
	assert.Nil(t, GetStorageProvider())
}
