# 🚀 cmd 模块 — 程序入口

> 📖 `cmd/whois-hacker` 是程序入口，定义 17 个命令行 flag，按「命令行 > YAML > 默认」优先级加载配置，完成缓存/代理/监控/告警初始化后启动 HTTP 服务，支持信号优雅关闭与指标导出。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `cmd/whois-hacker` |
| 源文件数 | 2（`main.go` + `main_test.go`） |
| 职责 | flag 解析、配置加载、组件初始化、HTTP 启停、优雅关闭 |
| 依赖 | `pkg/api`、`pkg/metrics`、`pkg/whois`、logrus |

---

## 📁 文件清单

| 文件 | 职责 |
|------|------|
| `main.go` | 全部入口逻辑（flag、init、main、setup 系列、loadConfigFromFile） |
| `main_test.go` | 入口测试 |

---

## 🏷️ 版本注入

Dockerfile / Makefile 通过 `-ldflags` 注入：

| 变量 | 说明 |
|------|------|
| `main.Version` | 版本号（来自 `VERSION` 文件或 Makefile `VERSION`） |
| `main.BuildTime` | 构建时间（UTC） |
| `main.GitCommit` | Git 短 commit hash |

---

## 🚩 17 个命令行 flag

| flag | 默认值 | 说明 |
|------|--------|------|
| `--config` | `config/config.yaml` | 配置文件路径 |
| `--host` | `127.0.0.1` | HTTP 监听地址 |
| `--port` | `8080` | HTTP 监听端口 |
| `--log-level` | `info` | 日志级别 |
| `--log-format` | `text` | 日志格式（text/json） |
| `--cache` | `true` | 启用缓存 |
| `--cache-type` | `local` | 缓存类型（local/redis） |
| `--cache-ttl` | `3600` | 缓存有效期（秒） |
| `--cache-warmup` | `false` | 启用缓存预热 |
| `--warmup-file` | `config/warmup.json` | 预热域名列表文件 |
| `--warmup-interval` | `1000` | 预热间隔（毫秒） |
| `--proxy` | `false` | 启用代理 |
| `--proxy-file` | `config/proxies.json` | 代理列表文件 |
| `--metrics` | `true` | 启用监控 |
| `--metrics-interval` | `60` | 监控采集间隔（秒） |
| `--alerts` | `true` | 启用告警 |
| `--alerts-interval` | `60` | 告警检查间隔（秒） |

---

## ⚙️ 配置优先级

```
命令行显式参数  >  YAML 配置文件  >  flag 默认值
```

`loadConfigFromFile` 通过 `flag.Visit` 判断哪些 flag 被命令行显式设置，仅在**未显式设置**时才用 YAML 值覆盖默认值。

---

## 🔄 main 流程

```
flag.Parse()
   │
   ▼
loadConfigFromFile()        ── 读取 config/config.yaml（不存在则跳过）
   │
   ▼
setupLogging()              ── logrus 级别与格式
   │
   ▼
whois.GetServerManager().LoadFromFile("config/servers.json")
   │
   ▼
setupCache()      ── (enableCache)   NewWhoisCache + 定期清理
setupProxy()      ── (enableProxy)   LoadProxiesFromFile
setupMetrics()    ── (enableMetrics) GetCollector + StartSystemMetricsCollection + 定期导出
setupAlerts()     ── (enableAlerts)  GetAlertManager + RegisterDefaultNotifiers + StartAlertManager
   │
   ▼
api.NewServer(host, port)   ── 设置 EnableProxy/Cache/Metrics/Alerts
   │
   ▼
&http.Server{Handler: apiServer.CreateHandler()}
   │
   ▼
goroutine: httpServer.ListenAndServe()
   │
   ▼
signal.Notify(SIGINT, SIGTERM)  ── 阻塞等待
   │
   ▼
httpServer.Shutdown(5s)     ── 优雅关闭
   │
   ▼
collector.ExportMetrics("data/metrics_final.json")  ── (enableMetrics)
```

---

## 🧩 setup 系列函数

| 函数 | 作用 |
|------|------|
| `setupLogging()` | 解析 log-level/log-format，设置 logrus |
| `setupCache()` | 构造 `CacheConfig`（含 Redis/Warmup 可选），`NewWhoisCache`，启动过期清理 goroutine |
| `setupProxy()` | `whois.LoadProxiesFromFile(proxyFile)` |
| `setupMetrics()` | `GetCollector` + `StartSystemMetricsCollection` + 每分钟导出 `data/metrics.json` |
| `setupAlerts()` | `GetAlertManager` + `RegisterDefaultNotifiers` + `StartAlertManager` |
| `loadConfigFromFile()` | YAML 加载 + 优先级合并 |

---

## 🚀 使用示例

```bash
# 默认启动
./whois-hacker

# 自定义配置
./whois-hacker --host 0.0.0.0 --port 9000 --log-level debug --cache-ttl 7200

# 启用代理与告警
./whois-hacker --proxy --proxy-file config/proxies.json --alerts
```

---

## ⚠️ 注意事项

- `main` 不读取环境变量，所有配置走 flag 或 YAML；Docker Compose 中的 `HTTP_HOST` 等环境变量当前**不被 main 读取**，需用 flag 传递。
- 优雅关闭超时为 **5 秒**，超时未完成的请求会被强制中断。
- `config/servers.json` 加载失败仅告警，不阻断启动（使用内置默认服务器表）。

---

## 🔗 相关链接

- [Docker 部署](../deploy/docker.md)
- [二进制部署](../deploy/binary.md)
- [api 模块](./api.md)
- [模块总览](./overview.md)
