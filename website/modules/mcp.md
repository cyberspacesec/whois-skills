# 🤖 mcp 模块 — 任务状态机

> 📖 `pkg/mcp` 实现 Model Context Protocol 风格的任务编排状态机，管理 Request/Task 生命周期，同时封装 WHOIS 查询能力，通过 10 个 HTTP 端点对外提供。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `pkg/mcp` |
| 源文件数 | 3（另有 1 个 `_test.go`） |
| 职责 | 任务规划、状态流转、进度追踪、WHOIS 查询封装 |
| 依赖 | `pkg/whois`、`google/uuid`、`gorilla/mux` |

---

## 📁 文件清单

| 文件 | 职责 |
|------|------|
| `models.go` | `Request`/`Task`/`RequestStore` 数据模型与状态常量 |
| `controller.go` | `Controller` — 任务管理 + WHOIS 查询双能力 |
| `server.go` | `Server` — HTTP 处理器与 `http.HandlerFunc` 包装器 |

---

## 🔁 状态机

### Request 状态

| 状态 | 说明 |
|------|------|
| `pending` | 待处理 |
| `in_progress` | 进行中 |
| `done` | 已完成（所有任务 approved） |

### Task 状态

| 状态 | 说明 |
|------|------|
| `pending` | 待处理 |
| `done` | 已标记完成 |
| `approved` | 已批准 |
| `failed` | 失败 |

流转：`pending → done → approved`；当所有任务 `approved` 后，Request 自动转为 `done`。

---

## 🧠 Controller 双能力

### 任务管理方法

| 方法 | 说明 |
|------|------|
| `PlanRequest(input)` | 规划新请求并创建任务 |
| `GetNextTask(input)` | 获取下一个待处理任务 |
| `MarkTaskDone(input)` | 标记任务完成 |
| `ApproveTaskCompletion(input)` | 批准任务完成 |
| `ApproveRequestCompletion(input)` | 批准请求完成 |
| `ListRequests()` | 列出所有请求 |
| `AddTasksToRequest(input)` | 向请求追加任务 |
| `UpdateTask(input)` | 更新任务标题/描述 |
| `DeleteTask(input)` | 删除任务 |
| `GetTaskDetails(input)` | 获取任务详情 |

### WHOIS 查询方法

| 方法 | 说明 |
|------|------|
| `ExecuteWhoisQuery(domain)` | 简版域名查询（旧 API） |
| `ExecuteWhoisQueryFull(ctx, input)` | 完整域名查询（返回结果+校验） |
| `ExecuteIPWhoisQuery(ctx, input)` | IP WHOIS 查询 |
| `ExecuteASNQuery(ctx, input)` | ASN 查询 |
| `ExecuteRDAPQuery(ctx, input)` | RDAP 统一查询（domain/ip/asn） |
| `CheckAvailability(ctx, input)` | 域名可用性 |
| `CompareWhoisInfo(ctx, input)` | WHOIS 对比 |
| `AssessWhoisQuality(ctx, input)` | 质量评估 |
| `NormalizeDomainName(input)` | 域名规范化 |

---

## 🌍 HTTP 端点（10 个）

由 `api.Server.registerMCPRoutes` 注册到 `/api/mcp/*`：

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/mcp/request_planning` | POST | 规划请求 |
| `/api/mcp/get_next_task` | POST | 获取下一任务 |
| `/api/mcp/mark_task_done` | POST | 标记完成 |
| `/api/mcp/approve_task_completion` | POST | 批准任务 |
| `/api/mcp/approve_request_completion` | POST | 批准请求 |
| `/api/mcp/open_task_details` | POST | 任务详情 |
| `/api/mcp/list_requests` | GET | 列出请求 |
| `/api/mcp/add_tasks_to_request` | POST | 追加任务 |
| `/api/mcp/update_task` | POST | 更新任务 |
| `/api/mcp/delete_task` | POST | 删除任务 |

::: details Server 双路由集成方式
`Server` 同时支持 `gorilla/mux`（`RegisterRoutes`）与标准 `http.ServeMux`（`Handle*()` 包装器）。主服务通过 `Handle*()` 包装器集成到 `/api/mcp/*`。
:::

---

## 🚀 使用示例

```go
mcpServer := mcp.NewServer(logrus.StandardLogger())

// 规划请求
output, _ := mcpServer.controller.PlanRequest(mcp.RequestPlanningInput{
    OriginalRequest: "查询 example.com 全部信息",
    Tasks: []mcp.TaskInput{
        {Title: "WHOIS 查询", Description: "查询 example.com"},
    },
})
// output.RequestID
```

---

## ⚠️ 注意事项

- `RequestStore` 为**内存存储**，进程重启后数据丢失。
- WHOIS 查询方法封装的是 `pkg/whois` 能力，与 [api 模块](./api.md) 的端点功能重叠，但走 MCP 任务模型。

---

## 🔗 相关链接

- [MCP 协议总览](../api/mcp/overview.md)
- [api 模块](./api.md)
- [模块总览](./overview.md)
