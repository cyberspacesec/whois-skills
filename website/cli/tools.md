# 🛠️ 运维与本地工具

> 🎯 本页介绍 CLI 新增的运维类子命令组与扩展命令：`config` / `cache` / `proxy` / `metrics` / `tools` 五大顶层子命令组，以及 `servers` / `rdap bootstrap` / `correlation` 扩展与 `batch resume` 断点续查。基于对源码 `cmd/whois-hacker/cmd_config.go`、`cmd_cache.go`、`cmd_proxy.go`、`cmd_metrics.go`、`cmd_tools.go`、`cmd_tools_extra.go`、`cmd_query.go`、`cmd_analyze.go` 的实际验证整理。

::: warning ⚠️ 跨进程状态局限
`cache` 与 `metrics` 操作的是 `pkg/whois` 的全局实例（`GetCache` / `GetGlobalMetrics`），这些实例是**进程内**的。每次 CLI 调用都是一个**新进程**，全局实例会重新懒加载初始化——因此 `cache stats` / `cache get` / `metrics stats` / `metrics export` 看到的是**本次进程内**的状态，无法观察到上一次 CLI 调用留下的缓存或指标。

要观察跨查询的缓存命中与指标累积，请用 `serve` 常驻模式，通过 HTTP 端点（`/api/servers`、`/api/metrics` 等）查看；或在脚本中先 `config apply` 一份启用 Redis 的库配置，再让查询与缓存走 Redis（跨进程共享）。
:::

---

## ⚙️ `config` —— 库配置（WhoisLibraryConfig）管理

`config` 子命令组管理**库级运行时配置**（`WhoisLibraryConfig`），覆盖查询/缓存/代理/限速/批量/监控/调度/可观测/日志九大子系统。它与全局 `--config` flag（加载应用级 `AppConfig` YAML）**正交**：本子命令操作的是库配置，格式为 JSON（`WhoisLibraryConfig` 的序列化形式）。

| 子命令 | 用途 |
|--------|------|
| `config show` | 显示库配置（默认值 / 当前全局 / 指定文件） |
| `config validate <file>` | 校验库配置文件是否合法 |
| `config save <file>` | 把默认或当前全局库配置保存到文件 |
| `config merge <base> <override>...` | 合并多份库配置文件 |
| `config apply <file>` | 加载并应用库配置到全局（影响后续查询行为） |

`config show` 的 flag：

| flag | 说明 |
|------|------|
| `--default` | 显示默认库配置 |
| `--file <path>` | 显示指定文件中的库配置 |
| `--summary` | 输出可读摘要（默认行为） |
| `--json` | 输出结构化 JSON |

`config save` 支持 `--default`（保存默认值，否则保存当前全局）。`config merge` 支持 `--json`（默认输出可读摘要，合并策略：override 中非零值覆盖 base）。

**典型示例**：

```bash
# 看默认库配置长什么样
whois-hacker config show --default

# 看当前全局库配置（json）
whois-hacker config show --json

# 保存默认配置到文件作为模板
whois-hacker config save default.json --default

# 校验自己编辑的库配置
whois-hacker config validate my.json

# 合并 base + override（输出摘要）
whois-hacker config merge base.json override.json

# 先应用一份启用 Redis 缓存的库配置，再查询（影响后续查询默认行为）
whois-hacker config apply redis.json
whois-hacker whois example.com
```

📖 配置文件体系详见 [配置文件](./config-file.md)，库配置字段详见 [配置系统](../guide/configuration.md)。

---

## 💾 `cache` —— WHOIS 缓存运维

`cache` 子命令组管理 `whois` 库的全局缓存实例（`GetCache`）。查询类子命令（`whois` / `ip` / `asn` 等）的结果会写入此缓存，`cache` 子命令用于运维与调试。

| 子命令 | 用途 |
|--------|------|
| `cache stats` | 显示缓存统计信息 |
| `cache get <domain>` | 查询指定域名的缓存条目 |
| `cache delete <domain>` | 删除指定域名的缓存条目 |
| `cache clear` | 清空全部缓存 |
| `cache clear-expired` | 清理过期的缓存条目 |
| `cache asn list` | 列出全部 ASN 详情缓存 |
| `cache asn clear` | 清空 ASN 详情缓存 |

`cache stats` / `cache get` / `cache asn list` 支持 `--json` 输出。`cache stats` 在缓存禁用时输出"缓存: 已禁用"，启用时输出类型/条目数/命中/未命中/过期/总请求/命中率。

**典型示例**：

```bash
# 看缓存统计
whois-hacker cache stats

# 查某域名的缓存条目（含完整 WhoisInfo）
whois-hacker cache get example.com --json

# 删一个条目
whois-hacker cache delete example.com

# 清空全部
whois-hacker cache clear

# 清过期
whois-hacker cache clear-expired

# ASN 详情缓存
whois-hacker cache asn list
whois-hacker cache asn clear
```

::: tip ℹ️ 跨进程局限
见本页顶部"跨进程状态局限"。CLI 单次调用的缓存命中率为 0 是正常的——查询与缓存统计不在同一进程。要观察跨查询命中，请用 `serve` 常驻模式。
:::

---

## 🔒 `proxy` —— 代理池运维

`proxy` 子命令组管理 `whois` 库的全局代理池（`GetProxyPool`），与全局 `--use-proxy` flag 配合使用。

| 子命令 | 用途 |
|--------|------|
| `proxy list` | 列出全部代理及其状态（可用/失败次数/平均响应/最后检查） |
| `proxy stats` | 显示代理池汇总统计（总数/可用/最后更新） |
| `proxy set <address>` | 设置单个全局 WHOIS 代理（`SetWhoisProxy`，替换默认客户端） |

`proxy set` 的 flag：

| flag | 默认 | 说明 |
|------|------|------|
| `--type` | `socks5` | 代理类型（`socks5` / `http`） |
| `--user` | （空） | 代理用户名 |
| `--pass` | （空） | 代理密码 |
| `--timeout` | `30` | 代理超时（秒） |

::: warning ⚠️ `proxy set` 与 `--use-proxy` 是两套机制
- `proxy set <address>` → 设置单个代理到 `defaultClient`（**不进** `ProxyPool`，`proxy list` 看不到）。
- `--use-proxy --proxy-file <file>` → 加载代理列表到 `ProxyPool`，查询时**轮询**。

二者独立，可按场景选择：单代理固定出口用 `proxy set`，多代理负载均衡用 `--use-proxy`。
:::

**典型示例**：

```bash
# 列出代理池（用 --use-proxy --proxy-file 加载后看得到）
whois-hacker proxy list

# 汇总统计
whois-hacker proxy stats

# 设置单个 socks5 代理
whois-hacker proxy set 127.0.0.1:1080 --type socks5

# 设置带认证的 http 代理
whois-hacker proxy set proxy.example.com:8080 --type http --user alice --pass secret
```

---

## 📈 `metrics` —— 指标查看与导出

`metrics` 子命令组查看与导出 `whois` 库的全局指标（`GetGlobalMetrics`）。

| 子命令 | 用途 |
|--------|------|
| `metrics stats` | 显示全局内置指标（`BuiltInStats`） |
| `metrics export` | 导出为 Prometheus exposition 文本格式 |

`metrics stats` 输出：总查询数 / 成功 / 失败 / 缓存命中 / 缓存未命中 / API 请求数 / 限流事件 / 总查询耗时 / 平均查询耗时。支持 `--json`。

`metrics export` 输出 Prometheus 文本格式，可直接被 Prometheus 抓取，指标名包括：`whois_queries_total`、`whois_queries_successful_total`、`whois_queries_failed_total`、`whois_cache_hits_total`、`whois_cache_misses_total`、`whois_api_requests_total`、`whois_rate_limit_events_total`、`whois_query_time_ms_total`。

**典型示例**：

```bash
# 看内置指标
whois-hacker metrics stats

# JSON 形式
whois-hacker metrics stats --json

# 导出 Prometheus 文本
whois-hacker metrics export
```

::: tip ℹ️ 跨进程局限
见本页顶部"跨进程状态局限"。单次 CLI 调用的指标都是 0 是正常的。要观察跨查询累积指标，请用 `serve` 常驻模式后访问 `/api/metrics`。
:::

---

## 🔧 `tools` —— 本地解析与提取工具（不联网）

`tools` 子命令组暴露 `whois` 库的本地工具函数，**纯本地计算，不发起网络请求**（`asn-prefixes` 与 `asn-ip-ranges` 除外，二者需联网查 ASN 详情）。

| 子命令 | 用途 |
|--------|------|
| `tools ip-parse <ip>` | 解析 IP WHOIS 原始文本为结构化信息（从 stdin 或 `--file` 读） |
| `tools domain <domain>` | 解析域名为结构化信息（TLD/SLD/子域名/通配符基础） |
| `tools tld <domain>` | 提取域名的有效 TLD（含复合 TLD 如 `.co.uk`） |
| `tools normalize <type> <value>` | 规范化联系人字段（`phone` / `name` / `email`） |
| `tools asn-prefixes <asn>` | 统计 ASN 的 IPv4/IPv6 前缀数（需联网查 ASN 详情） |
| `tools asn-ip-ranges <asn>` | 按 ASN 取宣告的 IP 段（需联网） |

`tools domain` 与 `tools asn-ip-ranges` 支持 `--json`。`tools tld` 支持 `--simple`（提取简单 TLD，即最后一段，默认提取有效 TLD）。`tools ip-parse` 支持 `--file`（默认 stdin）。

**典型示例**：

```bash
# 提取有效 TLD（复合 TLD 正确识别 .co.uk）
whois-hacker tools tld a.b.example.co.uk
# 输出: co.uk

# 提取简单 TLD（最后一段）
whois-hacker tools tld a.b.example.co.uk --simple
# 输出: uk

# 解析域名结构
whois-hacker tools domain a.b.example.co.uk
# 输出: 完整域名/顶级域/域名/子域名/通配符基础

# 解析 IP WHOIS 原始文本（管道）
cat raw.txt | whois-hacker tools ip-parse 8.8.8.8

# 规范化电话号
whois-hacker tools normalize phone "+1.234 567-8900"

# ASN 前缀数统计（联网）
whois-hacker tools asn-prefixes 13335

# ASN 宣告的 IP 段（联网）
whois-hacker tools asn-ip-ranges AS13335 --json
```

---

## 🖥️ `servers` —— WHOIS 服务器映射管理

`servers` 子命令组管理 WHOIS 服务器映射（`WhoisServerManager`）。直接调用 `servers`（无子命令）兼容旧行为：列出全部 TLD → 服务器映射，可按 `--tld` 过滤。

| 命令 | 用途 |
|------|------|
| `servers` | 列出全部映射（可 `--tld` 过滤） |
| `servers list` | 列出全部映射（同上） |
| `servers stats` | 显示服务器健康统计（总数/健康） |
| `servers discover <tld>` | 在线发现指定 TLD 的 WHOIS 服务器 |
| `servers refresh` | 刷新服务器列表 |
| `servers save <file>` | 保存当前映射到文件 |

`servers` / `servers list` 支持 `--tld`，`servers stats` 支持 `--json`。

**典型示例**：

```bash
# 列出全部映射
whois-hacker servers

# 只看 com 的服务器
whois-hacker servers --tld com

# 健康统计
whois-hacker servers stats

# 在线发现 com 的 WHOIS 服务器
whois-hacker servers discover com

# 刷新列表
whois-hacker servers refresh

# 保存当前映射
whois-hacker servers save my-servers.json
```

---

## 📡 `rdap bootstrap` —— RDAP bootstrap 映射查看

`rdap` 子命令组通过 RDAP（RFC 9083）查询域名/IP/ASN/实体信息。其中 `rdap bootstrap` 只查看 bootstrap 映射（TLD/ASN → RDAP 服务器），**不发起 RDAP 查询，仅返回元数据**。

| 子命令 | 用途 |
|------|------|
| `rdap domain <domain>` | RDAP 查询域名 |
| `rdap ip <ip>` | RDAP 查询 IP |
| `rdap asn <asn>` | RDAP 查询 ASN |
| `rdap entity <handle>` | RDAP 查询实体 |
| `rdap bootstrap` | 查看 RDAP bootstrap 映射（`--tld` / `--asn`） |

`rdap bootstrap` 的 flag：

| flag | 说明 |
|------|------|
| `--tld <tld>` | 按顶级域查看 RDAP 服务器 |
| `--asn <asn>` | 按 ASN 查看 RDAP 服务器（如 `13335` 或 `AS13335`） |

二者至少给一个，可同时给出（分别返回各自的 RDAP 服务器）。

**典型示例**：

```bash
# 看 com 的 RDAP 服务器
whois-hacker rdap bootstrap --tld com

# 看 AS13335 的 RDAP 服务器
whois-hacker rdap bootstrap --asn 13335

# 实际发起 RDAP 查询
whois-hacker rdap domain example.com
whois-hacker rdap ip 8.8.8.8
whois-hacker rdap asn 15169
```

---

## 🔗 `correlation` —— 多域名关联分析（扩展）

`correlation` 按邮箱/注册人/组织/NS/注册商五维聚类，构建关联图与资产画像。直接调用 `correlation <domains>` 执行完整分析（同 `analyze` 子命令）；子命令提供更细粒度的视图。

| 命令 | 用途 |
|------|------|
| `correlation <d1> [d2...]` | 完整关联分析（图+聚类+画像） |
| `correlation analyze <d1> [d2...]` | 完整关联分析（同上） |
| `correlation profile <d1> [d2...] --id <id> --type <t>` | 查指定实体资产画像 |
| `correlation registrars <d1> [d2...]` | 注册商维度统计 |

`correlation profile` 的 flag：

| flag | 说明 |
|------|------|
| `--id <entityID>` | 实体 ID（邮箱/注册人/组织名），必填 |
| `--type <type>` | 实体类型：`email` / `registrant` / `organization`（`org`），必填 |

**典型示例**：

```bash
# 完整关联分析（直接调用）
whois-hacker correlation a.com b.com c.com

# 同上（显式子命令）
whois-hacker correlation analyze a.com b.com c.com

# 先 analyze 拿到 entity ID，再深入查某实体画像
whois-hacker correlation profile a.com b.com c.com \
  --id "admin@example.com" --type email

# 注册商维度统计
whois-hacker correlation registrars a.com b.com c.com
```

---

## 📋 `batch` —— 断点续查

`batch` 从文本文件读取域名列表（每行一个，`#` 注释），流式批量查询，支持并发、限速、断点续查与进度输出。

| 命令 | 用途 |
|------|------|
| `batch <file>` | 从文件批量查询域名 |
| `batch resume` | 从断点文件恢复未完成的批量查询 |

`batch` 与 `batch resume` 共享 flag：

| flag | 默认 | 说明 |
|------|------|------|
| `--concurrency` | `5` | 并发数 |
| `--max-retries` | `3` | 最大重试次数 |
| `--query-delay` | `200` | 域间查询延迟（毫秒） |
| `--checkpoint` | （空） | 断点续查文件路径（`resume` 必填） |
| `--checkpoint-interval` | `10` | 每完成 N 个保存一次断点 |

`batch` 启动后台 worker 后立即返回，结果通过 `processor.Results()` 流式产出；进度回调（`[N/M] 成功 X 失败 Y 剩余 Z`）输出到 **stderr**，stdout 只输出最终 JSON。`batch resume` 只处理断点中尚未完成的域名。

**典型示例**：

```bash
# 正常批量查询（带断点）
whois-hacker batch domains.txt --checkpoint cp.json

# 中断后续跑（只处理 cp.json 中尚未完成的域名）
whois-hacker batch resume --checkpoint cp.json

# 调高并发 + 限速
whois-hacker batch domains.txt --concurrency 10 --query-delay 100 \
  --checkpoint cp.json --checkpoint-interval 20
```

::: tip ✅ batch cancel 时序 bug 已修复
历史上 `batch` 查询成功但结果始终为 `results: null`、进度回调不输出，根因是 `Process` 用 `defer p.cancel()` 在返回瞬间取消了 ctx，导致后台 worker 立即从 `ctx.Done()` 退出而不产出结果。已修复，详见 [常见问题](./faq.md)。
:::

---

## 🔗 相关文档

- 🚩 [命令行参数](./flags.md) — 全局 flag 与子命令 flag 速查
- ⚙️ [配置文件](./config-file.md) — 应用级 `AppConfig` YAML 与库级 `WhoisLibraryConfig` JSON
- 🚀 [启动与运行](./usage.md) — `serve` 常驻模式（跨查询观察 cache/metrics）
- ❓ [常见问题](./faq.md) — 已知 bug 与排查
