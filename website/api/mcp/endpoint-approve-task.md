# 📡 POST /approve_task_completion — 批准任务

> 📖 将已完成的任务（`done`）批准为 `approved`；当请求内所有任务均为 `approved` 时，请求自动联动为 `done`。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/approve_task_completion` 或 `/mcp/approve_task_completion` |
| Controller 方法 | `ApproveTaskCompletion(ApproveTaskInput)` |
| 状态变更 | 任务 `done` → `approved`；可能触发请求 → `done` |
| 前置条件 | 任务必须先处于 `done` |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |
| `taskId` | string | 是 | 任务 ID |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/approve_task_completion \
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
  "requestId": "a1b2c3d4-....",
  "taskId": "task-id-1",
  "message": "任务 查 WHOIS 已批准完成",
  "progress": "进度: 1/2 完成, 1/2 已批准"
}
```

---

## 🔄 状态转换

```
Task: done ──approve_task_completion──▶ approved
```

::: details 请求自动联动
`UpdateTask` 在任务变更为 `approved` 后，遍历所属请求检查是否所有任务均 `approved`：
- 是 → `request.Status = done`，填 `CompletedAt`；
- 否 → `request.Status = in_progress`。

因此批准最后一个任务后，请求可能直接跳到 `done`，无需再调 `approve_request_completion`（但显式批准仍是推荐做法，可作终态确认）。
:::

### 错误情形

- 任务非 `done` 状态：返回 `任务必须先标记为完成才能批准`。
- 任务不属于该请求：返回 `任务不属于指定的请求`。

---

## 🔗 相关

- 📡 [标记任务完成](./endpoint-mark-task-done.md)
- 📡 [批准请求](./endpoint-approve-request.md)
- 🎛️ [控制器 controller.go](./controller.md)
