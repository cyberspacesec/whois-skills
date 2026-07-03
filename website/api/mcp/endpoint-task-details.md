# 📡 POST /open_task_details — 任务详情

> 📖 按任务 ID 查询任务的完整信息，含状态、时间戳与完成说明。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/open_task_details` 或 `/mcp/open_task_details` |
| Controller 方法 | `GetTaskDetails(TaskDetailsInput)` |
| 状态变更 | 无（只读） |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `taskId` | string | 是 | 任务 ID |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/open_task_details \
  -H "Content-Type: application/json" \
  -d '{"taskId": "task-id-1"}'
```

---

## 📤 响应示例

```json
{
  "task": {
    "id": "task-id-1",
    "title": "查 WHOIS",
    "description": "查询域名注册信息",
    "status": "done",
    "created_at": "2026-07-03T10:00:00Z",
    "updated_at": "2026-07-03T10:01:00Z",
    "completed_at": "2026-07-03T10:01:00Z",
    "details": "已查询到注册商为 Example Registrar"
  },
  "message": "任务详情获取成功"
}
```

- `completed_at` 与 `details` 均带 `omitempty`，未完成时不出现。
- 直接返回 store 中的 `*Task` 指针序列化结果。

---

## 🔄 状态转换

只读，不改变状态。

---

## 🔗 相关

- 📡 [列出请求](./endpoint-list-requests.md)
- 📡 [更新任务](./endpoint-update-task.md)
- 🗂️ [数据模型 models.go](./models.md)
