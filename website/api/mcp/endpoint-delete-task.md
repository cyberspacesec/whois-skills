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

---

## 🔗 相关

- 📡 [追加任务](./endpoint-add-tasks.md)
- 📡 [更新任务](./endpoint-update-task.md)
- 🎛️ [控制器 controller.go](./controller.md)
