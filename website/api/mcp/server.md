# 🌐 server.go — MCP 服务端

> 📖 `Server` 把 `Controller` 的能力暴露为 HTTP JSON API，提供 gorilla/mux 路由注册与 `http.HandlerFunc` 包装两套集成方式，统一处理解码、调用与错误响应。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/mcp/server.go` |
| 结构 | `Server { controller *Controller; logger *logrus.Logger }` |
| 构造 | `NewServer(logger *logrus.Logger) *Server`（内部 `NewController()`） |
| 依赖 | `github.com/gorilla/mux`、`github.com/sirupsen/logrus` |

```go
type Server struct {
    controller *Controller
    logger     *logrus.Logger
}

func NewServer(logger *logrus.Logger) *Server {
    return &Server{controller: NewController(), logger: logger}
}
```

---

## 🔀 两种集成方式

### A. gorilla/mux —— `/mcp/*`

`RegisterRoutes(router *mux.Router)` 注册带方法约束的路由：

```go
func (s *Server) RegisterRoutes(router *mux.Router) {
    router.HandleFunc("/mcp/request_planning", s.handleRequestPlanning).Methods("POST")
    // ...
}
```

### B. http.ServeMux —— `/api/mcp/*`

`pkg/api/server.go` 的 `registerMCPRoutes` 调用各 `HandleXxx()` 包装器（返回 `http.HandlerFunc`）：

```go
mcpServer := mcp.NewServer(logrus.StandardLogger())
router.HandleFunc("/api/mcp/request_planning", mcpServer.HandleRequestPlanning())
// ...
```

两类方法最终都委托给内部未导出的 `handleXxx`。

下图展示两套 HTTP 集成路径如何复用同一套 `handleXxx` 处理逻辑，统一经过「解码 → Controller → 响应」的处理模式。

```mermaid
flowchart TD
  subgraph Routes[📡 两条 HTTP 路径]
    P1[/mcp/*<br/>gorilla/mux<br/>RegisterRoutes]
    P2[/api/mcp/*<br/>http.ServeMux<br/>HandleXxx 包装器]
  end

  P1 --> H[⚙️ handleXxx<br/>未导出]
  P2 --> H

  H --> Dec[🔧 json.Decode<br/>请求体]
  Dec --> DCheck{解码成功?}
  DCheck -- 否 --> E1[❌ 400 无效的请求格式]
  DCheck -- 是 --> Ctrl[🎛️ Controller 方法]
  Ctrl --> CCheck{返回 error?}
  CCheck -- 是 --> E2[❌ 500 err.Error]
  CCheck -- 否 --> OK[✅ respondJSON<br/>200 + 数据]
  E1 & E2 & OK --> Resp([📤 HTTP 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class P1,P2 entry
  class H,Dec,Ctrl,OK svc
  class DCheck,CCheck check
  class E1,E2 err
```

---

## 🧱 HandleXxx() 包装器

| 包装器 | 对应 handler |
|--------|--------------|
| `HandleRequestPlanning()` | `handleRequestPlanning` |
| `HandleGetNextTask()` | `handleGetNextTask` |
| `HandleMarkTaskDone()` | `handleMarkTaskDone` |
| `HandleApproveTaskCompletion()` | `handleApproveTaskCompletion` |
| `HandleApproveRequestCompletion()` | `handleApproveRequestCompletion` |
| `HandleOpenTaskDetails()` | `handleOpenTaskDetails` |
| `HandleListRequests()` | `handleListRequests` |
| `HandleAddTasksToRequest()` | `handleAddTasksToRequest` |
| `HandleUpdateTask()` | `handleUpdateTask` |
| `HandleDeleteTask()` | `handleDeleteTask` |

---

## 📡 端点表

| 路径 | 方法 | Controller 调用 |
|------|------|------------------|
| `/mcp/request_planning` | POST | `PlanRequest` |
| `/mcp/get_next_task` | POST | `GetNextTask` |
| `/mcp/mark_task_done` | POST | `MarkTaskDone` |
| `/mcp/approve_task_completion` | POST | `ApproveTaskCompletion` |
| `/mcp/approve_request_completion` | POST | `ApproveRequestCompletion` |
| `/mcp/open_task_details` | POST | `GetTaskDetails` |
| `/mcp/list_requests` | GET | `ListRequests` |
| `/mcp/add_tasks_to_request` | POST | `AddTasksToRequest` |
| `/mcp/update_task` | POST | `UpdateTask` |
| `/mcp/delete_task` | POST | `DeleteTask` |

::: tip /api/mcp/* 集成
上表路径前缀替换为 `/api/mcp/*` 即为主 API 服务器暴露的等价端点，由 `pkg/api/server.go` 注册。
:::

---

## 🔧 处理流程与辅助方法

每个 `handleXxx` 遵循相同模式：

```
解码 JSON → 调用 Controller → 返回 JSON
```

| 辅助方法 | 作用 |
|----------|------|
| `respondJSON(w, statusCode, data)` | 写 JSON 响应，设 `Content-Type: application/json` |
| `respondError(w, statusCode, message)` | 返回 `{"error": message}` |

### 状态码约定

| 情形 | 状态码 |
|------|--------|
| JSON 解码失败 | `400 Bad Request`（消息 "无效的请求格式"） |
| Controller 返回 error | `500 Internal Server Error`（消息为 `err.Error()`） |
| 成功 | `200 OK` |

::: details handleXxx 示意
```go
func (s *Server) handleRequestPlanning(w http.ResponseWriter, r *http.Request) {
    var input RequestPlanningInput
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        s.respondError(w, http.StatusBadRequest, "无效的请求格式")
        return
    }
    output, err := s.controller.PlanRequest(input)
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }
    s.respondJSON(w, http.StatusOK, output)
}
```
`list_requests` 无请求体，直接调用 Controller。
:::

---

## 🚀 使用示例

### 方式一：挂到 gorilla/mux

```go
import (
    "github.com/gorilla/mux"
    "github.com/sirupsen/logrus"
    "github.com/cyberspacesec/whois-skills/pkg/mcp"
)

r := mux.NewRouter()
mcpServer := mcp.NewServer(logrus.New())
mcpServer.RegisterRoutes(r)   // /mcp/*
http.ListenAndServe(":8080", r)
```

### 方式二：集成进主 API 的 ServeMux

```go
mux := http.NewServeMux()
mcpServer := mcp.NewServer(logrus.StandardLogger())
mux.HandleFunc("/api/mcp/request_planning", mcpServer.HandleRequestPlanning())
mux.HandleFunc("/api/mcp/list_requests",    mcpServer.HandleListRequests())
// ...其余 HandleXxx()
```

---

## 🔗 相关

- 🎛️ [控制器 controller.go](./controller.md)
- 🗂️ [数据模型 models.go](./models.md)
- 🧭 [MCP 概览](./overview.md)
