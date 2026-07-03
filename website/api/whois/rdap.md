# 📡 rdap.go — RDAP 查询客户端

> 📖 RDAP（Registration Data Access Protocol，RFC 9083）查询客户端，支持域名、IP、ASN、Entity 四类对象查询，内置 bootstrap映射，是现代 WHOIS 替代方案。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/rdap.go` |
| 核心职责 | RDAP 查询（域名/IP/ASN/Entity） |
| 协议 | RFC 9083（JSON over HTTPS） |
| 引导 | 内置 bootstrap 映射（TLD/IP/ASN） |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

// 域名
r, _ := whois.QueryRDAP("example.com")
fmt.Println("状态：", r.Status)
fmt.Println("注册商：", r.Entities)

// IP
ipr, _ := whois.QueryRDAP_IP("8.8.8.8")
fmt.Println("CIDR：", ipr.CIDR, "国家：", ipr.Country)

// ASN
asnr, _ := whois.QueryRDAP_ASN("AS13335")
fmt.Println("AS 名称：", asnr.Name, "国家：", asnr.Country)
```

---

## 📊 核心类型

### RDAPResult（域名）

```go
type RDAPResult struct {
    RawJSON        interface{}    // 原始 JSON
    ObjectClassName string        // 对象类型
    LDHName        string         // ASCII 域名
    UnicodeName    string         // Unicode 域名
    Status         []string       // 状态
    Nameservers    []RDAPNameserver
    Events         []RDAPEvent
    Entities       []RDAPEntity
    Links          []RDAPLink
    Remarks        []RDAPRemark
    QueryTime      time.Time
    Server         string         // 查询的 RDAP 服务器
}
```

### 相关子结构

```go
type RDAPEvent struct {
    EventAction string // registration/expiration/last changed
    EventDate   string
    EventActor  string
}

type RDAPEntity struct {
    Roles      []string        // registrar/registrant/admin/tech/abuse
    VCardArray []interface{}   // vCard 数据
    PublicIDs  []RDAPPublicID
}

type RDAPNameserver struct {
    LDHName     string
    IPAddresses *RDAPIPAddrs
}

type RDAPIPAddrs struct {
    V4 []string
    V6 []string
}

type RDAPLink struct {
    Rel  string
    Href string
    Type string
}

type RDAPRemark struct {
    Title       string
    Description []string
}

type RDAPPublicID struct {
    Type  string
    Value string
}
```

### RDAPQueryOptions

```go
type RDAPQueryOptions struct {
    Domain       string
    IP           string
    ASN          string
    EntityHandle string
    Timeout      int
    HTTPClient   *http.Client // 自定义 HTTP 客户端
}
```

### IP 结果

```go
type RDAPIPResult struct {
    StartAddress  string
    EndAddress    string
    CIDR          string
    IPVersion     string
    Type          string
    Name          string
    Country       string
    ParentHandle  string
    // ...
}
```

### ASN 结果

```go
type RDAPASNResult struct {
    ASN         int
    Handle      string
    Name        string
    Country     string
    Type        string
    StartAutnum int
    EndAutnum   int
    // ...
}
```

### RDAPBootstrap

```go
type RDAPBootstrap struct {
    dns         map[string]string   // TLD → RDAP URL
    ipRanges    []ipRange           // IP 段 → RIR
    asnRanges   []asnRange          // ASN 区间 → RIR
    lastUpdated time.Time
    loaded      bool
}
```

---

## 🔧 函数与方法

### Bootstrap

| 函数/方法 | 说明 |
|-----------|------|
| `GetRDAPBootstrap() *RDAPBootstrap` | 全局单例（`loadDefaults`） |
| `GetDNSServer(tld) string` | 按 TLD 查 RDAP 服务器 |
| `GetASN_RDAPServer(asn int) string` | 按 ASN 查 RDAP 服务器 |

### 域名查询

| 函数 | 说明 |
|------|------|
| `QueryRDAP(domain) (*RDAPResult, error)` | 域名查询 |
| `QueryRDAPWithContext(ctx, opts) (*RDAPResult, error)` | 带上下文 |

### IP 查询

| 函数 | 说明 |
|------|------|
| `QueryRDAP_IP(ip) (*RDAPIPResult, error)` | IP 查询 |
| `QueryRDAP_IPWithContext(ctx, opts) (*RDAPIPResult, error)` | 带上下文 |

### ASN 查询

| 函数 | 说明 |
|------|------|
| `QueryRDAP_ASN(asn) (*RDAPASNResult, error)` | ASN 查询 |
| `QueryRDAP_ASNWithContext(ctx, opts) (*RDAPASNResult, error)` | 带上下文 |

### Entity 查询

| 函数 | 说明 |
|------|------|
| `QueryRDAP_Entity(handle) (*RDAPEntityResult, error)` | Entity 查询 |
| `QueryRDAP_EntityWithContext(ctx, opts) (*RDAPEntityResult, error)` | 带上下文 |

---

## 🔍 关键实现要点

::: details loadDefaults 内置映射
`loadDefaults` 初始化三张映射表：

- **DNS map** — 40+ TLD → RDAP URL（com/net/org/info/biz/io/co/ai/cn/uk/de/fr/eu/au/jp/br/ru 等）
- **IP ranges** — 150+ 个 /8 CIDR → RIR
- **ASN ranges** — 100+ 个 ASN 区间 → RIR
:::

::: details discoverRDAPServer 域名引导
1. 从域名提取 TLD
2. 查 `dns map`，命中则返回
3. 未命中回退到 `https://rdap.iana.org/{tld}` 查询官方 bootstrap
4. 拼接 `{base}/domain/{domain}` 作为查询 URL
:::

::: details discoverIP_RDAPServer IP 引导
1. 遍历 `ipRanges`
2. 用 `net.ParseCIDR` + `ipNet.Contains(ip)` 判断 IP 所属
3. 未命中回退：IPv4 默认 ARIN，IPv6 默认 APNIC
:::

::: details discoverASN_RDAPServer ASN 引导
遍历 `asnRanges`，判断 ASN 是否落在某区间，返回对应 RIR 的 RDAP URL。

:::

::: details discoverEntityRDAPServer Entity 引导
按 handle 后缀（如 `-ARIN`）映射 RIR，默认 ARIN。

:::

::: details rdapHTTPRequest
HTTP 请求设置：

- `Accept: application/rdap+json`（RDAP 标准 MIME）
- 非 200 状态码返回错误
- 响应体 JSON unmarshal 到对应结果结构
:::

---

## 📝 使用示例

### 示例 1：域名 RDAP 查询

```go
r, _ := whois.QueryRDAP("example.com")
fmt.Println("状态:", r.Status)
for _, e := range r.Entities {
    if contains(e.Roles, "registrar") {
        fmt.Println("注册商:", e)
    }
}
for _, ev := range r.Events {
    fmt.Printf("%s: %s\n", ev.EventAction, ev.EventDate)
}
```

### 示例 2：IP 查询

```go
ipr, _ := whois.QueryRDAP_IP("8.8.8.8")
fmt.Printf("CIDR: %s, 国家: %s, 类型: %s\n",
    ipr.CIDR, ipr.Country, ipr.Type)
```

### 示例 3：ASN 查询

```go
asnr, _ := whois.QueryRDAP_ASN("AS13335")
fmt.Printf("AS%d %s (%s)\n", asnr.ASN, asnr.Name, asnr.Country)
```

### 示例 4：带超时与自定义客户端

```go
r, _ := whois.QueryRDAPWithContext(ctx, &whois.RDAPQueryOptions{
    Domain: "example.com",
    Timeout: 15,
    HTTPClient: &http.Client{Timeout: 15 * time.Second},
})
```

---

## ⚠️ 注意事项

- RDAP 是 WHOIS 的现代替代，但并非所有 TLD 都已部署 RDAP 服务。
- RDAP 返回的数据结构与 WHOIS 不同，更结构化但字段名有差异。
- 与传统 WHOIS 相比，RDAP 走 HTTPS，延迟更高但数据更可靠。

---

## 🔗 相关

- 📡 [ipwhois.md](./ipwhois.md) — 传统 IP WHOIS 查询
- 🚀 [asn-enhanced.md](./asn-enhanced.md) — 增强 ASN 查询（使用 RDAP）
- 🌐 [asn.md](./asn.md) — RADB 前缀查询
