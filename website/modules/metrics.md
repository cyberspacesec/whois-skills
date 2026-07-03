# 📊 metrics 模块 — 监控告警

> 📖 `pkg/metrics` 提供指标采集、告警规则与多通道通知能力，采集 API/WHOIS/缓存/系统四类指标，内置 4 条默认告警规则，支持 Email/Slack/Webhook 通知，系统指标基于 gopsutil。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `pkg/metrics` |
| 源文件数 | 4（另有 1 个 `_test.go`） |
| 职责 | 指标采集、告警规则评估、通知分发、系统指标采集 |
| 依赖 | `gopsutil/v3`（cpu/mem/load）、logrus |

---

## 📁 文件清单

| 文件 | 职责 |
|------|------|
| `collector.go` | `MetricsCollector` 单例 — 四类指标采集与导出 |
| `alert.go` | `AlertManager` 单例 — 告警规则、状态、历史 |
| `notifier.go` | Email/Slack/Webhook 三种通知器 |
| `system.go` | 系统指标采集（CPU/内存/负载，基于 gopsutil） |

---

## 📈 四类指标

`MetricsCollector` 维护四组指标：

| 类别 | 关键字段 | 采集入口 |
|------|----------|----------|
| **API** | TotalRequests、成功率、按路径/状态码统计、响应时间 | `RecordAPIRequest` |
| **WHOIS** | TotalQueries、成功率、按服务器统计、查询时间 | `RecordWHOISQuery` |
| **缓存** | 命中率、操作数 | `RecordCacheOperation` |
| **系统** | CPUUsage、MemoryUsage、LoadAvg | `collectSystemMetrics`（gopsutil） |

### 单例与启动

| 函数 | 说明 |
|------|------|
| `GetCollector() *MetricsCollector` | 获取采集器单例 |
| `GetAlertManager() *AlertManager` | 获取告警管理器单例 |
| `StartSystemMetricsCollection(interval)` | 启动周期性系统指标采集 |
| `StartAlertManager(interval)` | 启动周期性告警检查 |
| `collector.ExportMetrics(path)` | 导出指标到 JSON 文件 |
| `collector.GetMetrics()` | 获取全部指标 |

---

## 🚨 默认告警规则

`AlertManager` 内置 4 条规则（`registerDefaultRules`）：

| 规则名 | 描述 | 阈值 | 持续 | 级别 | 通道 |
|--------|------|------|------|------|------|
| `high_cpu_usage` | CPU 使用率过高 | > 80% | 5min | Warn | email, slack |
| `high_memory_usage` | 内存使用率过高 | > 90% | 5min | Warn | email, slack |
| `high_api_error_rate` | API 错误率过高 | > 10% | 5min | Error | email, slack |
| `high_whois_failure_rate` | WHOIS 失败率过高 | > 20% | 5min | Error | email, slack |

告警级别：`Info → Warn → Error → Critical`。

---

## 📬 通知器

`AlertNotifier` 接口实现：

| 通知器 | 字段要点 | 说明 |
|--------|----------|------|
| `EmailNotifier` | Host/Port/Username/Password、From/To/CC | SMTP 邮件 |
| `SlackNotifier` | WebhookURL、Channel、Username、IconEmoji | Slack Incoming Webhook |
| `WebhookNotifier` | URL、Method、Headers、FormatMessage | 通用 HTTP Webhook |

`RegisterDefaultNotifiers()` 注册默认通知器；`RegisterNotifier(name, notifier)` 自定义。

---

## 🌍 HTTP 端点

由 [api 模块](./api.md) 暴露：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/metrics` | GET | 返回全部指标（需 `EnableMetrics`，否则 503） |
| `/api/alerts` | GET | 返回告警历史（需 `EnableAlerts`，否则 503） |

---

## 🚀 使用示例

```go
// 初始化（通常在 cmd 中）
collector := metrics.GetCollector()
metrics.StartSystemMetricsCollection(60 * time.Second)

manager := metrics.GetAlertManager()
manager.RegisterDefaultNotifiers()
metrics.StartAlertManager(60 * time.Second)

// 记录指标
collector.RecordWHOISQuery("whois.verisign-grs.com", true, 120*time.Millisecond)

// 导出
collector.ExportMetrics("data/metrics.json")
```

---

## ⚠️ 注意事项

- 默认通知器的 SMTP/Slack/Webhook 配置需自行填充真实参数，否则通知发送会失败。
- 系统指标采集依赖 gopsutil，容器内需保证 `/proc` 等可访问。

---

## 🔗 相关链接

- [monitor 模块](./monitor.md) — WHOIS 查询性能监控（独立）
- [cmd 模块](./cmd.md) — `setupMetrics`/`setupAlerts`
- [模块总览](./overview.md)
