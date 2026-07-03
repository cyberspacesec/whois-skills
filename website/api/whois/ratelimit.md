# 🚦 ratelimit.go — 查询速率限制

> 📖 基于令牌桶的 WHOIS 查询速率限制器，支持全局与每服务器双维度限速，防止触发注册局反爬与 IP 封锁。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/ratelimit.go` |
| 核心职责 | 全局 + 每服务器双维度限速 |
| 算法 | 令牌桶（token bucket） |
| 阻塞模式 | `Wait()` |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

rl := whois.NewRateLimiter(whois.RateLimiterConfig{
    GlobalRate: 10, // 全局 10 req/s
    BurstSize:  20,
    PerServerRate: map[string]float64{
        "whois.verisign-grs.com": 5, // 该服务器 5 req/s
    },
})

// 非阻塞检查
if rl.Allow("whois.verisign-grs.com") {
    // 执行查询
}

// 阻塞等待
rl.Wait("whois.verisign-grs.com") // 阻塞到允许为止
query()
```

---

## 📊 核心类型

### RateLimiterConfig

```go
type RateLimiterConfig struct {
    GlobalRate    float64            // 全局速率（req/s）
    PerServerRate map[string]float64 // 每服务器速率
    BurstSize     int                // 突发大小
}
```

### RateLimiter

```go
type RateLimiter struct {
    config        RateLimiterConfig
    serverBuckets map[string]*tokenBucket // 每服务器令牌桶
    globalBucket  *tokenBucket            // 全局令牌桶
}
```

---

## 🔧 函数与方法

| 函数/方法 | 说明 |
|-----------|------|
| `NewRateLimiter(config) *RateLimiter` | 创建限速器 |
| `Allow(server string) bool` | 非阻塞检查（先全局后每服务器） |
| `Wait(server string)` | 阻塞循环 100ms 直到 Allow |

---

## 🔍 关键实现要点

::: details tokenBucket 令牌桶
未导出的 `tokenBucket` 结构：

```go
type tokenBucket struct {
    tokens     float64       // 当前令牌数
    maxTokens  float64       // 最大令牌数
    rate       float64       // 补充速率（req/s）
    lastRefill time.Time     // 上次补充时间
    mu         sync.Mutex
}
```

- `newTokenBucket(rate, burst)` — 初始 `tokens = maxTokens = burst`，`burst <= 0` 时默认 `burst = rate`
- `allow()` — 按距上次 refill 的秒数累加 `tokens`（上限 `maxTokens`），`tokens >= 1` 则扣 1 返回 true
:::

::: details Allow 双维度检查
`Allow(server)` 顺序检查：

1. **全局桶** `globalBucket.allow()` — 不通过返回 false
2. **服务器桶** `serverBuckets[server].allow()` — 不通过返回 false
3. 两者都通过才返回 true

未配置 `PerServerRate` 的服务器默认允许（不创建桶）。
:::

::: details Wait 阻塞模式
`Wait` 循环调用 `Allow`，未通过则 `time.Sleep(100 * time.Millisecond)` 重试，直到通过。适合不急于返回的场景。

```go
func (r *RateLimiter) Wait(server string) {
    for !r.Allow(server) {
        time.Sleep(100 * time.Millisecond)
    }
}
```
:::

::: details nil 接收者安全
`Allow` 在接收者为 `nil` 时直接返回 `true`，允许在不配置限速器的情况下安全调用。

```go
var rl *whois.RateLimiter // nil
rl.Allow("server") // true，不限速
```
:::

---

## 📝 使用示例

### 示例 1：全局限速

```go
rl := whois.NewRateLimiter(whois.RateLimiterConfig{
    GlobalRate: 5,  // 全局 5 req/s
    BurstSize:  10,
})

for _, domain := range domains {
    if rl.Allow("any") {
        query(domain)
    } else {
        time.Sleep(200 * time.Millisecond)
    }
}
```

### 示例 2：每服务器差异化限速

```go
rl := whois.NewRateLimiter(whois.RateLimiterConfig{
    GlobalRate: 20,
    BurstSize:  30,
    PerServerRate: map[string]float64{
        "whois.verisign-grs.com": 5,  // .com 严格限速
        "whois.nic.me":           2,  // .me 更严格
        "whois.iana.org":        10,  // IANA 宽松
    },
})

rl.Wait("whois.verisign-grs.com")
query()
```

### 示例 3：集成到客户端

```go
client := whois.NewWhoisClient()
client.SetRateLimiter(whois.NewRateLimiter(whois.RateLimiterConfig{
    GlobalRate: 10,
    BurstSize:  20,
}))
```

### 示例 4：阻塞式批量

```go
rl := whois.NewRateLimiter(whois.RateLimiterConfig{
    GlobalRate: 3,
    BurstSize:  5,
})
for _, domain := range domains {
    rl.Wait("global") // 自动等待
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: domain})
    // 不会超过 3 req/s
}
```

---

## 🔗 相关

- 🧠 [scheduler.md](./scheduler.md) — 智能调度器（含自适应令牌桶）
- 🔒 [proxy.md](./proxy.md) — 代理客户端（集成限速）
- 🔎 [query.md](./query.md) — 查询引擎
