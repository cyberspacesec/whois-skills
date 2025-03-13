package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	// WHOIS信息
	Info *whoisparser.WhoisInfo `json:"info"`

	// 原始WHOIS响应
	RawResponse string `json:"raw_response"`

	// 缓存时间
	CachedAt time.Time `json:"cached_at"`

	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
}

// CacheProvider 缓存提供者接口
type CacheProvider interface {
	Get(domain string) (*CacheEntry, bool)
	Set(domain string, entry *CacheEntry)
	Delete(domain string)
	Clear()
	GetStats() map[string]interface{}
}

// LocalCache 本地内存缓存
type LocalCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
	stats CacheStats
}

// RedisCache Redis缓存
type RedisCache struct {
	client *redis.Client
	stats  CacheStats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	mu sync.RWMutex

	// 缓存命中次数
	Hits int64

	// 缓存未命中次数
	Misses int64

	// 缓存条目数
	Entries int64

	// 过期条目数
	Expired int64

	// 总请求次数
	Requests int64
}

// WhoisCache WHOIS查询结果缓存
type WhoisCache struct {
	provider CacheProvider
	config   CacheConfig
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// 是否启用缓存
	Enabled bool

	// 缓存类型 (local/redis)
	Type string

	// Redis配置
	RedisConfig *RedisConfig

	// 缓存有效期（秒）
	TTL int64

	// 最大缓存条目数
	MaxEntries int

	// 清理间隔（秒）
	CleanupInterval int64

	// 预热配置
	WarmupConfig *WarmupConfig
}

// RedisConfig Redis配置
type RedisConfig struct {
	// Redis地址
	Addr string

	// 密码
	Password string

	// 数据库
	DB int

	// 连接池大小
	PoolSize int
}

// WarmupConfig 预热配置
type WarmupConfig struct {
	// 是否启用预热
	Enabled bool

	// 预热域名列表文件
	DomainsFile string

	// 预热并发数
	Concurrency int

	// 预热间隔（毫秒）
	Interval int64
}

// DefaultCacheConfig 默认缓存配置
var DefaultCacheConfig = CacheConfig{
	Enabled:         true,
	TTL:             3600,  // 1小时
	MaxEntries:      10000, // 最多缓存1万条记录
	CleanupInterval: 300,   // 5分钟清理一次
}

var (
	defaultCache *WhoisCache
	cacheOnce    sync.Once
)

// GetCache 获取缓存实例
func GetCache() *WhoisCache {
	cacheOnce.Do(func() {
		cache, err := NewWhoisCache(DefaultCacheConfig)
		if err != nil {
			logrus.Errorf("创建缓存失败: %v", err)
			// 如果创建失败，使用本地缓存
			cache, _ = NewWhoisCache(CacheConfig{
				Enabled: true,
				Type:    "local",
				TTL:     3600,
			})
		}
		defaultCache = cache
	})
	return defaultCache
}

// NewWhoisCache 创建新的WHOIS缓存
func NewWhoisCache(config CacheConfig) (*WhoisCache, error) {
	var provider CacheProvider
	var err error

	switch config.Type {
	case "redis":
		provider, err = newRedisCache(config.RedisConfig)
	default:
		provider = newLocalCache()
	}

	if err != nil {
		return nil, err
	}

	cache := &WhoisCache{
		provider: provider,
		config:   config,
	}

	// 如果启用了缓存预热，开始预热
	if config.WarmupConfig != nil && config.WarmupConfig.Enabled {
		go cache.warmup()
	}

	return cache, nil
}

// newLocalCache 创建本地缓存
func newLocalCache() *LocalCache {
	return &LocalCache{
		cache: make(map[string]*CacheEntry),
	}
}

// newRedisCache 创建Redis缓存
func newRedisCache(config *RedisConfig) (*RedisCache, error) {
	if config == nil {
		return nil, fmt.Errorf("Redis配置不能为空")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
		PoolSize: config.PoolSize,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("无法连接到Redis: %v", err)
	}

	return &RedisCache{
		client: client,
	}, nil
}

// Get 从缓存获取WHOIS信息
func (c *LocalCache) Get(domain string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.stats.mu.Lock()
	c.stats.Requests++
	c.stats.mu.Unlock()

	entry, exists := c.cache[domain]
	if !exists {
		c.stats.mu.Lock()
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.stats.mu.Lock()
		c.stats.Expired++
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	c.stats.mu.Lock()
	c.stats.Hits++
	c.stats.mu.Unlock()

	return entry, true
}

// Set 将WHOIS信息存入缓存
func (c *LocalCache) Set(domain string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[domain] = entry

	c.stats.mu.Lock()
	c.stats.Entries = int64(len(c.cache))
	c.stats.mu.Unlock()
}

// Delete 从缓存删除指定域名的信息
func (c *LocalCache) Delete(domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, domain)

	c.stats.mu.Lock()
	c.stats.Entries = int64(len(c.cache))
	c.stats.mu.Unlock()
}

// Clear 清空缓存
func (c *LocalCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry)

	c.stats.mu.Lock()
	c.stats.Entries = 0
	c.stats.mu.Unlock()
}

// GetStats 获取缓存统计信息
func (c *LocalCache) GetStats() map[string]interface{} {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	return map[string]interface{}{
		"type":     "local",
		"entries":  c.stats.Entries,
		"hits":     c.stats.Hits,
		"misses":   c.stats.Misses,
		"expired":  c.stats.Expired,
		"requests": c.stats.Requests,
		"hit_rate": float64(c.stats.Hits) / float64(c.stats.Requests) * 100,
	}
}

// Get 从Redis获取WHOIS信息
func (c *RedisCache) Get(domain string) (*CacheEntry, bool) {
	ctx := context.Background()

	c.stats.mu.Lock()
	c.stats.Requests++
	c.stats.mu.Unlock()

	data, err := c.client.Get(ctx, "whois:"+domain).Bytes()
	if err != nil {
		if err != redis.Nil {
			logrus.Errorf("从Redis获取缓存失败: %v", err)
		}
		c.stats.mu.Lock()
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		logrus.Errorf("解析缓存数据失败: %v", err)
		c.stats.mu.Lock()
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		c.stats.mu.Lock()
		c.stats.Expired++
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	c.stats.mu.Lock()
	c.stats.Hits++
	c.stats.mu.Unlock()

	return &entry, true
}

// Set 将WHOIS信息存入Redis
func (c *RedisCache) Set(domain string, entry *CacheEntry) {
	ctx := context.Background()

	data, err := json.Marshal(entry)
	if err != nil {
		logrus.Errorf("序列化缓存数据失败: %v", err)
		return
	}

	ttl := entry.ExpiresAt.Sub(time.Now())
	if err := c.client.Set(ctx, "whois:"+domain, data, ttl).Err(); err != nil {
		logrus.Errorf("写入Redis缓存失败: %v", err)
		return
	}

	c.stats.mu.Lock()
	c.stats.Entries++
	c.stats.mu.Unlock()
}

// Delete 从Redis删除指定域名的信息
func (c *RedisCache) Delete(domain string) {
	ctx := context.Background()

	if err := c.client.Del(ctx, "whois:"+domain).Err(); err != nil {
		logrus.Errorf("从Redis删除缓存失败: %v", err)
		return
	}

	c.stats.mu.Lock()
	c.stats.Entries--
	c.stats.mu.Unlock()
}

// Clear 清空Redis缓存
func (c *RedisCache) Clear() {
	ctx := context.Background()

	if err := c.client.FlushDB(ctx).Err(); err != nil {
		logrus.Errorf("清空Redis缓存失败: %v", err)
		return
	}

	c.stats.mu.Lock()
	c.stats.Entries = 0
	c.stats.mu.Unlock()
}

// GetStats 获取Redis缓存统计信息
func (c *RedisCache) GetStats() map[string]interface{} {
	ctx := context.Background()

	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()

	stats := map[string]interface{}{
		"type":     "redis",
		"hits":     c.stats.Hits,
		"misses":   c.stats.Misses,
		"expired":  c.stats.Expired,
		"requests": c.stats.Requests,
		"hit_rate": float64(c.stats.Hits) / float64(c.stats.Requests) * 100,
	}

	// 获取Redis信息
	if info, err := c.client.Info(ctx).Result(); err == nil {
		stats["redis_info"] = info
	}

	return stats
}

// warmup 执行缓存预热
func (c *WhoisCache) warmup() {
	if !c.config.WarmupConfig.Enabled {
		return
	}

	// 读取预热域名列表
	domains, err := readWarmupDomains(c.config.WarmupConfig.DomainsFile)
	if err != nil {
		logrus.Errorf("读取预热域名列表失败: %v", err)
		return
	}

	logrus.Infof("开始缓存预热，共 %d 个域名", len(domains))

	// 创建工作池
	concurrency := c.config.WarmupConfig.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	// 创建工作通道
	jobs := make(chan string, len(domains))
	for _, domain := range domains {
		jobs <- domain
	}
	close(jobs)

	// 启动工作协程
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range jobs {
				// 查询WHOIS信息
				info, err := ExecuteQuery(&QueryOptions{
					Domain: domain,
				})
				if err != nil {
					logrus.Errorf("预热查询失败 [%s]: %v", domain, err)
					continue
				}

				// 缓存结果
				entry := &CacheEntry{
					Info:      info,
					CachedAt:  time.Now(),
					ExpiresAt: time.Now().Add(time.Duration(c.config.TTL) * time.Second),
				}
				c.provider.Set(domain, entry)

				// 等待指定间隔
				if c.config.WarmupConfig.Interval > 0 {
					time.Sleep(time.Duration(c.config.WarmupConfig.Interval) * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	logrus.Info("缓存预热完成")
}

// readWarmupDomains 读取预热域名列表
func readWarmupDomains(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, err
	}

	return domains, nil
}

// Get 从缓存获取WHOIS信息
func (c *WhoisCache) Get(domain string) (*CacheEntry, bool) {
	if !c.config.Enabled {
		return nil, false
	}
	return c.provider.Get(domain)
}

// Set 将WHOIS信息存入缓存
func (c *WhoisCache) Set(domain string, info *whoisparser.WhoisInfo, rawResponse string) {
	if !c.config.Enabled {
		return
	}

	entry := &CacheEntry{
		Info:        info,
		RawResponse: rawResponse,
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(c.config.TTL) * time.Second),
	}

	c.provider.Set(domain, entry)
}

// Delete 从缓存删除指定域名的信息
func (c *WhoisCache) Delete(domain string) {
	if !c.config.Enabled {
		return
	}
	c.provider.Delete(domain)
}

// Clear 清空缓存
func (c *WhoisCache) Clear() {
	if !c.config.Enabled {
		return
	}
	c.provider.Clear()
}

// GetStats 获取缓存统计信息
func (c *WhoisCache) GetStats() map[string]interface{} {
	if !c.config.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	return c.provider.GetStats()
}
