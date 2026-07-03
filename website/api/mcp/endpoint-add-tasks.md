# 📡 POST /add_tasks_to_request — 追加任务

> 📖 向已有请求追加新任务；拒绝向已 `done` 的请求添加。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/add_tasks_to_request` 或 `/mcp/add_tasks_to_request` |
| Controller 方法 | `AddTasksToRequest(AddTasksInput)` |
| 状态变更 | 新任务以 `pending` 加入请求 |
| 限制 | 请求状态为 `done` 时拒绝 |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |
| `tasks` | array | 是 | 新任务列表，不可为空 |
| `tasks[].title` | string | 是 | 任务标题 |
| `tasks[].description` | string | 是 | 任务描述 |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/add_tasks_to_request \
  -H "Content-Type: application/json" \
  -d '{
    "requestId": "a1b2c3d4-....",
    "tasks": [
      {"title": "查 ASN", "description": "查询注册商 ASN 信息"}
    ]
  }'
```

---

## 📤 响应示例

```json
{
  "requestId": "a1b2c3d4-....",
  "tasks": [
    {"id": "task-id-3", "title": "查 ASN", "description": "查询注册商 ASN 信息"}
  ],
  "message": "已向请求添加 1 个新任务",
  "progress": "进度: 1/3 完成, 0/3 已批准"
}
```

- 新任务由 `uuid.New()` 生成 ID，状态 `pending`。
- 同时登记进 `RequestStore.tasks` 全局映射表。

---

## 🔄 状态转换

```
Request: pending | in_progress ──add_tasks_to_request──▶ 追加 pending 任务
Request: done                   ──add_tasks_to_request──▶ 拒绝（错误）
```

### 错误情形

- 请求已 `done`：返回 `无法向已完成的请求添加任务`。
- 请求不存在：返回 `请求 ID ... 不存在`。
- 任务列表为空：返回 `任务列表不能为空`。

---

## 🔗 相关

- 📡 [请求规划](./endpoint-request-planning.md)
- 📡 [更新任务](./endpoint-update-task.md)
- 📡 [删除任务](./endpoint-delete-task.md)
- 🎛️ [控制器 controller.go](./controller.md)
