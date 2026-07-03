package whois

import (
	"fmt"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/stretchr/testify/assert"
)

func TestLocalCache_GetSet(t *testing.T) {
	cache := newLocalCache()

	// 测试设置和获取
	info := &whoisparser.WhoisInfo{}
	entry := &CacheEntry{
		Info:        info,
		RawResponse: "test response",
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	cache.Set("example.com", entry)

	// 获取存在的条目
	got, found := cache.Get("example.com")
	assert.True(t, found)
	assert.NotNil(t, got)
	assert.Equal(t, "test response", got.RawResponse)

	// 获取不存在的条目
	_, found = cache.Get("nonexistent.com")
	assert.False(t, found)
}

func TestLocalCache_Expired(t *testing.T) {
	cache := newLocalCache()

	// 设置一个已过期的条目
	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour), // 已过期
	}

	cache.Set("expired.com", entry)

	// 过期条目应该返回未命中
	_, found := cache.Get("expired.com")
	assert.False(t, found)
}

func TestLocalCache_Delete(t *testing.T) {
	cache := newLocalCache()

	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	cache.Set("delete-me.com", entry)
	_, found := cache.Get("delete-me.com")
	assert.True(t, found)

	cache.Delete("delete-me.com")
	_, found = cache.Get("delete-me.com")
	assert.False(t, found)
}

func TestLocalCache_Clear(t *testing.T) {
	cache := newLocalCache()

	for i := 0; i < 5; i++ {
		entry := &CacheEntry{
			Info:      &whoisparser.WhoisInfo{},
			CachedAt:  time.Now(),
			ExpiresAt: time.Now().Add(time.Hour),
		}
		cache.Set(fmt.Sprintf("domain%d.com", i), entry)
	}

	stats := cache.GetStats()
	assert.Equal(t, int64(5), stats["entries"])

	cache.Clear()
	stats = cache.GetStats()
	assert.Equal(t, int64(0), stats["entries"])
}

func TestLocalCache_ClearExpired(t *testing.T) {
	cache := newLocalCache()

	// 添加一个有效条目
	validEntry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	cache.Set("valid.com", validEntry)

	// 添加一个过期条目
	expiredEntry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	cache.Set("expired.com", expiredEntry)

	// 清理过期条目
	cache.clearExpired()

	// 有效条目应该仍在
	_, found := cache.Get("valid.com")
	assert.True(t, found)

	// 过期条目应该已清理（Get 已经不会返回过期数据，但 clearExpired 是从 map 中删除）
	stats := cache.GetStats()
	assert.Equal(t, int64(1), stats["entries"])
}

func TestLocalCache_GetStats_NoDivisionByZero(t *testing.T) {
	cache := newLocalCache()

	// 没有任何请求时，GetStats 不应除零
	stats := cache.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, float64(0), stats["hit_rate"])
	assert.Equal(t, int64(0), stats["requests"])
}

func TestLocalCache_GetStats_HitRate(t *testing.T) {
	cache := newLocalCache()

	entry := &CacheEntry{
		Info:      &whoisparser.WhoisInfo{},
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	cache.Set("hit.com", entry)

	// 产生一次命中
	cache.Get("hit.com")
	// 产生一次未命中
	cache.Get("miss.com")

	stats := cache.GetStats()
	assert.Equal(t, int64(2), stats["requests"])
	assert.Equal(t, int64(1), stats["hits"])
	assert.Equal(t, int64(1), stats["misses"])
	hitRate, ok := stats["hit_rate"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 50.0, hitRate)
}

func TestWhoisCache_Disabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
		Type:    "local",
		TTL:     3600,
	}
	cache, err := NewWhoisCache(config)
	assert.NoError(t, err)

	// 禁用缓存时，Set 不应存储
	cache.Set("test.com", &whoisparser.WhoisInfo{}, "response")

	// 禁用缓存时，Get 应返回 false
	_, found := cache.Get("test.com")
	assert.False(t, found)

	// 禁用缓存时，GetStats 应返回 enabled=false
	stats := cache.GetStats()
	assert.Equal(t, false, stats["enabled"])
}

func TestWhoisCache_Enabled(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		Type:    "local",
		TTL:     3600,
	}
	cache, err := NewWhoisCache(config)
	assert.NoError(t, err)

	info := &whoisparser.WhoisInfo{}
	cache.Set("example.com", info, "raw whois data")

	entry, found := cache.Get("example.com")
	assert.True(t, found)
	assert.NotNil(t, entry)
	assert.Equal(t, "raw whois data", entry.RawResponse)
}

func TestWhoisCache_Delete(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		Type:    "local",
		TTL:     3600,
	}
	cache, err := NewWhoisCache(config)
	assert.NoError(t, err)

	cache.Set("delete.com", &whoisparser.WhoisInfo{}, "data")
	_, found := cache.Get("delete.com")
	assert.True(t, found)

	cache.Delete("delete.com")
	_, found = cache.Get("delete.com")
	assert.False(t, found)
}

func TestWhoisCache_Clear(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		Type:    "local",
		TTL:     3600,
	}
	cache, err := NewWhoisCache(config)
	assert.NoError(t, err)

	cache.Set("a.com", &whoisparser.WhoisInfo{}, "a")
	cache.Set("b.com", &whoisparser.WhoisInfo{}, "b")

	cache.Clear()

	_, found := cache.Get("a.com")
	assert.False(t, found)
	_, found = cache.Get("b.com")
	assert.False(t, found)
}

func TestWhoisCache_ClearExpired(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		Type:    "local",
		TTL:     3600,
	}
	cache, err := NewWhoisCache(config)
	assert.NoError(t, err)

	cache.Set("test.com", &whoisparser.WhoisInfo{}, "data")
	cache.ClearExpired() // 不应 panic
}

func TestNewWhoisCache_RedisNilConfig(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		Type:    "redis",
	}
	_, err := NewWhoisCache(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Redis配置不能为空")
}
