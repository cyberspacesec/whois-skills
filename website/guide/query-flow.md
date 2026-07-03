# 🔄 查询流程

> 🔬 跟随一次域名 WHOIS 查询，理解完整链路。

---

## 🎯 场景：查询 `example.com`

调用 `whois.ExecuteQueryWithResult(&QueryOptions{Domain: "example.com"})` 时发生了什么？

下方的时序图展示了调用方、查询引擎、服务器管理器、WHOIS 服务器与解析器之间的交互：

```mermaid
sequenceDiagram
    autonumber
    participant Caller as 调用方
    participant Engine as ExecuteQueryWithResult
    participant SM as ServerManager
    participant WS as whois.verisign-grs.com
    participant Parser as whoisparser.Parse

    Caller->>Engine: QueryOptions{Domain:"example.com"}
    Engine->>Engine: 校验 Domain + 设置 10s 超时

    loop 重试 0..5 次
        Engine->>SM: GetWhoisServer("example.com")
        SM->>SM: ExtractTLD → "com"
        SM->>SM: 查 servers 映射 + 健康检查
        SM-->>Engine: whois.verisign-grs.com
        Engine->>WS: 拨号 server:43 / 经代理
        WS-->>Engine: raw WHOIS 文本
        Engine->>Parser: Parse(rawResponse)
        alt 解析成功
            Parser-->>Engine: WhoisInfo
            Note over Engine: 跳出重试循环
        else 可重试错误
            Parser-->>Engine: error
            Engine->>Engine: CheckError → 等待后重试
        else 不可重试
            Parser-->>Engine: error
            Note over Engine: 直接返回错误
        end
    end

    Engine->>Engine: 计算 Latency + validateQueryResult
    Engine-->>Caller: QueryResult
```

---

## 📐 完整链路

```mermaid
flowchart TD
    Start(["ExecuteQueryWithResult(opts)"])

    Start --> V1["2. 校验 Domain 非空"]
    V1 --> V2["3. 设置默认超时 10s"]
    V2 --> Loop{"4. 重试循环<br/>0 .. MaxRetries (默认 5)"}

    Loop --> C1["4.1 检查 ctx.Err()"]
    C1 --> C1D{"超时?"}
    C1D -- 是 --> End1(["退出返回错误"])
    C1D -- 否 --> C2["4.2 GetWhoisServer"]

    C2 --> TLD["ExtractTLD → com<br/>domain_util.FldDomain (PSL)"]
    TLD --> MAP["查 servers[com]<br/>→ whois.verisign-grs.com"]
    MAP --> HL["getHealthyServer<br/>检查 serverHealth"]
    HL --> HLD{"健康?"}
    HLD -- 否 --> DEF["回退 defaultServer"]
    HLD -- 是 --> C3
    DEF --> C3["4.3 executeQueryWithTimeout"]

    C3 --> CTX["附加 ctx 超时"]
    CTX --> GR["goroutine + channel + select"]
    GR --> PR{"UseProxy?"}
    PR -- 是 --> DP["DirectWhoisWithContext<br/>自定义拨号<br/>代理池/缓存/限流"]
    PR -- 否 --> LK["whois.Whois (likexian 直连)"]
    DP --> RET["返回 rawResponse, server, usedProxy"]
    LK --> RET

    RET --> C4["4.4 whoisparser.Parse"]
    C4 --> PTD{"解析成功?"}
    PTD -- 是 --> C5["4.5 计算 Latency<br/>构造 QueryResult"]
    PTD -- 否 --> CE["CheckError"]
    CE --> CED{"可重试?<br/>(连接重置/限速/超时)"}
    CED -- 是 --> Wait["等待 IntervalMils"] --> Loop
    CED -- 否 --> End2(["返回错误"])

    C5 --> VLD{"ValidateResult?"}
    VLD -- 是 --> VR["validateQueryResult<br/>reflect 校验 RequiredFields"]
    VLD -- 否 --> Done
    VR --> Done(["返回 QueryResult ✓"])

    classDef start fill:#41b883,color:#fff,stroke:#2b7a4b
    classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef proc fill:#647eff,color:#fff,stroke:#4a5fd6
    classDef err fill:#f56c6c,color:#fff,stroke:#c04040
    classDef ok fill:#67c23a,color:#fff,stroke:#4e8e2a
    class Start,Done start
    class Loop,C1D,HLD,PR,PTD,CED,VLD check
    class V1,V2,C1,C2,TLD,MAP,HL,DEF,C3,CTX,GR,DP,LK,RET,C4,CE,Wait,C5,VR proc
    class End1,End2 err
    class Done ok
```

---

## 🛡️ 基础设施介入点

查询过程中，以下子系统会介入：

<div class="feature-grid">

<div class="feature-card">
<span class="feature-icon">🖥️</span>
<div class="feature-title">服务器管理</div>
<div class="feature-desc"><code>servers.go</code> 维护 TLD→服务器映射，5 分钟后台健康检查，不健康自动回退。</div>
</div>

<div class="feature-card">
<span class="feature-icon">💾</span>
<div class="feature-title">缓存</div>
<div class="feature-desc"><code>proxy.go</code> 的 <code>WhoisClient.QueryWithContext</code> 会先查缓存，命中则直接返回。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🔒</span>
<div class="feature-title">代理池</div>
<div class="feature-desc"><code>UseProxy</code> 时通过代理池轮询，失败标记故障熔断，连续失败 ≥3 标记不可用。</div>
</div>

<div class="feature-card">
<span class="feature-icon">⏱️</span>
<div class="feature-title">限速</div>
<div class="feature-desc"><code>RateLimiter.Allow(server)</code> 全局+每服务器双维度令牌桶，限速时阻塞等待。</div>
</div>

<div class="feature-card">
<span class="feature-icon">❌</span>
<div class="feature-title">错误分类</div>
<div class="feature-desc"><code>errors.go</code> 的 <code>CheckError</code> 按消息字符串分类，<code>IsRetryable</code> 决定是否重试。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🔄</span>
<div class="feature-title">引导跟随</div>
<div class="feature-desc">注册局返回 referral 时，<code>extractReferralServer</code> 提取注册商服务器继续查询（默认 3 次）。</div>
</div>

</div>

---

## 🌐 两条查询路径

Whois Hacker 有两条查询路径：

### 路径 A：likexian 库直连（默认）

`UseProxy=false` 时，走 `github.com/likexian/whois.Whois`，简单可靠。

### 路径 B：自定义客户端（启用代理或需要完整控制）

`UseProxy=true` 时，走 `DirectWhoisWithContext` → `WhoisClient`：

```mermaid
flowchart TD
    Entry(["WhoisClient.QueryWithContext"])

    Entry --> Cache{"查缓存"}
    Cache -- 命中 --> RetCache(["直接返回 rawResponse"])
    Cache -- 未命中 --> Pool{"有代理池?"}

    Pool -- 是 --> QPP["queryWithProxyPoolContext<br/>轮询代理"]
    Pool -- 否 --> QD["queryDirectContext"]

    QPP --> QD
    QD --> S1["extractTLD → getWhoisServer"]
    S1 --> S2["RateLimiter.Allow"]
    S2 --> S3["rawWhoisQuery (拨 server:43)"]
    S3 --> S4["extractReferralServer<br/>跟随 referral (≤3 次)"]
    S4 --> S5{"解析成功?"}
    S5 -- 是 --> WriteCache["写入缓存"]
    S5 -- 否 --> NoCache["不缓存"]
    WriteCache --> Ret(["返回 rawResponse"])
    NoCache --> Ret

    classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
    classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
    classDef proc fill:#647eff,color:#fff,stroke:#4a5fd6
    classDef ok fill:#67c23a,color:#fff,stroke:#4e8e2a
    class Entry,Ret,RetCache entry
    class Cache,Pool,S5 check
    class QPP,QD,S1,S2,S3,S4,WriteCache,NoCache proc
```

::: tip 💡 何时用路径 B
- 需要规避 IP 封禁 → 启用代理池
- 需要缓存 → 路径 B 自动写入缓存
- 需要限速 → 路径 B 受 RateLimiter 约束
:::

---

## 📊 结果结构

`QueryResult` 包含完整信息：

```go
type QueryResult struct {
    Info             *whoisparser.WhoisInfo  // 解析后的结构化数据
    RawResponse      string                  // 原始 WHOIS 文本
    QueryTime        time.Time               // 查询时刻
    Latency          int64                   // 延迟（毫秒）
    Server           string                  // 实际查询的服务器
    UsedProxy        bool                    // 是否用了代理
    RetryCount       int                     // 重试次数
    ValidationResult *ValidationResult       // 结果校验
}
```

---

## 🔗 相关文档

- 🔎 **[查询引擎 query.go](../api/whois/query.md)** — 完整 API
- 🖥️ **[服务器管理 servers.go](../api/whois/servers.md)** — TLD 映射
- ❌ **[错误体系 errors.go](../api/whois/errors.md)** — 错误分类
- 🎯 **[域名查询教程](./tutorial-domain.md)** — 动手实践
