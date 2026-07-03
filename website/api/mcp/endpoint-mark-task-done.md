# 📡 POST /mark_task_done — 标记任务完成

> 📖 将任务从 `pending` 推进到 `done`，并记录完成说明；任务归属校验失败会报错。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/mark_task_done` 或 `/mcp/mark_task_done` |
| Controller 方法 | `MarkTaskDone(MarkTaskDoneInput)` |
| 状态变更 | 任务 `pending` → `done`；请求可能进入 `in_progress` |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |
| `taskId` | string | 是 | 任务 ID |
| `completedDetails` | string | 否 | 完成说明，写入 `task.Details` |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/mark_task_done \
  -H "Content-Type: application/json" \
  -d '{
    "requestId": "a1b2c3d4-....",
    "taskId": "task-id-1",
    "completedDetails": "已查询到注册商为 Example Registrar"
  }'
```

---

## 📤 响应示例

```json
{
  "requestId": "a1b2c3d4-....",
  "taskId": "task-id-1",
  "message": "任务 查 WHOIS 已标记为完成",
  "progress": "进度: 1/2 完成, 0/2 已批准"
}
```

---

## 🔄 状态转换

```
Task: pending ──mark_task_done──▶ done
```

- 任务 `CompletedAt` 被填写。
- `UpdateTask` 联动：若请求内并非全部任务 `approved`，请求置为 `in_progress`。

---

## 🔗 相关

- 📡 [批准任务](./endpoint-approve-task.md)
- 📡 [获取下一个任务](./endpoint-get-next-task.md)
- 🎛️ [控制器 controller.go](./controller.md)
