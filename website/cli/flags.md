# 🚩 命令行参数

> 📋 `whois-hacker` 的全部 18 个命令行 flag，按子系统分组详解。每个 flag 都标注了类型、默认值与示例。

::: tip 🤖 给 AI 速查
本页是机器可读友好的参数参考。所有 flag 均为 Go 标准 `flag` 包风格：`--name value` 或 `-name value`，布尔 flag 可用 `--name`（开）或 `--name=false`（关）。
:::

---

## 📊 全量参数速查表

| flag | 类型 | 默认值 | 子系统 | 一句话 |
|------|------|--------|--------|--------|
| `--config` | string | `config/config.yaml` | 通用 | 配置文件路径 |
| `--host` | string | `127.0.0.1` | HTTP | 监听地址 |
| `--port` | int | `8080` | HTTP | 监听端口 |
| `--log-level` | string | `info` | 日志 | 日志级别 |
| `--log-format` | string | `text` | 日志 | 日志格式 |
| `--cache` | bool | `true` | 缓存 | 启用缓存 |
| `--cache-type` | string | `local` | 缓存 | 缓存类型 |
| `--cache-ttl` | int64 | `3600` | 缓存 | 缓存有效期（秒） |
| `--cache-warmup` | bool | `false` | 缓存 | 启用预热 |
| `--warmup-file` | string | `config/warmup.json` | 缓存 | 预热域名列表 |
| `--warmup-interval` | int64 | `1000` | 缓存 | 预热间隔（毫秒） |
| `--proxy` | bool | `false` | 代理 | 启用代理 |
| `--proxy-file` | string | `config/proxies.json` | 代理 | 代理列表文件 |
| `--metrics` | bool | `true` | 监控 | 启用监控 |
| `--metrics-interval` | int64 | `60` | 监控 | 采集间隔（秒） |
| `--alerts` | bool | `true` | 告警 | 启用告警 |
| `--alerts-interval` | int64 | `60` | 告警 | 检查间隔（秒） |

`--help` 可查看程序自动生成的参数列表：

```bash
./bin/whois-hacker --help
```

```mermaid
flowchart LR
    subgraph 通用["🔧 通用"]
        Config["--config"]
    end
    subgraph HTTP["🌐 HTTP 服务"]
        Host["--host"]
        Port["--port"]
    end
    subgraph Log["📝 日志"]
        LL["--log-level"]
        LF["--log-format"]
    end
    subgraph Cache["💾 缓存"]
        C1["--cache"]
        C2["--cache-type"]
        C3["--cache-ttl"]
        C4["--cache-warmup"]
        C5["--warmup-file"]
        C6["--warmup-interval"]
    end
    subgraph Proxy["🔒 代理"]
        P1["--proxy"]
        P2["--proxy-file"]
    end
    subgraph Obs["📈 监控/告警"]
        M1["--metrics"]
        M2["--metrics-interval"]
        A1["--alerts"]
        A2["--alerts-interval"]
    end

    classDef g fill:#909399,color:#fff,stroke:#6b6e72
    classDef h fill:#41b883,color:#fff,stroke:#2b7a4b
    classDef l fill:#647eff,color:#fff,stroke:#4a5fd6
    classDef c fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef p fill:#f56c6c,color:#fff,stroke:#c04040
    classDef o fill:#e6a23c,color:#fff,stroke:#b7821c
    class Config g
    class Host,Port h
    class LL,LF l
    class C1,C2,C3,C4,C5,C6 c
    class P1,P2 p
    class M1,M2,A1,A2 o
```

---

## 🔧 通用

### `--config`

- **类型**：`string`
- **默认**：`config/config.yaml`
- **作用**：YAML 配置文件路径。文件不存在时静默回退到 flag 默认值（仅 debug 日志提示）。
- **优先级**：命令行显式 flag > 配置文件 > flag 默认值（详见 [配置文件](./config-file.md)）。

```bash
./bin/whois-hacker --config /etc/whois/config.yaml
```

---

## 🌐 HTTP 服务

### `--host`

- **类型**：`string`
- **默认**：`127.0.0.1`
- **作用**：HTTP 服务监听地址。
  - `127.0.0.1`：仅本机可访问（默认，安全）
  - `0.0.0.0`：所有网卡可访问（容器/对外服务时用）

```bash
./bin/whois-hacker --host 0.0.0.0
```

### `--port`

- **类型**：`int`
- **默认**：`8080`
- **作用**：HTTP 服务监听端口。

```bash
./bin/whois-hacker --port 9090
```

---

## 📝 日志

### `--log-level`

- **类型**：`string`
- **默认**：`info`
- **可选**：`debug` / `info` / `warn` / `error`
- **作用**：日志级别。非法值回退到 `info`。详见 [日志与输出](./logging.md)。

```bash
./bin/whois-hacker --log-level debug
```

### `--log-format`

- **类型**：`string`
- **默认**：`text`
- **可选**：`text` / `json`
- **作用**：日志格式。生产环境建议 `json`，便于日志采集系统解析。

```bash
./bin/whois-hacker --log-format json
```

---

## 💾 缓存

### `--cache`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用查询结果缓存。命中缓存可显著降低延迟与对外请求量。

```bash
# 关闭缓存
./bin/whois-hacker --cache=false
```

### `--cache-type`

- **类型**：`string`
- **默认**：`local`
- **可选**：`local` / `redis`
- **作用**：缓存后端。`local` 为进程内内存缓存；`redis` 连接 `localhost:6379`（地址当前在源码中固定，详见 [FAQ](./faq.md)）。

```bash
./bin/whois-hacker --cache-type redis
```

### `--cache-ttl`

- **类型**：`int64`
- **默认**：`3600`
- **单位**：秒
- **作用**：缓存条目有效期。后台每 5 分钟清理一次过期条目。

```bash
./bin/whois-hacker --cache-ttl 7200
```

### `--cache-warmup`

- **类型**：`bool`
- **默认**：`false`
- **作用**：启动时是否预热缓存（按 `--warmup-file` 列表提前查询并缓存）。

```bash
./bin/whois-hacker --cache-warmup
```

### `--warmup-file`

- **类型**：`string`
- **默认**：`config/warmup.json`
- **作用**：预热用的域名列表文件（JSON 数组）。

```json
["example.com", "google.com", "github.com"]
```

### `--warmup-interval`

- **类型**：`int64`
- **默认**：`1000`
- **单位**：毫秒
- **作用**：预热时每两次查询之间的间隔，避免触发限速。

```bash
./bin/whois-hacker --cache-warmup --warmup-interval 500
```

---

## 🔒 代理

### `--proxy`

- **类型**：`bool`
- **默认**：`false`
- **作用**：是否启用代理池。启用后查询走 `--proxy-file` 中的代理轮询，规避 IP 封禁。详见 [proxy.go](../api/whois/proxy.md)。

```bash
./bin/whois-hacker --proxy
```

### `--proxy-file`

- **类型**：`string`
- **默认**：`config/proxies.json`
- **作用**：代理列表文件（JSON 数组，支持 SOCKS5/HTTP）。

```json
[
  {"type":"socks5","addr":"127.0.0.1:1080"},
  {"type":"http","addr":"http://user:pass@proxy.example.com:8080"}
]
```

---

## 📈 监控

### `--metrics`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用系统指标采集（CPU/内存/查询统计等）。每分钟导出到 `data/metrics.json`，关闭时导出 `data/metrics_final.json`。

```bash
./bin/whois-hacker --metrics=false
```

### `--metrics-interval`

- **类型**：`int64`
- **默认**：`60`
- **单位**：秒
- **作用**：系统指标采集间隔。

```bash
./bin/whois-hacker --metrics-interval 30
```

---

## 🚨 告警

### `--alerts`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用告警管理器（注册默认通知器并周期检查告警规则）。

```bash
./bin/whois-hacker --alerts=false
```

### `--alerts-interval`

- **类型**：`int64`
- **默认**：`60`
- **单位**：秒
- **作用**：告警规则检查间隔。

```bash
./bin/whois-hacker --alerts-interval 120
```

---

## 🎚️ flag 优先级机制

```mermaid
flowchart LR
    Explicit["🚩 命令行显式 flag<br/>flag.Visit 检测到"]
    YAML["📄 config.yaml 值"]
    Default["🔕 flag 默认值"]

    Explicit -->|"优先级最高"| Final{"最终生效值"}
    YAML -->|"仅当 flag 未显式设置时覆盖"| Final
    Default -->|"兜底"| Final

    classDef hi fill:#f56c6c,color:#fff,stroke:#c04040
    classDef mid fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef lo fill:#909399,color:#fff,stroke:#6b6e72
    classDef out fill:#41b883,color:#fff,stroke:#2b7a4b
    class Explicit hi
    class YAML mid
    class Default lo
    class Final out
```

实现要点（来自 `main.go` 的 `loadConfigFromFile`）：

1. `flag.Parse()` 先解析命令行
2. 加载 YAML 配置文件
3. 用 `flag.Visit`（只遍历**显式设置**的 flag）记录哪些参数是命令行传入的
4. 对每个配置项：**仅当命令行未显式设置时**，才用 YAML 的值覆盖 flag 默认值

::: tip 💡 验证优先级
若同时 `--port 9090` 且 `config.yaml` 中 `server.port: 8080`，最终监听 **9090**（命令行优先）。
:::

::: warning ⚠️ 布尔 flag 的优先级
布尔 flag（如 `--cache`）的"是否显式设置"由 `flag.Visit` 判断。`--cache=false` 也算显式设置，会覆盖 YAML 中的 `cache.enabled: true`。
:::

---

## 🧪 组合示例

```bash
# 生产环境：对外服务 + JSON 日志 + Redis 缓存 + 代理池
./bin/whois-hacker \
  --host 0.0.0.0 --port 8080 \
  --log-level info --log-format json \
  --cache-type redis --cache-ttl 7200 \
  --proxy --proxy-file config/proxies.json

# 调试环境：详细日志 + 关闭缓存看实时查询
./bin/whois-hacker \
  --log-level debug \
  --cache=false

# 轻量环境：关闭监控与告警，降低开销
./bin/whois-hacker \
  --metrics=false --alerts=false
```

---

## 🔗 相关文档

- ⚙️ [配置文件](./config-file.md) — YAML 字段与 flag 的对应关系
- 📝 [日志与输出](./logging.md) — 日志级别与格式详解
- 🚀 [启动与运行](./usage.md) — 完整启动流程
