# 🏗️ 架构总览

> 📐 理解 Whois Hacker 的整体设计与模块关系。

---

## 🧭 分层架构

Whois Hacker 采用**分层架构**，从上到下分为：入口层、服务层、能力层、基础设施层。

```mermaid
flowchart TB
    subgraph Entry["🚀 入口层 (cmd)"]
        CLI["命令行 flag<br/>YAML 配置 · 优雅关闭 · 版本注入"]
    end
    subgraph Service["🌐 服务层 (api · mcp · security)"]
        HTTP["HTTP 路由"]
        MW["中间件链"]
        MCP["MCP 任务流"]
        AUTH["API Key 认证"]
    end
    subgraph Capability["🔍 能力层 (pkg/whois · 24 个文件)"]
        Q["域名/IP/ASN/RDAP/反向"]
        B["批量 · 关联"]
        M["监控 · 质量"]
    end
    subgraph Infra["🏗️ 基础设施层 (whois 内置子系统)"]
        Cache["缓存"]
        Proxy["代理池"]
        RL["限流"]
        Sched["调度"]
        Srv["服务器管理"]
        Obs["可观测"]
    end
    CLI --> HTTP
    HTTP --> MW
    MW --> MCP
    MW --> AUTH
    HTTP --> Q
    MCP --> Q
    Q --> B
    Q --> M
    Q --> Cache
    Q --> Proxy
    Q --> RL
    Q --> Sched
    Q --> Srv
    Q --> Obs

    classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
    classDef service fill:#647eff,color:#fff,stroke:#4a5fd6
    classDef cap fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef infra fill:#909399,color:#fff,stroke:#6b6e72
    class CLI entry
    class HTTP,MW,MCP,AUTH service
    class Q,B,M cap
    class Cache,Proxy,RL,Sched,Srv,Obs infra
```

---

## 🧩 模块职责

| 模块 | 路径 | 职责 | 文档 |
|------|------|------|------|
| 🚀 cmd | `cmd/whois-hacker/` | 程序入口、配置加载、生命周期管理 | [cmd 模块](../modules/cmd.md) |
| 🌐 api | `pkg/api/` | HTTP 路由、中间件、响应封装 | [api 模块](../modules/api.md) |
| 🤖 mcp | `pkg/mcp/` | MCP 任务规划/执行/审批状态机 | [mcp 模块](../modules/mcp.md) |
| 🔍 whois | `pkg/whois/` | **核心能力库**，23 个源文件 | [whois 模块](../modules/whois.md) |
| 📈 metrics | `pkg/metrics/` | 指标采集、告警规则、通知器 | [metrics 模块](../modules/metrics.md) |
| 👁️ monitor | `pkg/monitor/` | WHOIS 查询性能监控 | [monitor 模块](../modules/monitor.md) |
| 🔒 security | `pkg/security/` | API Key 管理、认证中间件 | [security 模块](../modules/security.md) |

---

## 🔍 能力层：WHOIS 核心包内部结构

`pkg/whois` 是核心，包含 23 个源文件，按职责可分为五大类：

### 1️⃣ 查询能力

| 文件 | 能力 | 图标 |
|------|------|------|
| `query.go` | 域名 WHOIS 查询引擎、优先级队列聚合 | 🔎 |
| `ipwhois.go` | IP WHOIS 查询（IANA 引导 → RIR） | 🌐 |
| `asn.go` / `asn_enhanced.go` | ASN 查询（RADB + RDAP） | 🔢 |
| `rdap.go` | RDAP 标准查询（RFC 9083） | 📡 |
| `reverse.go` | 反向 WHOIS（Provider 抽象） | 🔄 |

### 2️⃣ 解析与处理

| 文件 | 能力 | 图标 |
|------|------|------|
| `ipparser.go` | IP WHOIS 响应结构化解析（5 大 RIR） | 🔬 |
| `format.go` | WHOIS 格式检测与原始文本清洗 | 📝 |
| `idn.go` | 国际化域名 Punycode 转换 | 🌍 |
| `diff.go` | 两份 WHOIS 信息字段差异对比 | 📊 |
| `export.go` | 导出 JSON/CSV/Markdown | 📤 |

### 3️⃣ 工程化能力

| 文件 | 能力 | 图标 |
|------|------|------|
| `batch.go` | 流式批量查询、断点续查 | 📋 |
| `cache.go` | 本地/Redis 双缓存、预热 | 💾 |
| `proxy.go` | SOCKS5/HTTP 代理池、故障熔断 | 🔒 |
| `ratelimit.go` | 令牌桶限速（全局+每服务器） | ⏱️ |
| `scheduler.go` | 自适应智能调度 | 🎛️ |
| `servers.go` | WHOIS 服务器管理、健康检查 | 🖥️ |

### 4️⃣ 情报分析

| 文件 | 能力 | 图标 |
|------|------|------|
| `correlation.go` | 多域名关联分析、资产画像 | 🔗 |
| `quality.go` | WHOIS 数据质量三维评分、隐私检测 | ⭐ |
| `availability.go` | 域名可注册性检测 | ✅ |
| `monitor.go` | 域名监控、到期/变更告警 | 👁️ |

### 5️⃣ 配置与可观测

| 文件 | 能力 | 图标 |
|------|------|------|
| `config.go` | 统一配置结构、JSON/YAML 加载 | ⚙️ |
| `errors.go` | 统一错误类型体系、分类器 | ❌ |
| `observability.go` | Metrics 接口、Prometheus/OTel | 📈 |

---

## 🔄 调用关系

```mermaid
flowchart TD
    CMD["cmd/whois-hacker<br/>程序入口"]

    CMD --> L1["加载配置<br/>whois.LoadYAMLConfig"]
    CMD --> L2["初始化缓存<br/>whois.NewWhoisCache"]
    CMD --> L3["加载代理<br/>whois.LoadProxiesFromFile"]
    CMD --> L4["启动监控<br/>metrics.StartSystemMetricsCollection"]
    CMD --> L5["启动告警<br/>metrics.StartAlertManager"]
    CMD --> L6["启动 API<br/>api.NewServer"]

    L6 --> R["注册 HTTP 路由"]
    L6 --> MWS["中间件链<br/>Recovery → Logging → CORS → Auth"]
    L6 --> MCPP["注册 MCP 路由<br/>mcp.NewServer"]

    MCPP --> CTRL["Controller 委托"]
    CTRL --> W["whois.* 查询函数"]

    R -.委托.-> W
    MWS -.保护.-> R

    classDef start fill:#41b883,color:#fff,stroke:#2b7a4b
    classDef init fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef api fill:#647eff,color:#fff,stroke:#4a5fd6
    classDef core fill:#909399,color:#fff,stroke:#6b6e72
    class CMD start
    class L1,L2,L3,L4,L5,L6 init
    class R,MWS,MCPP,CTRL api
    class W core
```

---

## 🎯 设计原则

<div class="feature-grid">

<div class="feature-card">
<span class="feature-icon">🔌</span>
<div class="feature-title">接口统一</div>
<div class="feature-desc">所有查询能力暴露一致的 Go 函数签名与 HTTP 端点，支持 context 超时。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🧱</span>
<div class="feature-title">分层解耦</div>
<div class="feature-desc">能力层不依赖服务层，可作为纯库使用；基础设施可独立替换。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🛡️</span>
<div class="feature-title">防御性</div>
<div class="feature-desc">代理池故障熔断、令牌桶限速、自适应退避，规避外部限流。</div>
</div>

<div class="feature-card">
<span class="feature-icon">📦</span>
<div class="feature-title">单例复用</div>
<div class="feature-desc">缓存、代理池、服务器管理、监控器等均为单例，全局共享状态。</div>
</div>

</div>

---

## 📚 深入阅读

- 🧩 **[模块全景](./modules-overview.md)** — 各模块速览
- 🔄 **[查询流程](./query-flow.md)** — 一次查询的完整链路
- ⚙️ **[配置系统](./configuration.md)** — 配置项详解
