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

下图展示标记任务完成的处理流程与请求状态联动逻辑。

```mermaid
flowchart TD
  Req([🌐 POST /mark_task_done<br/>{requestId, taskId, completedDetails}]) --> S[🌐 mcp.Server]
  S --> Ctrl[🎛️ MarkTaskDone]
  Ctrl --> V1{🔍 任务归属<br/>该请求?}
  V1 -- 否 --> E1[❌ 任务不属于指定请求]
  V1 -- 是 --> V2{⚙️ 任务为 pending?}
  V2 -- 否 --> E2[❌ 状态不允许]
  V2 -- 是 --> Upd[✏️ UpdateTask<br/>pending→done<br/>填 CompletedAt/Details]
  Upd --> Link[🔁 请求联动检查]
  Link --> Ck{所有任务 approved?}
  Ck -- 是 --> RD[请求→done]
  Ck -- 否 --> RP[请求→in_progress]
  RD & RP --> Prog[📊 生成进度]
  Prog --> Resp([📤 200 响应])
  E1 & E2 --> Resp

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class S,Ctrl,Upd,Link,RD,RP,Prog svc
  class V1,V2,Ck check
  class E1,E2 err
```

---

## 🔗 相关

- 📡 [批准任务](./endpoint-approve-task.md)
- 📡 [获取下一个任务](./endpoint-get-next-task.md)
- 🎛️ [控制器 controller.go](./controller.md)
