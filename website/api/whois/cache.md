# 💾 cache.go — WHOIS 结果缓存

> 📖 WHOIS 查询结果缓存层，提供本地内存与 Redis 双实现、缓存预热、命中率统计，显著降低重复查询延迟与对外部服务器的压力。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/cache.go` |
| 核心职责 | 缓存 WHOIS 结果、预热、统计 |
| 实现 | `LocalCache`（内存）+ `RedisCache`（远程） |
| 全局单例 | `GetCache()` |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

// 使用默认配置（本地内存）
cache, err := whois.NewWhoisCache(whois.DefaultCacheConfig)
if err != nil {
    log.Fatal(err)
}

// 写入
cache.Set("example.com", info, rawResponse)

// 读取
if entry, ok := cache.Get("example.com"); ok {
    fmt.Println("命中缓存：", entry.Info.Domain)
}

// 统计
stats := cache.GetStats()
fmt.Printf("命中率 %.1f%%\n",
    float64(stats["hits"].(int64))/float64(stats["requests"].(int64))*100)
```

---

## 📊 核心类型

### CacheEntry

```go
type CacheEntry struct {
    Info        *whoisparser.WhoisInfo // 解析结果
    RawResponse string                 // 原始响应
    CachedAt    time.Time              // 写入时刻
    ExpiresAt   time.Time              // 过期时刻
}
```

### CacheProvider 接口

```go
type CacheProvider interface {
    Get(domain string) (*CacheEntry, bool)
    Set(domain string, entry *CacheEntry)
    Delete(domain string)
    Clear()
    GetStats() map[string]interface{}
}
```

实现：`LocalCache`、`RedisCache`。

### CacheStats

```go
type CacheStats struct {
    Hits     int64 // 命中
    Misses   int64 // 未命中
    Entries  int64 // 条目数
    Expired  int64 // 已过期
    Requests int64 // 总请求
    // 内含 sync.Mutex
}
```

### CacheConfig

```go
type CacheConfig struct {
    Enabled         bool          // 是否启用
    Type            string        // "local" 或 "redis"
    RedisConfig     *RedisConfig  // Redis 配置
    TTL             int           // 生存时间（秒）
    MaxEntries      int           // 最大条目数
    CleanupInterval int           // 清理间隔（秒）
    WarmupConfig    *WarmupConfig // 预热配置
}

type RedisConfig struct {
    Addr     string
    Password string
    DB       int
    PoolSize int
}

type WarmupConfig struct {
    Enabled     bool
    DomainsFile string // 域名列表 JSON
    Concurrency int
    Interval    int    // 预热间隔（毫秒）
}
```

### 默认配置

```go
var DefaultCacheConfig = CacheConfig{
    Enabled:         true,
    TTL:             3600,
    MaxEntries:      10000,
    CleanupInterval: 300,
}
```

---

## 🔧 函数与方法

### 创建与全局访问

| 函数/方法 | 说明 |
|-----------|------|
| `GetCache() *WhoisCache` | 全局单例（`sync.Once`），失败回退本地缓存 |
| `NewWhoisCache(config) (*WhoisCache, error)` | 按配置创建，启用预热则启动 `cache.warmup()` |

### WhoisCache 方法

| 方法 | 说明 |
|------|------|
| `Get(domain) (*CacheEntry, bool)` | 读取（检查过期） |
| `Set(domain, info, raw)` | 写入 |
| `Delete(domain)` | 删除 |
| `Clear()` | 清空 |
| `ClearExpired()` | 清理过期条目 |
| `GetStats() map[string]interface{}` | 统计 |

---

## 🔍 关键实现要点

::: details LocalCache 实现
- 底层 `map[string]*CacheEntry` + `sync.RWMutex`
- `Get` 检查 `ExpiresAt`，过期则删除并返回未命中
- 读写锁保证并发安全，读多写少场景性能良好
:::

::: details RedisCache 实现
- key 格式：`whois:{domain}`
- `Set` 将 `CacheEntry` JSON 序列化写入，TTL = `ExpiresAt - now`
- `Get` 反序列化读取
- `Clear` 调用 `FlushDB`（注意：会清空整个 DB）
:::

::: details 缓存预热 warmup
启用 `WarmupConfig.Enabled` 时：

1. 从 `DomainsFile` 读取 JSON 域名列表
2. 创建工作池（默认并发 5）
3. 每个 worker 调用 `ExecuteQuery` 查询并写入缓存
4. 按 `Interval` 限速，避免冲击 WHOIS 服务器
:::

::: details 命中率计算
```
命中率 = Hits / Requests * 100%
```
其中 `Requests = Hits + Misses`。`CacheStats` 自带 `sync.Mutex` 保证原子计数。
:::

---

## 📝 使用示例

### 示例 1：本地内存缓存

```go
cache, _ := whois.NewWhoisCache(whois.DefaultCacheConfig)
cache.Set("example.com", info, raw)

if entry, ok := cache.Get("example.com"); ok {
    fmt.Println("缓存命中，注册商：", entry.Info.Registrar.Name)
}
```

### 示例 2：Redis 缓存

```go
config := whois.CacheConfig{
    Enabled: true,
    Type:    "redis",
    TTL:     7200,
    RedisConfig: &whois.RedisConfig{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
        PoolSize: 10,
    },
}
cache, err := whois.NewWhoisCache(config)
```

### 示例 3：缓存预热

```go
config := whois.DefaultCacheConfig
config.WarmupConfig = &whois.WarmupConfig{
    Enabled:     true,
    DomainsFile: "top_domains.json",
    Concurrency: 5,
    Interval:    200,
}
cache, _ := whois.NewWhoisCache(config)
// 构造时自动启动预热 goroutine
```

### 示例 4：使用全局单例

```go
// 在查询路径中通过 GetCache() 复用
cache := whois.GetCache()
if entry, ok := cache.Get(domain); ok {
    return entry.Info, nil // 命中缓存
}
// 未命中则查询并写入
```

---

## ⚠️ 注意事项

- `RedisCache.Clear()` 会清空整个 Redis DB，生产环境慎用。
- TTL 设置过短会降低命中率，过长会导致过期数据；建议 3600s 起步。
- `MaxEntries` 仅对 `LocalCache` 生效，Redis 不受此限制。
- 预热会向 WHOIS 服务器发起大量查询，务必配合限速。

---

## 🔗 相关

- ⚙️ [config.md](./config.md) — 全局配置
- 🔒 [proxy.md](./proxy.md) — 代理查询路径
- 🎯 [缓存配置教程](../../guide/tutorial-cache.md)
