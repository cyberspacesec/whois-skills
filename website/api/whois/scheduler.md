# 🧠 scheduler.go — 智能查询调度器

> 📖 智能查询调度器，按响应时间与限速反馈自适应调整查询间隔、退避策略、并发与令牌桶速率，实现既快又稳的查询节奏。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/scheduler.go` |
| 核心职责 | 自适应间隔、指数退避、健康状态、自适应限速 |
| 反馈源 | `RecordResult`（延迟 + 错误） |
| 限速 | 内置 `AdaptiveRateLimiter` |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

s := whois.NewSmartScheduler(whois.DefaultSchedulerConfig())

for _, domain := range domains {
    server := "whois.verisign-grs.com"

    // 1. 调度：获取等待时长
    wait, err := s.Schedule(ctx, server)
    if err != nil {
        // 服务器不健康，跳过或换服务器
        continue
    }
    if wait > 0 {
        time.Sleep(wait)
    }

    // 2. 执行查询
    start := time.Now()
    info, err := query(domain)
    latency := time.Since(start)

    // 3. 反馈结果
    s.RecordResult(server, latency, err)
}

// 查看统计
stats := s.GetStats()
fmt.Printf("调度 %d 次，限速 %d 次，退避 %d 次\n",
    stats.TotalScheduled, stats.TotalRateLimited, stats.TotalBackoffs)
```

---

## 📊 核心类型

### SchedulerConfig

```go
type SchedulerConfig struct {
    DefaultInterval       int     // 默认间隔（毫秒）
    MinInterval           int     // 最小间隔
    MaxInterval           int     // 最大间隔
    AdaptFactor           float64 // 自适应因子
    MaxConcurrency        int     // 最大并发
    BackoffInitialMs      int     // 退避初始（毫秒）
    BackoffMaxMs          int     // 退避上限
    BackoffMultiplier     float64 // 退避倍数
    HealthCheckInterval   int     // 健康检查间隔（秒）
    UnhealthyThreshold    int     // 不健康阈值（连续失败次数）
    RecoveryInterval      int     // 恢复间隔（秒）
}
```

### 默认配置（DefaultSchedulerConfig）

| 字段 | 默认值 |
|------|--------|
| `DefaultInterval` | 200 ms |
| `MinInterval` | 50 ms |
| `MaxInterval` | 5000 ms |
| `AdaptFactor` | 0.3 |
| `MaxConcurrency` | 5 |
| `BackoffInitialMs` | 1000 |
| `BackoffMaxMs` | 60000 |
| `BackoffMultiplier` | 2.0 |
| `HealthCheckInterval` | 300 s |
| `UnhealthyThreshold` | 3 |
| `RecoveryInterval` | 60 s |

### ServerState

```go
type ServerState struct {
    Server              string
    AvgLatency          time.Duration
    LastLatency         time.Duration
    QueryCount          int64
    FailureCount        int64
    RateLimitedCount    int64
    ConsecutiveFailures int
    CurrentBackoff      time.Duration
    NextAllowedTime     time.Time
    Healthy             bool
    AdaptiveInterval    time.Duration
    // 内含私有 latencyHistory（保留最近 10）
}
```

### SchedulerStats

```go
type SchedulerStats struct {
    TotalScheduled    int64
    TotalRateLimited  int64
    TotalBackoffs     int64
    TotalUnhealthy    int64
    TotalAdaptations  int64
}
```

### AdaptiveRateLimiter

```go
type AdaptiveRateLimiter struct {
    currentRate           float64
    minRate               float64
    maxRate               float64
    bucket                *tokenBucket
    consecutiveSuccess    int
    consecutiveRateLimited int
    lastAdjust            time.Time
}
```

---

## 🔧 函数与方法

### 创建

| 函数 | 说明 |
|------|------|
| `DefaultSchedulerConfig() SchedulerConfig` | 默认配置 |
| `NewSmartScheduler(config) *SmartScheduler` | 创建（含 `NewAdaptiveRateLimiter(5.0, 1.0, 20.0)`） |
| `NewAdaptiveRateLimiter(initialRate, minRate, maxRate) *AdaptiveRateLimiter` | 创建自适应限速器 |

### 调度与反馈

| 方法 | 说明 |
|------|------|
| `Schedule(ctx, server) (time.Duration, error)` | 返回等待时长（0 立即执行） |
| `RecordResult(server, latency, err)` | 反馈查询结果 |
| `GetServerState(server) *ServerState` | 获取服务器状态 |
| `GetAllServerStates() map` | 所有服务器状态 |
| `GetStats() SchedulerStats` | 全局统计 |
| `MarkServerHealthy(server)` | 标记健康 |
| `MarkServerUnhealthy(server)` | 标记不健康 |

### AdaptiveRateLimiter 方法

| 方法 | 说明 |
|------|------|
| `Allow() bool` | 是否允许（每 30s 调整速率） |
| `RecordSuccess()` | 记录成功（连续 10 次提速 ×1.1） |
| `RecordRateLimited()` | 记录限速（立即降速 ×0.7） |
| `GetCurrentRate() float64` | 当前速率 |

---

## 🔍 关键实现要点

::: details Schedule 调度逻辑
1. 获取或创建 `ServerState`
2. 若不健康 → 返回错误
3. 若在退避期内 → 返回剩余等待时间
4. 若 `AdaptiveRateLimiter.Allow()` 为 false → 返回 `1000/rate ms` 等待
5. 否则返回 `AdaptiveInterval`
:::

::: details RecordResult 反馈处理
1. 更新 `latencyHistory`（保留最近 10），计算平均延迟
2. 若出错：
   - `FailureCount++`、`ConsecutiveFailures++`
   - 限速错误触发 `handleRateLimit`（指数退避 × `BackoffMultiplier`，上限 `BackoffMaxMs`）+ `adaptiveLimiter.RecordRateLimited()`
   - 连续失败 ≥ `UnhealthyThreshold` → 标记不健康
3. 若成功：
   - 重置 `ConsecutiveFailures`
   - `decreaseInterval`（× `(1 - AdaptFactor*0.5)`，下限 `MinInterval`）
:::

::: details AdaptiveRateLimiter 自适应
- `adjustRate` — 每 30s 检查：连续成功 ≥5 提速率 ×1.1，重建令牌桶
- `RecordSuccess` — 连续 10 次成功提速 ×1.1
- `RecordRateLimited` — 立即降速 ×0.7
- 速率范围 `[minRate, maxRate]`
:::

::: details isRateLimitError 判断
调用 `CheckError(err)` 后判断 `Type`：

- `ErrRateLimited` → 限速
- `ErrServerConnectFailed` → 视为限速（触发退避）

两者都会触发 `handleRateLimit` 与自适应降速。
:::

---

## 📝 使用示例

### 示例 1：自适应查询循环

```go
s := whois.NewSmartScheduler(whois.DefaultSchedulerConfig())

for _, domain := range domains {
    wait, err := s.Schedule(ctx, server)
    if err != nil {
        time.Sleep(60 * time.Second) // 不健康，等恢复
        continue
    }
    time.Sleep(wait)

    start := time.Now()
    info, err := query(domain)
    s.RecordResult(server, time.Since(start), err)
}
```

### 示例 2：多服务器调度

```go
servers := []string{"whois.verisign-grs.com", "whois.nic.me", "whois.iana.org"}
for _, domain := range domains {
    server := pickServer(domain, servers)
    wait, _ := s.Schedule(ctx, server)
    time.Sleep(wait)
    // 查询并反馈
}
// 每个服务器独立维护状态与退避
```

### 示例 3：监控服务器健康

```go
states := s.GetAllServerStates()
for server, st := range states {
    fmt.Printf("%s: 健康=%v, 平均延迟=%v, 连续失败=%d\n",
        server, st.Healthy, st.AvgLatency, st.ConsecutiveFailures)
}
```

### 示例 4：手动标记健康状态

```go
// 外部健康检查发现服务器恢复
s.MarkServerHealthy("whois.nic.me")
```

---

## 🔗 相关

- 🚦 [ratelimit.md](./ratelimit.md) — 基础令牌桶限速
- 🔒 [proxy.md](./proxy.md) — 代理客户端
- ❌ [errors.md](./errors.md) — 错误分类（识别限速错误）
