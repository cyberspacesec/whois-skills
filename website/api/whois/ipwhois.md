# 📡 ipwhois.go — IP 地址 WHOIS 查询

> 📖 IP 地址 WHOIS 查询客户端，通过 IANA 引导（`whois.iana.org`）定位到对应 RIR，再查询获取详细网络信息。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/ipwhois.go` |
| 核心职责 | IP WHOIS 查询 |
| 引导服务器 | `whois.iana.org` |
| 查询流程 | IANA 引导 → RIR 查询 |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

// 1. 简化入口
result, err := whois.QueryIP("8.8.8.8")
if err != nil {
    log.Fatal(err)
}
fmt.Println("RIR 服务器：", result.Server)
fmt.Println("延迟：", result.Latency, "ms")

// 2. 带选项
result, err = whois.QueryIPWithOptions(whois.IPWhoisOptions{
    IP:      "1.1.1.1",
    Timeout: 15,
    UseProxy: true,
})
```

---

## 📊 核心类型

### IPWhoisResult

```go
type IPWhoisResult struct {
    IP           string                    // 查询的 IP
    RawResponse  string                    // 原始 WHOIS 响应
    QueryTime    time.Time                 // 查询时刻
    Server       string                    // 实际查询的 RIR 服务器
    Latency      int64                     // 延迟（毫秒）
    Info         *whoisparser.WhoisInfo    // 解析结果（通常为空）
}
```

:::tip
`Info` 字段通常为 `nil`，因为 `whoisparser.Parse` 主要面向域名 WHOIS，对 IP RIR 响应解析能力有限。如需结构化 IP 信息，请用 [ipparser.md](./ipparser.md) 的 `ParseIPWhois`。
:::

### IPWhoisOptions

```go
type IPWhoisOptions struct {
    IP       string        // IP 地址
    Timeout  int           // 超时（秒）
    UseProxy bool          // 是否走代理
}
```

---

## 🔧 导出函数

| 函数 | 说明 |
|------|------|
| `QueryIP(ip string) (*IPWhoisResult, error)` | 简化入口 |
| `QueryIPWithOptions(opts IPWhoisOptions) (*IPWhoisResult, error)` | 带选项 |
| `QueryIPWithContext(ctx, opts) (*IPWhoisResult, error)` | 带上下文（**主流程**） |

---

## 🔍 关键实现要点

::: details 三步查询流程
`QueryIPWithContext` 执行以下三步：

1. **IANA 引导** — `client.rawWhoisQuery(ctx, "whois.iana.org", ip)` 获取引导响应
2. **提取 RIR** — `extractReferralServer` 从引导响应中提取 RIR 服务器
3. **RIR 查询** — 向提取到的 RIR 服务器查询 IP

若第 2 步提取不到 RIR，则直接返回 IANA 的响应。
:::

::: details extractReferralServer
从 IANA 响应中匹配以下模式提取 RIR：

- `Registrar WHOIS Server: `（域名风格）
- `whois: `（IP 风格，IANA 响应用此格式）

提取后去除空白与端口后缀。
:::

::: details 解析尝试
查询完成后调用 `whoisparser.Parse` 尝试解析，但通常失败（IP 响应格式与域名不同）。即使失败也保留 `RawResponse` 供 [ipparser.md](./ipparser.md) 进一步解析。
:::

::: details 代理支持
`UseProxy=true` 时，`client.pool = GetProxyPool()`，后续 `rawWhoisQuery` 走代理拨号。代理池使用见 [proxy.md](./proxy.md)。
:::

---

## 📝 使用示例

### 示例 1：基础查询

```go
result, _ := whois.QueryIP("8.8.8.8")
fmt.Println("服务器：", result.Server)
fmt.Println("延迟：", result.Latency)
// 进一步结构化解析
info, _ := whois.ParseIPWhois(result.RawResponse, "8.8.8.8")
fmt.Println("组织：", info.Organization.Name)
```

### 示例 2：带超时与代理

```go
ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

result, err := whois.QueryIPWithContext(ctx, whois.IPWhoisOptions{
    IP:       "1.1.1.1",
    Timeout:  15,
    UseProxy: true,
})
```

### 示例 3：批量 IP 查询

```go
ips := []string{"8.8.8.8", "8.8.4.4", "1.1.1.1", "9.9.9.9"}
for _, ip := range ips {
    r, err := whois.QueryIP(ip)
    if err != nil {
        fmt.Printf("%s: 错误 %v\n", ip, err)
        continue
    }
    fmt.Printf("%s via %s (%dms)\n", ip, r.Server, r.Latency)
}
```

---

## ⚠️ 注意事项

- IANA 引导查询增加一次额外往返，整体延迟高于域名查询。
- IPv6 地址查询流程相同，IANA 会引导到对应 RIR（通常 APNIC/ARIN）。
- 如需更结构化的 IP 信息，建议直接使用 [rdap.md](./rdap.md) 的 `QueryRDAP_IP`。

---

## 🔗 相关

- 🌐 [ipparser.md](./ipparser.md) — IP 响应结构化解析
- 📡 [rdap.md](./rdap.md) — RDAP IP 查询（现代替代）
- 🔎 [query.md](./query.md) — 域名查询引擎
