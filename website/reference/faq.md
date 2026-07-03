# ❓ FAQ — 常见问题

> 📖 汇总 whois-skills 使用过程中的高频问题与解答。

---

## 📖 目录

- [查询超时怎么办？](#查询超时怎么办)
- [如何规避被 WHOIS 服务器封禁？](#如何规避被-whois-服务器封禁)
- [缓存不生效怎么办？](#缓存不生效怎么办)
- [IDN 域名查不到结果？](#idn-域名查不到结果)
- [IP WHOIS 解析结果为空？](#ip-whois-解析结果为空)
- [批量查询中断如何恢复？](#批量查询中断如何恢复)
- [MCP 与 HTTP API 有什么区别？](#mcp-与-http-api-有什么区别)
- [如何对接反向 WHOIS？](#如何对接反向-whois)
- [apikeys.json 权限问题？](#apikeys-json-权限问题)
- [Docker 健康检查失败？](#docker-健康检查失败)

---

## 查询超时怎么办？

**现象**：`context deadline exceeded` 或查询返回超时错误。

**方案**：

1. 增大 `Timeout`（秒，默认 10）与 `MaxRetries`（默认 5）：

   ```go
   &whois.QueryOptions{
       Domain:     "example.com",
       Timeout:    30,
       MaxRetries: 8,
   }
   ```

2. 启用代理绕过网络限制：`UseProxy: true` + 配置代理池（`config/proxies.json`，`--proxy`）。
3. 检查目标 TLD 服务器是否可达，必要时更新 `config/servers.json`。

---

## 如何规避被 WHOIS 服务器封禁？

**方案**：

1. **代理池**：`--proxy` + `LoadProxiesFromFile`，`ProxyPool` 自动轮转。
2. **限速**：[ratelimit.go](../api/whois/ratelimit.md) 的 `RateLimiter`（令牌桶，按 server 维度）。
3. **智能调度退避**：[scheduler.go](../api/whois/scheduler.md) 的 `SmartScheduler`，遇 `RateLimit` 自动增大间隔（`increaseInterval`），健康后逐步恢复。
4. **重试间隔**：`QueryOptions.IntervalMils`（默认 1000ms）。

---

## 缓存不生效怎么办？

**检查清单**：

1. `EnableCache` 是否为 `true`（flag `--cache` 默认 true，`api.Server.EnableCache` 需显式设置）。
2. `CacheConfig.TTL` 是否过小（默认 3600s）。
3. 缓存类型 `local`/`redis` 是否正确，Redis 需 `RedisConfig` 可达。
4. 查询是否带不同参数导致 cache key 不同（当前按 domain 为 key）。
5. 是否调用了 `ClearExpired` 过早清理。

参考 [cache.go](../api/whois/cache.md)。

---

## IDN 域名查不到结果？

**现象**：中文等国际化域名查询失败或返回空。

**方案**：先调用 `NormalizeDomain` 做 Punycode 规范化，再查询：

```go
normalized, err := whois.NormalizeDomain("例子.中国")
if err != nil { ... }
whois.ExecuteQueryWithResult(&whois.QueryOptions{Domain: normalized})
```

参考 [idn.go](../api/whois/idn.md)，HTTP 端点 `/api/idn`。

---

## IP WHOIS 解析结果为空？

**原因**：部分 RIR（如 AFRINIC）数据不全，或响应格式非标准。

**方案**：

1. 改用 RDAP：`QueryRDAP_IPWithContext`（端点 `/api/rdap/ip`），RDAP 返回结构化数据更可靠。
2. 检查 `ipparser.go` 是否识别到正确 RIR（`detectRIR`）。
3. 查看 `IPWhoisResult.RawResponse` 原始文本确认是否真的无数据。

参考 [ipwhois.go](../api/whois/ipwhois.md)、[rdap.go](../api/whois/rdap.md)。

---

## 批量查询中断如何恢复？

**方案**：使用断点续传 `ResumeFromCheckpoint`：

```go
processor, err := whois.ResumeFromCheckpoint(ctx, whois.DefaultStreamBatchConfig())
processor.Process(ctx, allDomains) // 仅查询未完成的
```

断点文件由 `StreamBatchProcessor` 自动保存（`CheckpointDir`）。也可手动 `LoadCheckpointFromFile`。

参考 [batch.go](../api/whois/batch.md)。

---

## MCP 与 HTTP API 有什么区别？

| 维度 | HTTP API（`pkg/api`） | MCP（`pkg/mcp`） |
|------|----------------------|------------------|
| 模型 | 无状态请求-响应 | 有状态 Request/Task 状态机 |
| 端点 | `/api/whois` 等功能端点 | `/api/mcp/*` 任务编排端点 |
| 适用 | 一次性查询 | 多步骤任务规划、追踪、审批 |
| 状态 | 不持久化 | 内存存储（重启丢失） |
| 查询能力 | 直接调 whois | Controller 封装 whois |

参考 [api 模块](../modules/api.md)、[mcp 模块](../modules/mcp.md)。

---

## 如何对接反向 WHOIS？

**方案**：实现 `ReverseWhoisProvider` 接口：

```go
type ReverseWhoisProvider interface {
    SearchByRegistrant(ctx, query, opts) ([]*ReverseWhoisResult, error)
    SearchByEmail(ctx, email, opts) ([]*ReverseWhoisResult, error)
    SearchByOrganization(ctx, org, opts) ([]*ReverseWhoisResult, error)
    Name() string
}
```

然后：

```go
client := whois.NewReverseWhoisClient(myProvider)
results, _ := client.SearchByEmail(ctx, "admin@example.com", nil)
```

参考 [reverse.go](../api/whois/reverse.md)。需自行接入第三方反向 WHOIS 数据源。

---

## apikeys.json 权限问题？

**现象**：`SaveConfig` 写入失败或启动告警权限。

**说明**：`APIKeyManager.SaveConfig` 以 **0600** 权限写入 `config/apikeys.json`，仅属主可读写。

**方案**：

1. 确保运行用户为文件属主：`chown <user>:<group> config/apikeys.json`。
2. Docker 中以 `appuser` 运行，需在镜像构建时 `chown`。
3. 注意：security 模块当前**尚未接入主服务**（api 用占位中间件），见 [security 模块](../modules/security.md)。

---

## Docker 健康检查失败？

**原因**：

1. `docker-compose.yml` 健康检查引用 `/app/bin/whois-hacker version`，但 Dockerfile 复制到 `/app/whois-hacker`，**路径不一致**导致失败。
2. 端口未映射或服务未启动。

**方案**：改用 curl 健康检查：

```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/api/health"]
```

详见 [Docker Compose](../deploy/compose.md) 与 [故障排查](./troubleshooting.md)。

---

## 🔗 相关链接

- [故障排查](./troubleshooting.md)
- [模块总览](../modules/overview.md)
- [快速开始](../guide/getting-started.md)
