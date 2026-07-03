# 📡 POST /update_task — 更新任务

> 📖 更新任务的标题或描述；仅非空字段更新，拒绝更新已 `done` 或 `approved` 的任务。

---

## 📋 概述

| 项目 | 内容 |
|------|------|
| 方法 | `POST` |
| 路径 | `/api/mcp/update_task` 或 `/mcp/update_task` |
| Controller 方法 | `UpdateTask(UpdateTaskInput)` |
| 状态变更 | 仅改 `Title`/`Description`/`UpdatedAt`，不改状态 |
| 限制 | 拒绝更新 `done`/`approved` 任务 |

---

## 📥 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `requestId` | string | 是 | 请求 ID |
| `taskId` | string | 是 | 任务 ID |
| `title` | string | 否 | 新标题，空则不更新 |
| `description` | string | 否 | 新描述，空则不更新 |

### curl 示例

```bash
curl -X POST http://localhost:8080/api/mcp/update_task \
  -H "Content-Type: application/json" \
  -d '{
    "requestId": "a1b2c3d4-....",
    "taskId": "task-id-1",
    "title": "查 WHOIS（修订）",
    "description": "查询域名注册信息并记录注册商"
  }'
```

---

## 📤 响应示例

```json
{
  "requestId": "a1b2c3d4-....",
  "taskId": "task-id-1",
  "message": "任务信息已更新",
  "progress": "进度: 0/2 完成, 0/2 已批准"
}
```

---

## 🔄 状态转换

不改任务状态，仅更新字段并刷新 `UpdatedAt`。

### 错误情形

- 任务为 `done` 或 `approved`：返回 `无法更新已完成或已批准的任务`。
- 任务不属于该请求：返回 `任务不属于指定的请求`。

::: tip 部分更新
两个字段均带 `omitempty`，传空字符串视为不更新——可实现只改标题或只改描述的部分更新。
:::

下图展示更新任务端点的状态校验与部分更新逻辑，拒绝改动已 done/approved 任务。

```mermaid
flowchart TD
  Req([🌐 POST /update_task<br/>{requestId, taskId, title, description}]) --> S[🌐 mcp.Server]
  S --> Ctrl[🎛️ UpdateTask]
  Ctrl --> V1{🔍 任务归属<br/>该请求?}
  V1 -- 否 --> E1[❌ 任务不属于指定请求]
  V1 -- 是 --> V2{⚙️ 任务非 done/approved?}
  V2 -- 否 --> E2[❌ 无法更新已完成/已批准]
  V2 -- 是 --> Upd[✏️ 仅非空字段更新<br/>Title/Description<br/>刷新 UpdatedAt]
  Upd --> Prog[📊 生成进度]
  Prog --> Resp([📤 200 任务信息已更新])
  E1 & E2 --> Resp

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class S,Ctrl,Upd,Prog svc
  class V1,V2 check
  class E1,E2 err
```

---

## 🔗 相关

- 📡 [追加任务](./endpoint-add-tasks.md)
- 📡 [删除任务](./endpoint-delete-task.md)
- 📡 [任务详情](./endpoint-task-details.md)
- 🎛️ [控制器 controller.go](./controller.md)
