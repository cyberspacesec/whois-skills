# 🔄 查询流程

> 🔬 跟随一次域名 WHOIS 查询，理解完整链路。

---

## 🎯 场景：查询 `example.com`

调用 `whois.ExecuteQueryWithResult(&QueryOptions{Domain: "example.com"})` 时发生了什么？

---

## 📐 完整链路

```
1. ExecuteQueryWithResult(opts)
   │
   ├─ 2. 校验 Domain 非空
   │
   ├─ 3. 设置默认超时 (10s)
   │
   └─ 4. 重试循环 (0 .. MaxRetries, 默认 5)
        │
        ├─ 4.1 检查 ctx.Err() → 超时则退出
        │
        ├─ 4.2 GetServerManager().GetWhoisServer("example.com")
        │       │
        │       ├─ ExtractTLD("example.com") → "com"
        │       │      └─ domain_util.FldDomain (基于 PSL)
        │       │
        │       ├─ 查 servers["com"] → "whois.verisign-grs.com"
        │       │
        │       └─ getHealthyServer → 检查 serverHealth
        │              └─ 不健康则回退 defaultServer
        │
        ├─ 4.3 executeQueryWithTimeout(ctx, server, domain)
        │       │
        │       ├─ 附加 ctx 超时 (若无 deadline)
        │       │
        │       ├─ goroutine + channel + select 实现超时
        │       │
        │       ├─ UseProxy=true ?
        │       │   ├─ 是 → DirectWhoisWithContext (自定义拨号)
        │       │   │       └─ NewWhoisClient → 代理池/缓存/限流
        │       │   └─ 否 → whois.Whois (likexian 库直连)
        │       │
        │       └─ 返回 rawResponse, server, usedProxy
        │
        ├─ 4.4 whoisparser.Parse(rawResponse)
        │       │
        │       ├─ 成功 → 跳出重试循环
        │       └─ 失败 → CheckError 判断是否可重试
        │              ├─ 可重试 (连接重置/限速/超时) → 等待 IntervalMils 后重试
        │              └─ 不可重试 (解析失败) → 直接返回错误
        │
        └─ 4.5 计算 Latency, 构造 QueryResult
                │
                └─ ValidateResult=true ?
                    └─ validateQueryResult
                        └─ reflect 遍历结构体校验 RequiredFields
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

```
WhoisClient.QueryWithContext
  ├─ 查缓存 → 命中返回
  ├─ 有代理池 ?
  │   ├─ 是 → queryWithProxyPoolContext (轮询代理)
  │   └─ 否 → queryDirectContext
  │           ├─ extractTLD → getWhoisServer
  │           ├─ RateLimiter.Allow
  │           ├─ rawWhoisQuery (拨 server:43)
  │           ├─ extractReferralServer → 跟随 referral (≤3)
  │           └─ 解析成功才缓存
  └─ 返回 rawResponse
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
