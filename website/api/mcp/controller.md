# 🎛️ controller.go — MCP 控制器

> 📖 `Controller` 是 MCP 协议的中枢，既管理请求/任务的生命周期，又封装 `pkg/whois` 的全部查询能力，对外提供统一的方法签名。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/mcp/controller.go` |
| 结构 | `Controller { store *RequestStore }` |
| 构造 | `NewController() *Controller`（内部 `NewRequestStore()`） |
| 依赖 | `pkg/whois`、`github.com/google/uuid`、`whois-parser` |

```go
type Controller struct {
    store *RequestStore
}

func NewController() *Controller {
    return &Controller{store: NewRequestStore()}
}
```

---

## 🅰️ 任务管理方法

| 方法 | 输入 | 输出 | 关键行为 |
|------|------|------|----------|
| `PlanRequest` | `RequestPlanningInput` | `*RequestPlanningOutput, error` | 用 `uuid` 生成 requestID 与各 taskID，初始状态 `pending` |
| `GetNextTask` | `GetNextTaskInput` | `*GetNextTaskOutput, error` | 返回下一个 pending 任务；无则 `all_tasks_done=true` |
| `MarkTaskDone` | `MarkTaskDoneInput` | `*MarkTaskDoneOutput, error` | pending→done，校验任务归属 |
| `ApproveTaskCompletion` | `ApproveTaskInput` | `*ApproveTaskOutput, error` | **需先 done**，done→approved |
| `ApproveRequestCompletion` | `ApproveRequestInput` | `*ApproveRequestOutput, error` | **需所有任务 approved**，请求→done |
| `ListRequests` | — | `*ListRequestsOutput, error` | 列出全部请求摘要 |
| `AddTasksToRequest` | `AddTasksInput` | `*AddTasksOutput, error` | **拒绝向 done 请求添加** |
| `UpdateTask` | `UpdateTaskInput` | `*UpdateTaskOutput, error` | **拒绝更新 done/approved**，非空字段才更新 |
| `DeleteTask` | `DeleteTaskInput` | `*DeleteTaskOutput, error` | **拒绝删除 done/approved** |
| `GetTaskDetails` | `TaskDetailsInput` | `*TaskDetailsOutput, error` | 返回完整 `*Task` |

### 输入 / 输出类型

```go
type TaskInput struct {
    Title       string `json:"title"`
    Description string `json:"description"`
}

type RequestPlanningInput struct {
    OriginalRequest string      `json:"originalRequest"`
    Tasks           []TaskInput `json:"tasks"`
    SplitDetails    string      `json:"splitDetails,omitempty"`
}

type RequestPlanningOutput struct {
    RequestID string              `json:"requestId"`
    Tasks     []map[string]string `json:"tasks"`   // 每项含 id/title/description
    Message   string              `json:"message"`
}

type GetNextTaskInput  struct { RequestID string `json:"requestId"` }
type GetNextTaskOutput struct {
    RequestID    string      `json:"requestId"`
    AllTasksDone bool        `json:"all_tasks_done"`
    Task         *TaskOutput `json:"task,omitempty"`
    Progress     string      `json:"progress"`
}
type TaskOutput struct {
    ID, Title, Description, Status string
}

type MarkTaskDoneInput struct {
    RequestID        string `json:"requestId"`
    TaskID           string `json:"taskId"`
    CompletedDetails string `json:"completedDetails,omitempty"`
}
type MarkTaskDoneOutput struct {
    RequestID, TaskID, Message, Progress string
}

type ApproveTaskInput  struct { RequestID, TaskID string }
type ApproveTaskOutput struct { RequestID, TaskID, Message, Progress string }

type ApproveRequestInput  struct { RequestID string `json:"requestId"` }
type ApproveRequestOutput struct { RequestID, Message, Progress string }

type ListRequestsOutput struct {
    Requests []RequestSummary `json:"requests"`
    Message  string           `json:"message"`
}
type RequestSummary struct {
    ID, OriginalRequest string
    Status              RequestStatus
    TaskCount           int
    CompletedTasks      int
    CreatedAt           time.Time
}

type AddTasksInput  struct { RequestID string; Tasks []TaskInput }
type AddTasksOutput struct {
    RequestID string              `json:"requestId"`
    Tasks     []map[string]string `json:"tasks"`
    Message, Progress             string
}

type UpdateTaskInput struct {
    RequestID, TaskID, Title, Description string
}
type UpdateTaskOutput struct{ RequestID, TaskID, Message, Progress string }

type DeleteTaskInput struct{ RequestID, TaskID string }
type DeleteTaskOutput struct {
    RequestID string `json:"request_id"`
    Message, Progress string
}

type TaskDetailsInput  struct { TaskID string `json:"taskId"` }
type TaskDetailsOutput struct {
    Task    *Task  `json:"task"`
    Message string `json:"message"`
}
```

### 进度信息

`generateProgressInfo(request)` 遍历任务，统计 `done`（含 `approved`）与 `approved` 数量，返回：

```
进度: X/Y 完成, Z/Y 已批准
```

---

## 🅱️ WHOIS 查询方法

| 方法 | 输入 | 输出 | 说明 |
|------|------|------|------|
| `ExecuteWhoisQuery` | `domain string` | `*whoisparser.WhoisInfo, error` | 简易版，委托 `whois.Execute` |
| `ExecuteWhoisQueryFull` | `ctx, WhoisQueryInput` | `*WhoisQueryOutput, error` | 完整版，默认 timeout=10s |
| `ExecuteIPWhoisQuery` | `ctx, IPQueryInput` | `*IPQueryOutput, error` | IP WHOIS |
| `ExecuteASNQuery` | `ctx, ASNQueryInput` | `*ASNQueryOutput, error` | source 映射 `radb`/`rdap`/`all` |
| `ExecuteRDAPQuery` | `ctx, RDAPQueryInput` | `*RDAPQueryOutput, error` | Type 支持 `domain`/`ip`/`asn` |
| `CheckAvailability` | `ctx, AvailabilityInput` | `*AvailabilityOutput, error` | 域名可用性 |
| `CompareWhoisInfo` | `ctx, WhoisCompareInput` | `*WhoisCompareOutput, error` | 两域名对比 |
| `AssessWhoisQuality` | `ctx, QualityInput` | `*QualityOutput, error` | 质量评分 |
| `NormalizeDomainName` | `NormalizeInput` | `*NormalizeOutput, error` | action: `normalize`/`to_punycode`/`to_unicode` |

### 输入 / 输出类型

```go
type WhoisQueryInput struct {
    Domain         string
    UseProxy       bool
    Timeout        int   // 默认 10
    MaxRetries     int
    ValidateResult bool
    RequiredFields []string
}
type WhoisQueryOutput struct {
    Info    *whoisparser.WhoisInfo
    Raw     string
    Server  string
    Latency int64
    Retries int
    Valid   bool
    Errors  []string
}

type IPQueryInput  struct { IP string; Timeout int; UseProxy bool }
type IPQueryOutput struct {
    IP, RawResponse, Server string
    Latency                 int64
    Info                    *whoisparser.WhoisInfo
}

type ASNQueryInput struct {
    ASN             int
    Source          string // radb / rdap / all
    Timeout         int
    IncludePrefixes bool
    IncludeBGP      bool
}
type ASNQueryOutput struct {
    ASN          int
    Name         string
    Organization string
    Country      string
    RIR          string
    Description  string
    IPv4Prefixes []string
    IPv6Prefixes []string
}

type RDAPQueryInput struct {
    Type    string // domain / ip / asn
    Target  string
    Timeout int
}
type RDAPQueryOutput struct {
    Type, Target string
    Result       interface{}
    Server       string
}

type AvailabilityInput  struct { Domain string }
type AvailabilityOutput struct {
    Domain              string
    Available           bool
    Status, Message     string
}

type WhoisCompareInput  struct { Domain1, Domain2 string }
type WhoisCompareOutput struct {
    Domain1, Domain2 string
    Changes          []*whois.WhoisChange
    Count            int
}

type QualityInput  struct { Domain string }
type QualityOutput struct {
    Domain                          string
    TotalScore, Completeness        int
    Timeliness, Reliability         int
    Level                           string
}

type NormalizeInput struct {
    Domain string
    Action string // normalize / to_punycode / to_unicode
}
type NormalizeOutput struct {
    Original, Result string
    IsIDN            bool
    Action           string
}
```

::: details ASN source 映射
```go
source := whois.ASNSourceAll
switch input.Source {
case "radb": source = whois.ASNSourceRADB
case "rdap": source = whois.ASNSourceRDAP
}
```
空值与其他值均落到 `ASNSourceAll`。
:::

---

## 🚀 使用示例

### 任务管理流程

```go
ctrl := mcp.NewController()

// 1. 规划
out, _ := ctrl.PlanRequest(mcp.RequestPlanningInput{
    OriginalRequest: "调研 example.com 归属",
    Tasks: []mcp.TaskInput{
        {Title: "查 WHOIS", Description: "查询域名注册信息"},
        {Title: "查 RDAP", Description: "查询 RDAP 数据"},
    },
})
requestID := out.RequestID

// 2. 取任务 -> 标记完成 -> 批准
next, _ := ctrl.GetNextTask(mcp.GetNextTaskInput{RequestID: requestID})
ctrl.MarkTaskDone(mcp.MarkTaskDoneInput{
    RequestID: requestID, TaskID: next.Task.ID, CompletedDetails: "OK",
})
ctrl.ApproveTaskCompletion(mcp.ApproveTaskInput{RequestID: requestID, TaskID: next.Task.ID})

// 3. 批准请求
ctrl.ApproveRequestCompletion(mcp.ApproveRequestInput{RequestID: requestID})
```

### WHOIS 查询

```go
info, _ := ctrl.ExecuteWhoisQuery("example.com")

full, _ := ctrl.ExecuteWhoisQueryFull(ctx, mcp.WhoisQueryInput{
    Domain:         "example.com",
    Timeout:        15,
    ValidateResult: true,
    RequiredFields: []string{"Registrar"},
})

asn, _ := ctrl.ExecuteASNQuery(ctx, mcp.ASNQueryInput{ASN: 13335, Source: "rdap"})
rdap, _ := ctrl.ExecuteRDAPQuery(ctx, mcp.RDAPQueryInput{Type: "domain", Target: "example.com"})
avail, _ := ctrl.CheckAvailability(ctx, mcp.AvailabilityInput{Domain: "example.com"})
cmp, _ := ctrl.CompareWhoisInfo(ctx, mcp.WhoisCompareInput{Domain1: "a.com", Domain2: "b.com"})
q, _ := ctrl.AssessWhoisQuality(ctx, mcp.QualityInput{Domain: "example.com"})
n, _ := ctrl.NormalizeDomainName(mcp.NormalizeInput{Domain: "münchen.de", Action: "to_punycode"})
```

---

## 🔗 相关

- 🗂️ [数据模型 models.go](./models.md)
- 🌐 [服务端 server.go](./server.md)
- 🧭 [MCP 概览](./overview.md)
