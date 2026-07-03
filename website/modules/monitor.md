# ⏱️ monitor 模块 — 性能监控

> 📖 `pkg/monitor` 提供 WHOIS 查询性能监控器，统计查询次数、成功率、延迟分布（P90/P95/P99），支持按域名统计与装饰器模式。当前为独立模块，需查询层主动调用。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `pkg/monitor` |
| 源文件数 | 1（`performance.go`，另有 1 个 `_test.go`） |
| 职责 | WHOIS 查询性能统计、百分位延迟、错误记录、装饰器 |
| 依赖 | logrus |

---

## 📁 文件清单

| 文件 | 职责 |
|------|------|
| `performance.go` | `PerformanceMonitor` 单例 — 全部性能监控能力 |

---

## 🧱 核心类型

### PerformanceMonitor

```go
type PerformanceMonitor struct {
    startTime         time.Time
    totalQueries      int64
    successfulQueries int64
    failedQueries     int64
    proxyQueries      int64
    directQueries     int64
    totalLatencyMs    int64
    recentLatencies   []int64    // 用于百分位计算
    domainStats       map[string]*DomainStat
    recentErrors      []ErrorRecord
    // ...
}
```

### PerformanceStats（输出）

| 字段 | 说明 |
|------|------|
| `UptimeSeconds` | 运行时长 |
| `TotalQueries` / `SuccessfulQueries` / `FailedQueries` | 查询计数 |
| `SuccessRate` | 成功率（%） |
| `AvgLatencyMs` | 平均延迟 |
| `P90LatencyMs` / `P95LatencyMs` / `P99LatencyMs` | 百分位延迟 |
| `QueriesPerSecond` | QPS |
| `RecentDomains` / `RecentErrors` | 最近域名与错误 |

---

## 🔧 核心函数

| 函数 | 说明 |
|------|------|
| `GetMonitor() *PerformanceMonitor` | 获取单例 |
| `RecordQuery(domain, latencyMs, successful, usedProxy)` | 记录一次查询 |
| `RecordError(domain, errMsg, usedProxy)` | 记录一次错误 |
| `GetStats() PerformanceStats` | 获取统计快照 |
| `LogStats(interval)` | 定期输出统计到日志 |
| `StartPerformanceLogging(interval)` | 启动后台日志记录 |
| `WithPerformanceMonitoring(domain, usedProxy, fn)` | 装饰器：自动计时与记录 |

---

## 📐 百分位计算

`percentile(values, p)` 对最近延迟排序后取分位点：

```go
index = int(float64(len(sorted)-1) * float64(p) / 100.0)
```

仅基于 `recentLatencies`（有上限），适合滚动窗口近似。

---

## 🚀 使用示例

### 主动记录

```go
mon := monitor.GetMonitor()
start := time.Now()
info, err := whois.ExecuteQueryWithResult(opts)
mon.RecordQuery("example.com", time.Since(start).Milliseconds(), err == nil, false)
if err != nil {
    mon.RecordError("example.com", err.Error(), false)
}
```

### 装饰器模式

```go
result, err := monitor.WithPerformanceMonitoring("example.com", false, func() (interface{}, error) {
    return whois.ExecuteQueryWithResult(opts)
})
```

### 查看统计

```go
stats := monitor.GetStats()
fmt.Printf("P95: %dms, 成功率: %.1f%%\n", stats.P95LatencyMs, stats.SuccessRate)
```

---

## ⚠️ 注意事项

::: warning ⚠️ 模块尚未接入主流程
`monitor` 目前是**独立库**，`api` 与 `cmd` 均未直接调用它。WHOIS 查询端点（`/api/whois`）记录的是 [metrics 模块](./metrics.md) 的 `RecordWHOISQuery`，而非本模块。如需启用查询级性能百分位，需在查询层主动调用 `RecordQuery`，或用 `WithPerformanceMonitoring` 包裹查询函数。
:::

- 百分位基于滚动窗口，样本量受 `maxLatencyRecords` 限制，非全量精确值。
- 单例内存状态，进程重启即丢失。

---

## 🔗 相关链接

- [metrics 模块](./metrics.md) — 系统级监控告警
- [observability.go](../api/whois/observability.md) — whois 内的可观测能力
- [模块总览](./overview.md)
