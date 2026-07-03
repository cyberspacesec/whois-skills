# 🛡️ middleware.go — 中间件

> 📖 HTTP API 的中间件链实现，提供认证、CORS、日志与 panic 恢复四个中间件，按 Recovery → Logging → CORS → Auth 的顺序包裹业务处理器。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/api/middleware.go` |
| 中间件数量 | 4 个 |
| 包裹工具 | 未导出 `responseWriter` 类型捕获状态码 |

---

## 🔧 四个中间件

### AuthMiddleware

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // TODO: 实现认证逻辑
        next.ServeHTTP(w, r)
    })
}
```

::: warning 占位实现
当前为占位实现，含 `TODO: 实现认证逻辑`，**直接放行**，未实际校验请求。生产环境需自行补充鉴权逻辑（如校验 `Authorization` 头）。
:::

| 项目 | 说明 |
|------|------|
| 位置 | 中间件链最内层 |
| 行为 | 直接调用 `next.ServeHTTP` |
| TODO | 实现 API Key / Token 校验 |

---

### CORSMiddleware

```go
func CORSMiddleware(next http.Handler) http.Handler
```

设置跨域响应头，并处理 OPTIONS 预检请求：

| 响应头 | 取值 |
|--------|------|
| `Access-Control-Allow-Origin` | `*` |
| `Access-Control-Allow-Methods` | `GET, POST, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, Authorization` |

| 行为 | 说明 |
|------|------|
| `OPTIONS` 请求 | 直接返回 `200 OK`，不进入业务处理器 |
| 其他请求 | 设置头后调用 `next.ServeHTTP` |

---

### LoggingMiddleware

```go
func LoggingMiddleware(next http.Handler) http.Handler
```

记录每个请求的访问日志。

| 处理步骤 | 说明 |
|------|------|
| 记录开始时间 | `start := time.Now()` |
| 包装 ResponseWriter | `rw := &responseWriter{ResponseWriter: w}` 捕获状态码 |
| 调用业务处理器 | `next.ServeHTTP(rw, r)` |
| 计算耗时 | `duration := time.Since(start)` |
| 输出日志 | logrus 字段日志，消息 `HTTP请求` |

记录字段：

| 字段 | 来源 |
|------|------|
| `method` | `r.Method` |
| `path` | `r.URL.Path` |
| `status` | `rw.statusCode` |
| `duration` | 请求耗时 |
| `client_ip` | `r.RemoteAddr` |
| `user_agent` | `r.UserAgent()` |

---

### RecoveryMiddleware

```go
func RecoveryMiddleware(next http.Handler) http.Handler
```

位于中间件链最外层，捕获业务处理器中的 panic。

| 处理步骤 | 说明 |
|------|------|
| `defer recover()` | 捕获 panic |
| 记录错误 | `logrus.Errorf("请求处理panic: %v", err)` |
| 返回错误响应 | `SendErrorResponse(w, 500, "服务器内部错误")` |

::: tip 作用
确保单个请求的 panic 不会使整个服务进程崩溃，并向客户端返回 `500 Internal Server Error`。
:::

---

## 📦 responseWriter 类型

未导出的包装类型，用于捕获响应状态码：

```go
type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    if rw.statusCode == 0 {
        rw.statusCode = http.StatusOK  // 默认 200
    }
    return rw.ResponseWriter.Write(b)
}
```

| 方法 | 说明 |
|------|------|
| `WriteHeader(code)` | 记录状态码后转发 |
| `Write(b)` | 若未显式设置状态码，默认置为 200 |

---

## 🚀 使用示例

### 直接组合使用

```go
handler := RecoveryMiddleware(
    LoggingMiddleware(
        CORSMiddleware(
            AuthMiddleware(myHandler))))
```

### 通过 Server 装配

```go
s := api.NewServer("127.0.0.1", 8080)
// 内部已按 Recovery → Logging → CORS → Auth 顺序自动装配
// 自定义中间件通过 AddMiddleware 添加，包在最外层
s.AddMiddleware(myCustomMiddleware)
s.Start()
```

---

## 🔗 相关

- 🖥️ [server.md](./server.md) — 服务器结构与中间件装配
- 📦 [response.md](./response.md) — `SendErrorResponse` 实现
- 🌐 [overview.md](./overview.md) — API 概览
