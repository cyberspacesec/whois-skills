# 📦 batch.go — 流式批量 WHOIS 查询

> 📖 流式批量 WHOIS 查询处理器，支持断点续查、并发限速、进度/结果回调、统计预估，适合大规模域名资产盘点。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/batch.go` |
| 核心职责 | 批量并发查询、断点续查、进度回调、统计预估 |
| 依赖 | `query.go`（`ExecuteQueryWithResultContextContext`） |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

config := whois.DefaultStreamBatchConfig()
config.Concurrency = 10
config.CheckpointFile = "checkpoint.json"

processor := whois.NewStreamBatchProcessor(config)

// 进度回调
processor.OnProgress(func(stats whois.StreamBatchStats) {
    fmt.Printf("进度 %d/%d\n", stats.Completed, stats.TotalTasks)
})

// 结果回调
processor.OnResult(func(r whois.StreamBatchResult) {
    fmt.Printf("%s 完成，延迟 %dms\n", r.Domain, r.Latency)
})

// 异步执行
err := processor.Process(ctx, []string{"a.com", "b.com", "c.com"})
```

---

## ⚙️ StreamBatchConfig

```go
type StreamBatchConfig struct {
    Concurrency        int           // 并发数
    Timeout            int           // 单查询超时（秒）
    MaxRetries         int           // 最大重试
    RetryInterval      int           // 重试间隔（毫秒）
    CheckpointFile     string        // 断点文件路径
    CheckpointInterval int           // 断点保存间隔（秒）
    QueryDelay         int           // 域间限速延迟（毫秒）
    UseProxy           bool          // 是否走代理
}
```

### 默认值（DefaultStreamBatchConfig）

| 字段 | 默认值 |
|------|--------|
| `Concurrency` | 5 |
| `Timeout` | 10 |
| `MaxRetries` | 3 |
| `RetryInterval` | 1000 |
| `CheckpointInterval` | 10 |
| `QueryDelay` | 200 |

---

## 📊 结果与统计类型

### StreamBatchResult

```go
type StreamBatchResult struct {
    Domain       string                    // 域名
    Info         *whoisparser.WhoisInfo    // 解析结果
    RawResponse  string                    // 原始响应
    Latency      int64                     // 延迟（毫秒）
    Error        error                     // 错误
    RetryCount   int                       // 重试次数
    FromCache    bool                      // 是否来自缓存
}
```

### StreamBatchStats

```go
type StreamBatchStats struct {
    TotalTasks         int64    // 总任务数
    Completed          int64    // 已完成
    SuccessCount       int64    // 成功数
    FailureCount       int64    // 失败数
    CacheHits          int64    // 缓存命中
    AvgLatency         int64    // 平均延迟（毫秒）
    Elapsed            int64    // 已耗时（毫秒）
    EstimatedRemaining int64    // 预计剩余（毫秒）
}
```

### Checkpoint 结构

```go
type Checkpoint struct {
    BatchID          string                        // 批次 ID
    CreatedAt        time.Time
    AllDomains       []string                      // 全部域名
    CompletedDomains map[string]bool               // 已完成域名
    Results          map[string]*CheckpointResult  // 结果快照
    TotalTasks       int64
    SuccessCount     int64
    FailureCount     int64
}

type CheckpointResult struct {
    RawResponse string
    Error       string
    Latency     int64
    RetryCount  int
    FromCache   bool
}
```

---

## 🔧 函数与方法

### 创建与配置

| 函数/方法 | 说明 |
|-----------|------|
| `DefaultStreamBatchConfig() StreamBatchConfig` | 默认配置 |
| `NewStreamBatchProcessor(config) *StreamBatchProcessor` | 创建处理器 |

### 回调与结果

| 方法 | 说明 |
|------|------|
| `OnProgress(callback func(stats))` | 进度回调 |
| `OnResult(callback func(result))` | 结果回调 |
| `Results() <-chan *StreamBatchResult` | 结果 channel |

### 执行与控制

| 方法 | 说明 |
|------|------|
| `Process(ctx, domains []string) error` | 异步执行批量查询 |
| `Cancel()` | 取消并保存断点 |
| `GetStats() StreamBatchStats` | 获取统计 |

### 断点续查

| 函数/方法 | 说明 |
|-----------|------|
| `LoadCheckpointFromFile(filePath) (*Checkpoint, error)` | 从文件加载断点 |
| `ResumeFromCheckpoint(ctx, config) (*StreamBatchProcessor, error)` | 从断点恢复 |
| `CollectResults(resultChan) []*StreamBatchResult` | 阻塞收集所有结果 |

---

## 🔍 关键实现要点

::: details Process 主流程
1. 创建带取消的 context
2. 若配置了 `CheckpointFile`：
   - `loadCheckpoint` 读取已有断点
   - 计算 `pendingDomains`（未完成的域名）
3. 创建任务 channel
4. 启动 `Concurrency` 个 worker goroutine
5. 每个 worker：
   - 应用 `QueryDelay` 限速
   - 调用 `ExecuteQueryWithResultContextContext`
   - 结果写入 `resultChan`
   - 原子计数 + 触发断点保存
:::

::: details 断点原子写入
断点文件采用原子写入策略：先写入 `.tmp` 临时文件，再通过 `os.Rename` 原子替换正式文件，避免写入中途崩溃导致断点损坏。
:::

::: details 预估剩余时间
`EstimatedRemaining` 计算公式：

```
avgPerDomain = Elapsed / Completed
remaining = TotalTasks - Completed
EstimatedRemaining = avgPerDomain * remaining
```

随已完成数增加，预估值逐步收敛。
:::

::: details 域间限速 QueryDelay
`QueryDelay` 在每个 worker 取到任务后、实际查询前引入固定延迟（毫秒），用于降低对 WHOIS 服务器的瞬时压力。与 [ratelimit.md](./ratelimit.md) 的令牌桶不同，这是 worker 级别的简单延迟。
:::

---

## 📝 使用示例

### 示例 1：完整批量流程

```go
config := whois.DefaultStreamBatchConfig()
config.Concurrency = 20
config.CheckpointFile = "batch_checkpoint.json"

p := whois.NewStreamBatchProcessor(config)
p.OnProgress(func(s whois.StreamBatchStats) {
    fmt.Printf("\r[%d/%d] 成功 %d 失败 %d 剩余 %vms",
        s.Completed, s.TotalTasks, s.SuccessCount, s.FailureCount, s.EstimatedRemaining)
})

domains := []string{"google.com", "github.com", "example.com"}
if err := p.Process(ctx, domains); err != nil {
    log.Fatal(err)
}

// 阻塞收集
results := whois.CollectResults(p.Results())
fmt.Printf("\n完成，共 %d 条结果\n", len(results))
```

### 示例 2：断点续查

```go
// 从已有断点恢复
p, err := whois.ResumeFromCheckpoint(ctx, whois.DefaultStreamBatchConfig())
if err != nil {
    log.Fatal(err)
}
p.Process(ctx, nil) // 域名列表从断点加载
```

### 示例 3：手动取消

```go
go func() {
    time.Sleep(30 * time.Second)
    p.Cancel() // 取消并保存断点
}()
p.Process(ctx, bigDomainList)
```

---

## ⚠️ 注意事项

- `CollectResults` 是阻塞调用，会等到 `resultChan` 关闭后才返回。
- 进程异常退出时，只要配置了 `CheckpointFile`，下次可用 `ResumeFromCheckpoint` 恢复未完成任务。
- `Concurrency` 不宜过高，建议 5-20，避免触发注册局限速。

---

## 🔗 相关

- 🔎 [query.md](./query.md) — 单次查询引擎
- 🚦 [ratelimit.md](./ratelimit.md) — 速率限制
- 🎯 [批量查询教程](../../guide/tutorial-batch.md)
