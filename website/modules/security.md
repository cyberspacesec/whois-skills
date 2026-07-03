# 🔐 security 模块 — 认证鉴权

> 📖 `pkg/security` 提供 API Key 管理（生成/验证/权限/速率限制）、认证中间件与请求日志。注意：当前 `pkg/api` 主服务使用的是占位 `AuthMiddleware`，本模块的真实认证尚未接入。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `pkg/security` |
| 源文件数 | 3 + `config/` |
| 职责 | API Key 全生命周期、认证中间件、速率限制、请求日志 |
| 依赖 | logrus |

---

## 📁 文件清单

| 文件 | 职责 |
|------|------|
| `apikey.go` | `APIKey` 结构与 `APIKeyManager` 单例 |
| `middleware.go` | `AuthMiddleware(requiredPermission)` 工厂、`RequestLogger`、速率限制 |
| `response.go` | `APIResponse` 与响应函数 |
| `config/apikeys.json` | API Key 持久化文件（权限 0600） |

---

## 🔑 APIKey 结构

```go
type APIKey struct {
    ID          string    `json:"id"`
    Key         string    `json:"key"`
    Description string    `json:"description"`
    Permissions []string  `json:"permissions"`   // 权限列表，含 "admin" 则全权
    RateLimit   int       `json:"rate_limit"`     // 每分钟请求上限
    CreatedAt   time.Time `json:"created_at"`
    ExpiresAt   time.Time `json:"expires_at,omitempty"`
    Enabled     bool      `json:"enabled"`
}
```

---

## 🧠 APIKeyManager 单例

| 方法 | 说明 |
|------|------|
| `GetAPIKeyManager() *APIKeyManager` | 获取单例 |
| `InitAPIKeys(configFile) error` | 初始化并加载配置 |
| `GenerateAPIKey(description, permissions, rateLimit)` | 生成新 Key |
| `ValidateKey(apiKey, requiredPermission) (*APIKey, error)` | 校验 Key 与权限 |
| `LoadConfig(path) error` | 从 JSON 加载 |
| `SaveConfig() error` | 保存到 `config/apikeys.json`（权限 **0600**） |
| `GetAPIKey` / `ListAPIKeys` | 查询 |
| `EnableAPIKey` / `DisableAPIKey` / `DeleteAPIKey` | 启用/禁用/删除 |
| `SetKeyExpiration` / `UpdateKeyPermissions` / `UpdateKeyRateLimit` | 更新属性 |

权限校验：`ValidateKey` 遍历 `Permissions`，命中 `requiredPermission` 或 `"admin"` 即通过。

---

## 🛡️ AuthMiddleware 工厂

```go
func AuthMiddleware(requiredPermission string) func(http.Handler) http.Handler
```

处理流程：

1. 读取请求头 `X-API-Key`
2. 缺失 → 记日志并拒绝
3. `ValidateKey(apiKey, requiredPermission)` 校验
4. `checkRateLimit(key, r)` 速率限制（按 `APIKeyID:ClientIP` 维度）
5. 通过则放行，记录请求日志

### RequestLogger

| 函数 | 说明 |
|------|------|
| `GetRequestLogger() *RequestLogger` | 单例（默认保留最近 1000 条） |
| `NewRequestLogger(maxLogs)` | 自定义容量 |
| `AddLog(log)` / `GetRecentLogs()` | 记录/查询 |

---

## 📄 config/apikeys.json

```json
[
  {
    "id": "test-id",
    "key": "test-key",
    "permissions": null,
    "rate_limit": 120,
    "created_at": "0001-01-01T00:00:00Z"
  }
]
```

::: warning 🔒 文件权限
`SaveConfig` 以 `0600` 权限写入，仅属主可读写。部署时需确保运行用户为文件属主。
:::

---

## 🚀 使用示例

```go
// 初始化
security.InitAPIKeys("config/apikeys.json")

// 生成 Key
key, _ := security.GetAPIKeyManager().GenerateAPIKey(
    "生产环境", []string{"whois:read", "batch:read"}, 60,
)

// 接入路由
router.Handle("/api/whois",
    security.AuthMiddleware("whois:read")(whoisHandler),
)
```

请求示例：

```bash
curl -H "X-API-Key: <your-key>" http://localhost:8080/api/whois
```

---

## ⚠️ 注意事项

::: warning ⚠️ 尚未接入主服务
`pkg/api` 的 `middleware.go` 中 `AuthMiddleware` 是**占位实现**（`// TODO: 实现认证逻辑`），直接放行所有请求。如需启用真实鉴权，需将 `api.Server.addMiddleware` 中的占位 `AuthMiddleware(next)` 替换为 `security.AuthMiddleware("required")(next)`，并确保 `api` 与 `security` 的 `response.go` 不冲突（两者均定义了 `SendSuccessResponse`/`SendErrorResponse`）。
:::

- 速率限制为内存令牌桶，单机维度，重启即重置。
- 默认配置文件内置 `test-key`，**生产环境务必删除或更换**。

---

## 🔗 相关链接

- [api 模块](./api.md) — 主 HTTP 服务（占位认证）
- [cmd 模块](./cmd.md) — 入口配置
- [模块总览](./overview.md)
