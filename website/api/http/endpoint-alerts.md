# 🚨 告警端点 — GET /api/alerts

> 📖 告警历史查询端点，需服务器开启 `EnableAlerts`，返回 `metrics.GetAlertManager().GetHistory()` 的告警事件列表。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/alerts` |
| 方法 | `GET` |
| 处理器 | `handleAlerts` |
| 前置条件 | `s.EnableAlerts == true` |
| 底层函数 | `metrics.GetAlertManager().GetHistory()` |

---

## 📝 请求

### 请求参数

无。

### curl 示例

```bash
curl http://127.0.0.1:8080/api/alerts
```

::: warning 前置条件
必须在创建服务器时设置 `s.EnableAlerts = true`，否则返回 `503`。
:::

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": [
    {
      "id": "alert-001",
      "type": "high_error_rate",
      "level": "warning",
      "message": "WHOIS 查询错误率超过阈值: 15%",
      "value": 0.15,
      "threshold": 0.10,
      "triggered_at": "2026-07-03T10:30:00Z",
      "resolved": false
    },
    {
      "id": "alert-002",
      "type": "high_latency",
      "level": "critical",
      "message": "平均延迟过高: 2500ms",
      "value": 2500,
      "threshold": 2000,
      "triggered_at": "2026-07-03T11:00:00Z",
      "resolved": true,
      "resolved_at": "2026-07-03T11:15:00Z"
    }
  ]
}
```

### AlertEvent 结构说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `string` | 告警 ID |
| `type` | `string` | 告警类型（如 `high_error_rate`、`high_latency`） |
| `level` | `string` | 严重级别（`info` / `warning` / `critical`） |
| `message` | `string` | 告警描述 |
| `value` | `float64` | 触发时的实际值 |
| `threshold` | `float64` | 触发阈值 |
| `triggered_at` | `time.Time` | 触发时间 |
| `resolved` | `bool` | 是否已恢复 |
| `resolved_at` | `time.Time` | 恢复时间（仅 `resolved=true` 时） |

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `503` | `EnableAlerts = false` | `告警功能未启用` |
| `405` | 非 GET 方法 | `仅支持GET请求` |

::: tip 检查顺序
先检查 `EnableAlerts`（未启用返回 503），再检查方法（非 GET 返回 405）。
:::

下图展示 alerts 端点的检查顺序与告警事件的生命周期，告警由阈值触发、可被恢复。

```mermaid
flowchart TD
  Req([🌐 GET /api/alerts]) --> MW[🛡️ 中间件链]
  MW --> C1{🔍 EnableAlerts?}
  C1 -- false --> E1[❌ 503 告警功能未启用]
  C1 -- true --> C2{⚙️ 方法为 GET?}
  C2 -- 否 --> E2[❌ 405 仅支持GET请求]
  C2 -- 是 --> AM[🚨 metrics.GetAlertManager]
  AM --> GH[📦 GetHistory]
  GH --> Resp([✅ 200 返回告警列表])
  E1 & E2 & Resp --> Out([📤 HTTP 响应])

  subgraph Life[⏰ 告警生命周期]
    T[触发: value 超过 threshold] --> Act[active: resolved=false]
    Act --> Res[恢复: resolved=true]
  end

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class MW,AM,GH,T,Act,Res svc
  class C1,C2 check
  class E1,E2 err
```

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 🖥️ [server.md](./server.md) — `EnableAlerts` 配置
- 📊 [endpoint-metrics.md](./endpoint-metrics.md) — 监控指标端点
