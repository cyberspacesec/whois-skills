# 🛠️ 故障排查

> 📖 按错误现象分类的排查指南，含错误码表与解决方案。

---

## 📖 目录

- [连接超时 / 连接重置](#连接超时--连接重置)
- ["queried interval is too short" / "access is too fast"](#queried-interval-is-too-short--access-is-too-fast)
- [域名解析失败](#域名解析失败)
- [HTTP 405 Method Not Allowed](#http-405-method-not-allowed)
- [HTTP 400 Bad Request](#http-400-bad-request)
- [HTTP 500 Internal Server Error](#http-500-internal-server-error)
- [HTTP 503 Service Unavailable（metrics/alerts）](#http-503-service-unavailable-metrics-alerts)
- [Docker 启动失败](#docker-启动失败)
- [编译失败](#编译失败)

---

## 错误码速查表

| HTTP 码 | 触发场景 | 关键排查点 |
|---------|----------|------------|
| 400 | 请求体非 JSON / 必填字段缺失 | 检查 body 与字段 |
| 405 | 非 GET/POST | 确认端点要求的 method |
| 500 | 查询失败、JSON 编码失败 | 看日志、检查上游 |
| 503 | metrics/alerts 功能未启用 | 开启 `EnableMetrics`/`EnableAlerts` |

---

## 连接超时 / 连接重置

**现象**：`dial tcp ... i/o timeout`、`connection reset by peer`。

**排查**：

1. 网络可达性：`ping whois.verisign-grs.com`、`telnet whois.verisign-grs.com 43`。
2. 启用代理：`--proxy --proxy-file config/proxies.json`，`UseProxy: true`。
3. 增大 `Timeout` 与 `MaxRetries`。
4. 检查 `config/servers.json` 中 TLD→服务器映射是否正确。

参考 [proxy.go](../api/whois/proxy.md)、[errors.go](../api/whois/errors.md)。

---

## "queried interval is too short" / "access is too fast"

**现象**：WHOIS 服务器返回限速错误文本。

**排查**：被目标服务器限速。

**解决方案**：

1. 启用 [RateLimiter](../api/whois/ratelimit.md)（令牌桶，按 server 维度限速）。
2. 启用 [SmartScheduler](../api/whois/scheduler.md)：遇限速自动 `increaseInterval` 退避，恢复后 `decreaseInterval`。
3. 增大 `QueryOptions.IntervalMils`（重试间隔）。
4. 切换代理 IP。

`isRateLimitError`（scheduler.go）与 `CheckError`（errors.go）会识别此类错误并标记可重试。

---

## 域名解析失败

**现象**：`failed to get whois server`、找不到 TLD 服务器。

**排查**：

1. IDN 域名先 `NormalizeDomain` 转 Punycode。
2. 检查 `ExtractEffectiveTLD` 是否正确提取多级 TLD（如 `.co.uk`）。
3. `config/servers.json` 是否包含该 TLD，必要时 `UpdateServer(tld, server)` 或 `loadDefaultServers`。
4. 服务器健康检查：`WhoisServerManager` 会标记不健康服务器并跳过。

参考 [servers.go](../api/whois/servers.md)、[idn.go](../api/whois/idn.md)。

---

## HTTP 405 Method Not Allowed

**现象**：`仅支持POST请求` / `仅支持GET请求`。

**排查**：端点对 method 有严格要求：

| 端点类别 | 要求 method |
|----------|------------|
| `/api/whois`、`/api/ip`、`/api/asn`、`/api/rdap/*`、`/api/availability`、`/api/diff`、`/api/quality`、`/api/correlation`、`/api/batch`、`/api/format`、`/api/export/*`、`/api/idn` | **POST** |
| `/api/batch/status`、`/api/servers`、`/api/metrics`、`/api/alerts`、`/api/health` | **GET** |

参考 [api 模块端点表](../modules/api.md)。

---

## HTTP 400 Bad Request

**现象**：`无效的请求格式` / `域名不能为空`。

**排查**：

1. 请求头含 `Content-Type: application/json`。
2. body 为合法 JSON。
3. 必填字段非空：
   - `/api/whois` → `domain`
   - `/api/ip` → `ip`
   - `/api/asn` → `asn`（正整数）
   - `/api/diff` → `domain1` + `domain2`
   - `/api/correlation` → `domains`（≥2 个）
   - `/api/batch` → `domains`（非空数组）

---

## HTTP 500 Internal Server Error

**现象**：`查询失败: ...` / `导出失败: ...`。

**排查**：

1. 查看服务日志（logrus 输出）获取上游错误。
2. 上游 WHOIS/RDAP 查询失败 → 参考[连接超时](#连接超时--连接重置)与[限速](#queried-interval-is-too-short--access-is-too-fast)。
3. `whoisparser.Parse` 解析失败 → 原始响应可能非标准，用 `/api/format` 检测格式。
4. 导出失败 → 检查 `io.Writer` 与 `WhoisInfo` 是否为 nil。

---

## HTTP 503 Service Unavailable（metrics/alerts）

**现象**：`监控功能未启用` / `告警功能未启用`。

**原因**：`api.Server.EnableMetrics` 或 `EnableAlerts` 为 false。

**解决**：

- 启动时加 `--metrics --alerts`（默认即 true）。
- 确认 `cmd` 中 `setupMetrics()`/`setupAlerts()` 执行成功（看日志「监控功能已启用」「告警功能已启用」）。
- 自定义 http.Server 时显式 `apiServer.EnableMetrics = true`。

参考 [metrics 模块](../modules/metrics.md)。

---

## Docker 启动失败

**现象**：容器立即退出或反复重启。

**排查**：

1. **路径不一致**：`docker-compose.yml` 的 `command`/`healthcheck` 引用 `/app/bin/whois-hacker`，但 Dockerfile 复制到 `/app/whois-hacker`。日志会报 `no such file or directory`。修复见 [compose 文档](../deploy/compose.md)。
2. **端口占用**：`8080` 被占用 → `docker run` 改映射端口，或 `lsof -i:8080` 排查。
3. **serve 子命令**：`CMD` 含 `serve`，flag.Parse 在此处停止解析，后续参数被忽略。建议去掉 `serve`。
4. **权限**：挂载卷属主非 `appuser` → `chown` 数据目录。

```bash
docker logs whois-hacker
docker-compose logs
```

---

## 编译失败

**现象**：`go build` 报错。

**排查**：

1. **Go 版本**：需 1.23+（`go.mod` 声明）。`go version` 确认。
2. **依赖**：`go mod tidy && go mod verify`，必要时 `go mod download`。
3. **gopsutil**：`pkg/metrics` 依赖 `github.com/shirou/gopsutil/v3`，CGO 可禁用（`CGO_ENABLED=0`）。
4. **缓存**：`go clean -cache` 后重试。
5. **交叉编译**：`make build-all` 各平台需对应 GOOS/GOARCH。

---

## 🔗 相关链接

- [FAQ](./faq.md)
- [Docker Compose](../deploy/compose.md)
- [errors.go](../api/whois/errors.md) — 错误分类
