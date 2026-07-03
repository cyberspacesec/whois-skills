# 📡 POST /get_next_task — 获取下一个任务

> 📖 取指定请求中下一个 `pending` 任务；若无待处理任务，返回 `all_tasks_done=true` 且不带 `task`。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/get_next_task` 或 `/mcp/get_next_task` |
| Controller 方法 | `GetNextTask(GetNextTaskInput)` |
| 状态变更 | 无（只读，返回首个 pending 任务） |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/get_next_task \
  -H "Content-Type: application/json" \
  -d '{"requestId": "a1b2c3d4-...."}'
```

---

## 📤 响应示例

### 有待处理任务

```json
{
  "requestId": "a1b2c3d4-....",
  "all_tasks_done": false,
  "task": {
    "id": "task-id-1",
    "title": "查 WHOIS",
    "description": "查询域名注册信息",
    "status": "pending"
  },
  "progress": "进度: 0/2 完成, 0/2 已批准"
}
```

### 全部完成

```json
{
  "requestId": "a1b2c3d4-....",
  "all_tasks_done": true,
  "progress": "进度: 2/2 完成, 2/2 已批准"
}
```

::: tip 无 task 字段
`all_tasks_done=true` 时 `task` 字段被 `omitempty` 省略，调用方应优先判断 `all_tasks_done`。
:::

下图展示取下一个待处理任务的判定流程，无 pending 任务时返回 `all_tasks_done=true`。

```mermaid
flowchart TD
  Req([🌐 POST /get_next_task<br/>{requestId}]) --> S[🌐 mcp.Server]
  S --> Ctrl[🎛️ GetNextTask]
  Ctrl --> Store[🗂️ RequestStore]
  Store --> Find[🔍 GetNextPendingTask]
  Find --> R{有 pending 任务?}
  R -- 是 --> Out1[✅ 返回 task<br/>all_tasks_done=false]
  R -- 否 nil, nil --> Out2[✅ 返回<br/>all_tasks_done=true<br/>省略 task]
  Out1 & Out2 --> Prog[📊 生成进度信息<br/>X/Y 完成, Z/Y 已批准]
  Prog --> Resp([📤 200 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef infra fill:#909399,color:#fff,stroke:#6b6e72

  class Req,Resp entry
  class S,Ctrl,Out1,Out2,Prog svc
  class R check
  class Store,Find infra
```

---

## 🔄 状态转换

本端点为只读，不改变任何状态。`progress` 中"完成"含 `done` 与 `approved`。

---

## 🔗 相关

- 📡 [标记任务完成](./endpoint-mark-task-done.md)
- 📡 [请求规划](./endpoint-request-planning.md)
- 🎛️ [控制器 controller.go](./controller.md)
