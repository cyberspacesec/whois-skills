# 📡 POST /delete_task — 删除任务

> 📖 从请求中删除指定任务；拒绝删除已 `done` 或 `approved` 的任务。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/delete_task` 或 `/mcp/delete_task` |
| Controller 方法 | `DeleteTask(DeleteTaskInput)` |
| 状态变更 | 从请求任务切片移除，并从全局 tasks 表删除 |
| 限制 | 拒绝删除 `done`/`approved` 任务 |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |
| `taskId` | string | 是 | 任务 ID |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/delete_task \
  -H "Content-Type: application/json" \
  -d '{
    "requestId": "a1b2c3d4-....",
    "taskId": "task-id-1"
  }'
```

---

## 📤 响应示例

```json
{
  "request_id": "a1b2c3d4-....",
  "message": "任务 '查 WHOIS' 已删除",
  "progress": "进度: 0/1 完成, 0/1 已批准"
}
```

::: warning 注意字段名
响应中请求 ID 的 JSON key 为 `request_id`（与其他端点的 `requestId` 不同，源自 `DeleteTaskOutput` 的标签定义）。
:::

---

## 🔄 状态转换

```
Task: pending ──delete_task──▶ 从请求与全局表移除
Task: done | approved ──delete_task──▶ 拒绝（错误）
```

- 从 `request.Tasks` 切片删除对应索引，刷新 `request.UpdatedAt`。
- 加锁从 `RequestStore.tasks` 全局映射表 `delete`。

### 错误情形

- 任务为 `done` 或 `approved`：返回 `无法删除已完成或已批准的任务`。
- 任务不存在于请求中：返回 `任务不存在于请求中`。

下图展示删除任务端点的状态校验与双表移除流程，已 done/approved 任务受保护不可删。

```mermaid
flowchart TD
  Req([🌐 POST /delete_task<br/>{requestId, taskId}]) --> S[🌐 mcp.Server]
  S --> Ctrl[🎛️ DeleteTask]
  Ctrl --> V1{🔍 任务存在于<br/>该请求?}
  V1 -- 否 --> E1[❌ 任务不存在于请求中]
  V1 -- 是 --> V2{⚙️ 任务非 done/approved?}
  V2 -- 否 --> E2[❌ 无法删除已完成/已批准]
  V2 -- 是 --> Del1[✂️ 从 request.Tasks 切片移除]
  Del1 --> Del2[✂️ delete tasks 全局表]
  Del2 --> Upd[🔄 刷新 request.UpdatedAt]
  Upd --> Prog[📊 生成进度]
  Prog --> Resp([📤 200 任务已删除<br/>request_id 字段名])
  E1 & E2 --> Resp

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class S,Ctrl,Del1,Del2,Upd,Prog svc
  class V1,V2 check
  class E1,E2 err
```

---

## 🔗 相关

- 📡 [追加任务](./endpoint-add-tasks.md)
- 📡 [更新任务](./endpoint-update-task.md)
- 🎛️ [控制器 controller.go](./controller.md)
