package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- LocalFileStorage.Save tmp 写入失败 ----

func TestLocalFileStorage_SaveTmpWriteFail(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	// 让 keyPath 落到一个只读目录下
	// 先创建 dir/sub 只读
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.Chmod(sub, 0444) // 只读
	defer os.Chmod(sub, 0755)
	// keyPath("sub:key") = dir/sub/key.json → MkdirAll(dir/sub) OK，WriteFile 失败
	err := s.Save(context.Background(), "sub:key", map[string]string{"a": "b"})
	assert.Error(t, err)
}

// ---- LocalFileStorage.Delete 路径为目录 ----

func TestLocalFileStorage_DeletePathIsDir(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocalFileStorage(dir)
	// keyPath("d:key") = dir/d/key.json；创建为非空目录 → os.Remove 失败（非 IsNotExist）
	path := s.keyPath("d:key")
	os.MkdirAll(path, 0755)
	os.MkdirAll(filepath.Join(path, "child"), 0755) // 非空
	err := s.Delete(context.Background(), "d:key")
	assert.Error(t, err)
}

// ---- LocalFileStorage.List Rel 失败 ----

func TestLocalFileStorage_List_RelError(t *testing.T) {
	// filepath.Rel 失败极难构造（需 path 不在 directory 下）。
	// Walk 始终用 directory 作为基，Rel 一般成功。此分支实际不可达。
	// 仅验证正常 List 不 panic（已在 storage_extra 覆盖）。
	s, _ := NewLocalFileStorage(t.TempDir())
	s.Save(context.Background(), "whois:a.com", map[string]string{"a": "b"})
	_, err := s.List(context.Background(), "whois:")
	assert.NoError(t, err)
}

// ---- RedisStorage.Load miniredis 关闭后（非 redis.Nil 错误）----

func TestRedisStorage_Load_ClientError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	cleanup()
	var out map[string]string
	err = s.Load(context.Background(), "any", &out)
	assert.Error(t, err)
}

// ---- RedisStorage.List miniredis 关闭后 ----

func TestRedisStorage_List_ClientError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	cleanup()
	_, err = s.List(context.Background(), "whois:")
	// 关闭后 Scan 迭代错误
	assert.Error(t, err)
}

// ---- RedisStorage.Exists miniredis 关闭后 ----

func TestRedisStorage_Exists_ClientError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	cleanup()
	_, err = s.Exists(context.Background(), "any")
	assert.Error(t, err)
}

// ---- RedisStorage.Save miniredis 关闭后 ----

func TestRedisStorage_Save_ClientError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	s, err := NewRedisStorage(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	cleanup()
	err = s.Save(context.Background(), "k:1", map[string]string{"a": "b"})
	assert.Error(t, err)
}
