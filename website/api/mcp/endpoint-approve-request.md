# 📡 POST /approve_request_completion — 批准请求

> 📖 显式批准整个请求完成，将其状态置为 `done`；要求请求内所有任务均已 `approved`。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/approve_request_completion` 或 `/mcp/approve_request_completion` |
| Controller 方法 | `ApproveRequestCompletion(ApproveRequestInput)` |
| 状态变更 | 请求 → `done` |
| 前置条件 | 请求内所有任务均为 `approved` |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/approve_request_completion \
  -H "Content-Type: application/json" \
  -d '{"requestId": "a1b2c3d4-...."}'
```

---

## 📤 响应示例

```json
{
  "requestId": "a1b2c3d4-....",
  "message": "请求已完成并批准",
  "progress": "进度: 2/2 完成, 2/2 已批准"
}
```

---

## 🔄 状态转换

```
Request: in_progress (或 pending) ──approve_request_completion──▶ done
```

- 调用 `UpdateRequestStatus(requestID, RequestStatusDone)`，填 `CompletedAt`。

### 错误情形

- 存在任务未 `approved`：返回 `所有任务必须先被批准才能完成请求`。
- 请求不存在：返回 `请求 ID ... 不存在`。

::: tip 与 approve_task 的关系
通常 `approve_task_completion` 批准最后一个任务时，请求已自动联动为 `done`。本端点用于在所有任务 `approved` 后作显式终态确认，语义上等价但更清晰。
:::

---

## 🔗 相关

- 📡 [批准任务](./endpoint-approve-task.md)
- 📡 [列出请求](./endpoint-list-requests.md)
- 🎛️ [控制器 controller.go](./controller.md)
