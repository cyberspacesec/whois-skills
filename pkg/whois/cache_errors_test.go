package whois

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- RedisCache 错误分支：关闭 miniredis 后操作 ----

func TestRedisCache_Get_ClientError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	// 关闭 miniredis，使后续 Get 失败（非 redis.Nil）
	cleanup()
	_, ok := rc.Get("any.com")
	assert.False(t, ok)
}

func TestRedisCache_Set_WriteError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	cleanup()
	// 关闭后 Set 写入失败
	rc.Set("x.com", &CacheEntry{ExpiresAt: time.Now().Add(time.Hour)})
	// 无 panic
}

func TestRedisCache_Delete_DeleteError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	cleanup()
	rc.Delete("x.com")
	// 无 panic
}

func TestRedisCache_Clear_FlushError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	cleanup()
	rc.Clear()
	// 无 panic
}

func TestRedisCache_GetStats_InfoError(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	cleanup()
	stats := rc.GetStats()
	// Info 失败时不含 redis_info，但应有 type
	assert.Equal(t, "redis", stats["type"])
}

// ---- warmup 查询失败分支 ----

func TestWarmup_QueryError(t *testing.T) {
	defer withStubQueryProvider(&stubWhoisQueryProvider{queryErr: assertError("boom")})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	dir := t.TempDir()
	path := filepath.Join(dir, "domains.json")
	os.WriteFile(path, []byte(`["err.com"]`), 0644)

	c, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60,
		WarmupConfig: &WarmupConfig{Enabled: true, DomainsFile: path, Concurrency: 0, Interval: 1}})
	c.warmup()
	// 查询失败，不缓存
	_, ok := c.Get("err.com")
	assert.False(t, ok)
}

// helper: 把 string 转为 error（避免重复 import errors）
type strErr string

func (e strErr) Error() string { return string(e) }

func assertError(s string) error { return strErr(s) }

// ---- WhoisCache GetStats enabled (local provider) ----

func TestWhoisCache_GetStats_Enabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60})
	c.Set("a.com", &whoisparser.WhoisInfo{}, "raw")
	stats := c.GetStats()
	assert.NotEqual(t, false, stats["enabled"])
	// local provider 返回 type=local
	_, hasType := stats["type"]
	assert.True(t, hasType)
}

// ---- WhoisCache Get enabled/disabled ----

func TestWhoisCache_Get_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	_, ok := c.Get("x.com")
	assert.False(t, ok)
}

func TestWhoisCache_Set_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	assert.NotPanics(t, func() { c.Set("x.com", &whoisparser.WhoisInfo{}, "raw") })
}

// ---- LocalCache Delete 不存在的 key (Entries 已为 0，仍正常) ----

func TestLocalCache_DeleteNonExistent(t *testing.T) {
	lc := newLocalCache()
	lc.Delete("nope")
	stats := lc.GetStats()
	assert.Equal(t, int64(0), stats["entries"])
}

// 确保 context import 被使用
var _ = context.Background
