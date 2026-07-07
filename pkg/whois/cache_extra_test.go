package whois

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"context"
	"time"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ---- RedisCache CRUD via miniredis ----

func newTestRedisCache(t *testing.T) *RedisCache {
	t.Helper()
	addr, cleanup := newMiniredis(t)
	t.Cleanup(cleanup)
	rc, err := newRedisCache(&RedisConfig{Addr: addr})
	if err != nil {
		t.Fatalf("newRedisCache: %v", err)
	}
	return rc
}

func TestRedisCache_SetGet(t *testing.T) {
	rc := newTestRedisCache(t)
	entry := &CacheEntry{
		Info:        &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "example.com"}},
		RawResponse: "raw",
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	rc.Set("example.com", entry)
	got, ok := rc.Get("example.com")
	assert.True(t, ok)
	assert.NotNil(t, got)
	assert.Equal(t, "raw", got.RawResponse)
}

func TestRedisCache_Get_Miss(t *testing.T) {
	rc := newTestRedisCache(t)
	got, ok := rc.Get("nonexistent.com")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRedisCache_Get_Expired(t *testing.T) {
	rc := newTestRedisCache(t)
	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // 已过期
	}
	rc.Set("expired.com", entry)
	// Set 时 ttl 为负，miniredis 会立即过期或不存
	got, ok := rc.Get("expired.com")
	// 已过期 → miss
	_ = got
	assert.False(t, ok)
}

func TestRedisCache_Get_ManualExpiredEntry(t *testing.T) {
	// 直接往 redis 写入一个已过期的 CacheEntry JSON，绕过 Set 的 ttl 计算
	rc := newTestRedisCache(t)
	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(-time.Hour), // 已过期
	}
	data, _ := json.Marshal(entry)
	// 使用底层 client 写入一个长 ttl 的键，但 entry.ExpiresAt 已过期
	rc.client.Set(context.Background(), "whois:manual.com", string(data), time.Hour)
	got, ok := rc.Get("manual.com")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRedisCache_Get_CorruptData(t *testing.T) {
	rc := newTestRedisCache(t)
	// 写入非法 JSON
	rc.client.Set(context.Background(), "whois:bad.com", "not-json", time.Hour)
	got, ok := rc.Get("bad.com")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestRedisCache_Delete(t *testing.T) {
	rc := newTestRedisCache(t)
	entry := &CacheEntry{ExpiresAt: time.Now().Add(time.Hour)}
	rc.Set("del.com", entry)
	rc.Delete("del.com")
	_, ok := rc.Get("del.com")
	assert.False(t, ok)
}

func TestRedisCache_Clear(t *testing.T) {
	rc := newTestRedisCache(t)
	rc.Set("a.com", &CacheEntry{ExpiresAt: time.Now().Add(time.Hour)})
	rc.Set("b.com", &CacheEntry{ExpiresAt: time.Now().Add(time.Hour)})
	rc.Clear()
	_, ok := rc.Get("a.com")
	assert.False(t, ok)
}

func TestRedisCache_GetStats(t *testing.T) {
	rc := newTestRedisCache(t)
	rc.Set("a.com", &CacheEntry{ExpiresAt: time.Now().Add(time.Hour)})
	_, _ = rc.Get("a.com")  // hit
	_, _ = rc.Get("miss.com") // miss
	stats := rc.GetStats()
	assert.Equal(t, "redis", stats["type"])
	assert.NotZero(t, stats["hits"])
}

func TestRedisCache_GetStats_ZeroRequests(t *testing.T) {
	rc := newTestRedisCache(t)
	stats := rc.GetStats()
	assert.Equal(t, "redis", stats["type"])
	assert.Equal(t, float64(0), stats["hit_rate"])
}

// ---- newRedisCache nil 与 ping 失败 ----

func TestNewRedisCache_NilConfig(t *testing.T) {
	_, err := newRedisCache(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis配置不能为空")
}

func TestNewRedisCache_PingFail(t *testing.T) {
	// 连接一个不存在的地址 → Ping 失败
	_, err := newRedisCache(&RedisConfig{Addr: "127.0.0.1:1", PoolSize: 1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法连接到Redis")
}

// ---- NewWhoisCache redis 成功路径 ----

func TestNewWhoisCache_Redis(t *testing.T) {
	addr, cleanup := newMiniredis(t)
	defer cleanup()
	c, err := NewWhoisCache(CacheConfig{
		Enabled: true,
		Type:    "redis",
		RedisConfig: &RedisConfig{Addr: addr},
		TTL:     60,
	})
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

// ---- GetCache（含创建失败回退） ----

func TestGetCache_Default(t *testing.T) {
	c := GetCache()
	assert.NotNil(t, c)
}

// ---- warmup / readWarmupDomains ----

func TestReadWarmupDomains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domains.json")
	err := os.WriteFile(path, []byte(`["a.com","b.com"]`), 0644)
	assert.NoError(t, err)
	domains, err := readWarmupDomains(path)
	assert.NoError(t, err)
	assert.Equal(t, []string{"a.com", "b.com"}, domains)
}

func TestReadWarmupDomains_NotFound(t *testing.T) {
	_, err := readWarmupDomains("/nonexistent/path/to/file.json")
	assert.Error(t, err)
}

func TestReadWarmupDomains_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	err := os.WriteFile(path, []byte(`not json`), 0644)
	assert.NoError(t, err)
	_, err = readWarmupDomains(path)
	assert.Error(t, err)
}

func TestWarmup_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60,
		WarmupConfig: &WarmupConfig{Enabled: false}})
	// Enabled=false 直接 return
	c.warmup()
	// 无 panic
}

func TestWarmup_ReadDomainsFail(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60,
		WarmupConfig: &WarmupConfig{Enabled: true, DomainsFile: "/no/such/file.json"}})
	// 读取失败 → 直接 return，不 panic
	c.warmup()
}

func TestWarmup_SuccessWithStub(t *testing.T) {
	// 预热走 ExecuteQuery → 全局 provider；用 stub 注入
	defer withStubQueryProvider(&stubWhoisQueryProvider{
		raw:  "raw",
		info: whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x"}},
	})()
	defer registerLocalWhoisServer("com", "whois.verisign-grs.com")()

	dir := t.TempDir()
	path := filepath.Join(dir, "domains.json")
	os.WriteFile(path, []byte(`["a.com","b.com"]`), 0644)

	c, _ := NewWhoisCache(CacheConfig{Enabled: true, Type: "local", TTL: 60,
		WarmupConfig: &WarmupConfig{Enabled: true, DomainsFile: path, Concurrency: 2, Interval: 1}})
	c.warmup()
	// 预热后缓存应包含两个域名
	_, ok := c.Get("a.com")
	assert.True(t, ok)
}

// ---- WhoisCache disabled 分支 ----

func TestWhoisCache_Delete_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	assert.NotPanics(t, func() { c.Delete("x.com") })
}

func TestWhoisCache_Clear_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	assert.NotPanics(t, func() { c.Clear() })
}

func TestWhoisCache_ClearExpired_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	assert.NotPanics(t, func() { c.ClearExpired() })
}

func TestWhoisCache_GetStats_Disabled(t *testing.T) {
	c, _ := NewWhoisCache(CacheConfig{Enabled: false, Type: "local"})
	stats := c.GetStats()
	assert.Equal(t, false, stats["enabled"])
}

func TestWhoisCache_ClearExpired_RedisProvider(t *testing.T) {
	// Redis provider 分支：ClearExpired 不做任何事（Redis 自动过期）
	addr, cleanup := newMiniredis(t)
	defer cleanup()
	c, err := NewWhoisCache(CacheConfig{Enabled: true, Type: "redis",
		RedisConfig: &RedisConfig{Addr: addr}, TTL: 60})
	assert.NoError(t, err)
	assert.NotPanics(t, func() { c.ClearExpired() })
}
