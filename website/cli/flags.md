# 🚩 命令行参数

> 📋 `whois-hacker` 基于 `cobra` 的子命令结构：一组**全局持久 flag** 被所有子命令继承，另有各子命令的**专属 flag**。本页按"全局 → serve → 各查询子命令"分组详解。

::: tip 🤖 给 AI 速查
本页是机器可读友好的参数参考。所有 flag 为 cobra 风格：`--name value` 或 `--name=value`，布尔 flag 可用 `--name`（开）或 `--name=false`（关）。全局 flag 可放在子命令**前或后**，例如 `whois-hacker --log-level debug whois x.com` 与 `whois-hacker whois x.com --log-level debug` 等价。
:::

::: tip 📋 命令树一览
```
whois-hacker
├── serve           # 启动 HTTP 服务
├── version         # 版本信息
├── whois <domain>  # 域名 WHOIS
├── ip <ip>         # IP WHOIS
├── asn <asn>       # ASN 查询
├── rdap            # RDAP 父命令（domain/ip/asn/entity）
├── availability    # 可注册性
├── diff            # 差异对比
├── quality         # 质量评分
├── correlation     # 关联分析
├── batch <file>    # 批量查询
├── idn             # IDN 转换
├── format          # 格式检测
├── export          # 导出
├── servers         # 服务器列表
└── completion      # shell 自动补全
```
:::

---

## 🌐 全局持久 flag（所有子命令继承）

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--config` | string | `config/config.yaml` | 配置文件路径 |
| `--log-level` | string | `info` | 日志级别（debug/info/warn/error） |
| `--log-format` | string | `text` | 日志格式（text/json） |
| `--timeout` | int | `10` | 查询超时秒数 |
| `--use-proxy` | bool | `false` | 是否使用代理 |
| `--proxy-file` | string | `config/proxies.json` | 代理列表文件 |
| `--format` | string | `json` | 输出格式（json/text/raw，仅查询类生效） |

`--help` 可查看对应命令的参数列表：

```bash
./bin/whois-hacker --help        # 根命令
./bin/whois-hacker serve --help  # serve 子命令
./bin/whois-hacker whois --help  # whois 子命令
```

```mermaid
flowchart LR
    subgraph Global["🌐 全局持久 flag（所有子命令继承）"]
        Config["--config"]
        LL["--log-level"]
        LF["--log-format"]
        TO["--timeout"]
        UP["--use-proxy"]
        PF["--proxy-file"]
        FMT["--format"]
    end

    classDef g fill:#909399,color:#fff,stroke:#6b6e72
    class Config,LL,LF,TO,UP,PF,FMT g
```

### `--config`

- **类型**：`string`
- **默认**：`config/config.yaml`
- **作用**：YAML 配置文件路径。文件不存在时静默回退到 flag 默认值（仅 debug 日志提示）。
- **优先级**：命令行显式 flag > 配置文件 > flag 默认值（详见 [配置文件](./config-file.md)）。

```bash
./bin/whois-hacker serve --config /etc/whois/config.yaml
```

### `--log-level`

- **类型**：`string`
- **默认**：`info`
- **可选**：`debug` / `info` / `warn` / `error`
- **作用**：日志级别。非法值回退到 `info`。详见 [日志与输出](./logging.md)。

```bash
./bin/whois-hacker whois example.com --log-level debug
```

### `--log-format`

- **类型**：`string`
- **默认**：`text`
- **可选**：`text` / `json`
- **作用**：日志格式。生产环境建议 `json`，便于日志采集系统解析。

```bash
./bin/whois-hacker serve --log-format json
```

### `--timeout`

- **类型**：`int`
- **默认**：`10`
- **单位**：秒
- **作用**：单次查询的超时上限。网络受限或查冷门 TLD 时可调大。

```bash
./bin/whois-hacker whois example.com --timeout 30
```

### `--use-proxy`

- **类型**：`bool`
- **默认**：`false`
- **作用**：是否启用代理池。启用后查询走 `--proxy-file` 中的代理轮询，规避 IP 封禁。详见 [proxy.go](../api/whois/proxy.md)。

```bash
./bin/whois-hacker whois example.com --use-proxy
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

### `--format`

- **类型**：`string`
- **默认**：`json`
- **可选**：`json` / `text` / `raw`
- **作用**：输出格式。**仅查询类子命令生效**（whois/ip/asn/rdap 等）。`json` 输出结构化 JSON，`text` 输出可读文本，`raw` 输出上游原始记录。

```bash
./bin/whois-hacker whois example.com --format raw
```

---

## 🌐 serve 专属 flag

`serve` 子命令用于启动常驻 HTTP 服务，flag 围绕服务运行时配置展开。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--host` | string | `127.0.0.1` | HTTP 监听地址 |
| `--port` | int | `8080` | HTTP 监听端口 |
| `--cache` | bool | `true` | 启用缓存 |
| `--cache-type` | string | `local` | 缓存后端（local/redis） |
| `--cache-ttl` | int64 | `3600` | 缓存有效期（秒） |
| `--cache-warmup` | bool | `false` | 启动时预热缓存 |
| `--warmup-file` | string | `config/warmup.json` | 预热域名列表文件 |
| `--metrics` | bool | `true` | 启用系统指标采集 |
| `--metrics-interval` | int64 | `60` | 指标采集间隔（秒） |
| `--alerts` | bool | `true` | 启用告警管理器 |
| `--alerts-interval` | int64 | `60` | 告警检查间隔（秒） |

### `--host`

- **类型**：`string`
- **默认**：`127.0.0.1`
- **作用**：HTTP 服务监听地址。
  - `127.0.0.1`：仅本机可访问（默认，安全）
  - `0.0.0.0`：所有网卡可访问（容器/对外服务时用）

```bash
./bin/whois-hacker serve --host 0.0.0.0
```

### `--port`

- **类型**：`int`
- **默认**：`8080`
- **作用**：HTTP 服务监听端口。

```bash
./bin/whois-hacker serve --port 9090
```

### `--cache`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用查询结果缓存。命中缓存可显著降低延迟与对外请求量。

```bash
# 关闭缓存
./bin/whois-hacker serve --cache=false
```

### `--cache-type`

- **类型**：`string`
- **默认**：`local`
- **可选**：`local` / `redis`
- **作用**：缓存后端。`local` 为进程内内存缓存；`redis` 连接 `localhost:6379`（地址当前在源码中固定，详见 [FAQ](./faq.md)）。

```bash
./bin/whois-hacker serve --cache-type redis
```

### `--cache-ttl`

- **类型**：`int64`
- **默认**：`3600`
- **单位**：秒
- **作用**：缓存条目有效期。后台每 5 分钟清理一次过期条目。

```bash
./bin/whois-hacker serve --cache-ttl 7200
```

### `--cache-warmup`

- **类型**：`bool`
- **默认**：`false`
- **作用**：启动时是否预热缓存（按 `--warmup-file` 列表提前查询并缓存）。

```bash
./bin/whois-hacker serve --cache-warmup
```

### `--warmup-file`

- **类型**：`string`
- **默认**：`config/warmup.json`
- **作用**：预热用的域名列表文件（JSON 数组）。

```json
["example.com", "google.com", "github.com"]
```

### `--metrics`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用系统指标采集（CPU/内存/查询统计等）。每分钟导出到 `data/metrics.json`，关闭时导出 `data/metrics_final.json`。

```bash
./bin/whois-hacker serve --metrics=false
```

### `--metrics-interval`

- **类型**：`int64`
- **默认**：`60`
- **单位**：秒
- **作用**：系统指标采集间隔。

```bash
./bin/whois-hacker serve --metrics-interval 30
```

### `--alerts`

- **类型**：`bool`
- **默认**：`true`
- **作用**：是否启用告警管理器（注册默认通知器并周期检查告警规则）。

```bash
./bin/whois-hacker serve --alerts=false
```

### `--alerts-interval`

- **类型**：`int64`
- **默认**：`60`
- **单位**：秒
- **作用**：告警规则检查间隔。

```bash
./bin/whois-hacker serve --alerts-interval 120
```

---

## 🔍 各查询子命令 flag

查询类子命令共享全局 `--timeout`/`--use-proxy`/`--format` 等，另有各自专属 flag。

### `whois <domain>`

域名 WHOIS 查询。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--max-retries` | int | — | 最大重试次数 |
| `--validate` | bool | `false` | 校验结果完整性 |
| `--follow-referral` | bool | `false` | 跟随 registry→registrar referral |
| `--required-fields` | string | — | 要求必须返回的字段（逗号分隔） |
| `--raw` | bool | `false` | 输出上游原始 WHOIS 文本 |

```bash
./bin/whois-hacker whois example.com \
  --max-retries 5 --validate --follow-referral \
  --required-fields registrar,creation_date
```

### `ip <ip>`

IP WHOIS 查询。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--raw` | bool | `false` | 输出上游原始 WHOIS 文本 |

```bash
./bin/whois-hacker ip 8.8.8.8 --raw
```

### `asn <asn>`

ASN 查询。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--source` | string | — | 数据来源（radb/rdap/all） |
| `--include-prefixes` | bool | `false` | 包含宣告的 IP 前缀 |
| `--include-bgp` | bool | `false` | 包含 BGP 信息 |

```bash
./bin/whois-hacker asn 15169 --source all --include-prefixes --include-bgp
```

### `rdap`（父命令，含 domain/ip/asn/entity 子命令）

RDAP 标准查询，无专属 flag，使用全局 `--format` 控制输出。

```bash
./bin/whois-hacker rdap domain example.com
./bin/whois-hacker rdap entity handle-xyz
```

### `batch <file>`

批量查询文件中的域名列表。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--concurrency` | int | — | 并发数 |
| `--max-retries` | int | — | 单域名最大重试 |
| `--query-delay` | int | — | 查询间隔（毫秒，规避限速） |
| `--checkpoint` | bool | `false` | 启用断点续传 |
| `--checkpoint-interval` | int | — | checkpoint 落盘间隔（秒） |

```bash
./bin/whois-hacker batch domains.txt \
  --concurrency 5 --max-retries 3 --query-delay 200 \
  --checkpoint --checkpoint-interval 30
```

### `idn <domain>`

IDN 国际化域名转换。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--action` | string | — | 转换动作（to-ascii/to-unicode/normalize/detect） |

```bash
./bin/whois-hacker idn 中文.com --action to-ascii
# → xn--fiq228c.com
```

### `format [file]`

WHOIS 文本格式检测。从文件读，未给文件则从 stdin 读。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--detect-only` | bool | `false` | 仅检测格式，不解析字段 |

```bash
cat raw.txt | ./bin/whois-hacker format --detect-only
./bin/whois-hacker format raw.txt
```

### `export <domain>`

导出域名情报为指定格式。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--format` | string | `json` | 导出格式（json/csv/markdown） |

::: tip ⚠️ 注意 export 的 --format
`export` 的 `--format` 取值为 `json/csv/markdown`，与全局 `--format`（json/text/raw）不同——此处是导出目标格式，覆盖了全局 flag。
:::

```bash
./bin/whois-hacker export example.com --format csv
```

### `servers`

WHOIS 服务器映射列表。

| flag | 类型 | 默认值 | 一句话 |
|------|------|--------|--------|
| `--tld` | string | — | 仅显示指定 TLD 的服务器 |

```bash
./bin/whois-hacker servers --tld com
```

---

## 🎚️ flag 优先级机制

```mermaid
flowchart LR
    Explicit["🚩 命令行显式 flag<br/>Flags().Changed 命中"]
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

实现要点（来自 `root.go` 的 `PersistentPreRunE`、`helpers.go` 的 `loadConfigFromFile`，以及 `cmd_serve.go` 的 `applyServeConfigFromYAML`）：

1. cobra 先解析命令行 flag（含全局持久 flag 与子命令专属 flag）
2. 在 `PersistentPreRunE` 中加载 YAML 配置文件，对 `log`/`proxy` 字段做"未显式则覆盖"
3. `serve` 子命令执行时，`applyServeConfigFromYAML` 对 `server`/`cache`/`metrics`/`alerts` 字段做同样处理
4. 用 `cmd.Flags().Changed("name")`（cobra 等价于旧 `flag.Visit`，只判断**显式设置**的 flag）记录哪些参数是命令行传入的
5. 对每个配置项：**仅当命令行未显式设置时**，才用 YAML 的值覆盖 flag 默认值

::: tip 💡 验证优先级
若同时 `serve --port 9090` 且 `config.yaml` 中 `server.port: 8080`，最终监听 **9090**（命令行优先）。
:::

::: warning ⚠️ 布尔 flag 的优先级
布尔 flag（如 `--cache`）的"是否显式设置"由 `Flags().Changed` 判断。`--cache=false` 也算显式设置，会覆盖 YAML 中的 `cache.enabled: true`。
:::

---

## 🧪 组合示例

```bash
# 生产环境 serve：对外服务 + JSON 日志 + Redis 缓存 + 代理池
./bin/whois-hacker serve \
  --host 0.0.0.0 --port 8080 \
  --log-level info --log-format json \
  --cache-type redis --cache-ttl 7200 \
  --use-proxy --proxy-file config/proxies.json

# 调试环境 serve：详细日志 + 关闭缓存看实时查询
./bin/whois-hacker serve \
  --log-level debug \
  --cache=false

# 轻量环境 serve：关闭监控与告警，降低开销
./bin/whois-hacker serve \
  --metrics=false --alerts=false

# 直接查询：带全局 flag 与子命令 flag
./bin/whois-hacker whois example.com \
  --log-level debug --format json \
  --max-retries 5 --validate --required-fields registrar,creation_date
```

---

## 🔗 相关文档

- ⚙️ [配置文件](./config-file.md) — YAML 字段与 flag 的对应关系
- 📝 [日志与输出](./logging.md) — 日志级别与格式详解
- 🚀 [启动与运行](./usage.md) — 完整启动流程
