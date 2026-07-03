# 🌐 IP 查询教程

> 📖 查询 IP 地址的 WHOIS 信息，理解 IANA 引导与 RIR 定位。

---

## 1️⃣ 基础查询

```go
package main

import (
	"fmt"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

func main() {
	result, err := whois.QueryIP("8.8.8.8")
	if err != nil {
		panic(err)
	}

	fmt.Printf("查询服务器: %s\n", result.Server)
	fmt.Printf("延迟: %d ms\n", result.Latency)
	fmt.Printf("原始响应:\n%s\n", result.RawResponse)
}
```

`QueryIP` 内部完成三步：
1. 查询 `whois.iana.org` 获取引导
2. `extractReferralServer` 提取 RIR 服务器
3. 查询 RIR 服务器获取详细响应

---

## 2️⃣ 带选项查询

```go
result, err := whois.QueryIPWithOptions(&whois.IPWhoisOptions{
	IP:       "8.8.8.8",
	Timeout:  10,
	UseProxy: false,
})
```

### IPWhoisOptions 字段

| 字段 | 类型 | 默认 | 说明 |
|------|------|------|------|
| `IP` | string | - | IP 地址（必填） |
| `Timeout` | int | 10 | 超时（秒） |
| `UseProxy` | bool | false | 是否走代理 |

---

## 3️⃣ Context 版本（支持超时取消）

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

result, err := whois.QueryIPWithContext(ctx, &whois.IPWhoisOptions{
	IP:      "2001:4860:4860::8888",
	Timeout: 15,
})
```

---

## 4️⃣ 结构化解析

IP WHOIS 响应各 RIR 格式不一，用 `ParseIPWhois` 统一解析：

```go
result, _ := whois.QueryIP("8.8.8.8")

info, err := whois.ParseIPWhois(result.RawResponse, "8.8.8.8")
if err != nil {
	panic(err)
}

fmt.Printf("RIR: %s\n", info.RIR)              // ARIN
fmt.Printf("CIDR: %s\n", info.Network.CIDR)    // 8.8.8.0/24
fmt.Printf("网段名: %s\n", info.Network.Name)
fmt.Printf("国家: %s\n", info.Network.Country)
fmt.Printf("组织: %s\n", info.Organization.Name)
if info.ASN != nil {
	fmt.Printf("ASN: AS%d (%s)\n", info.ASN.Number, info.ASN.Name)
}
```

### IPWhoisInfo 结构

```go
type IPWhoisInfo struct {
    Query        string         // 查询的 IP
    Network      *IPNetwork     // 网段信息
    Organization *IPOrganization // 组织信息
    Contacts     []*IPContact   // 联系人
    ASN          *ASNInfo       // ASN 信息
    RIR          string         // ARIN/RIPE/APNIC/LACNIC/AFRINIC
    RawResponse  string         // 原始响应
}
```

📖 详见 [ipparser.go 文档](../api/whois/ipparser.md)。

---

## 5️⃣ 五大 RIR 自动识别

`detectRIR` 自动识别响应来自哪个 RIR：

| RIR | 覆盖区域 | 关键词 |
|-----|---------|--------|
| 🇺🇸 ARIN | 北美 | "american registry" |
| 🇪🇺 RIPE | 欧洲/中东/中亚 | "ripe network" |
| 🌏 APNIC | 亚太 | "asia pacific" |
| 🌎 LACNIC | 拉美 | "lacnic" |
| 🌍 AFRINIC | 非洲 | "afrinic" |

每个 RIR 有独立解析器，最终统一到 `IPWhoisInfo`。

---

## 6️⃣ RDAP 查询 IP

RDAP 是 WHOIS 的现代替代协议，返回结构化 JSON：

```go
result, err := whois.QueryRDAP_IP("8.8.8.8")
if err != nil {
	panic(err)
}

fmt.Printf("CIDR: %s\n", result.CIDR)
fmt.Printf("起始: %s\n", result.StartAddress)
fmt.Printf("结束: %s\n", result.EndAddress)
fmt.Printf("国家: %s\n", result.Country)
fmt.Printf("名称: %s\n", result.Name)
```

📖 详见 [RDAP 文档](../api/whois/rdap.md)。

::: tip 💡 WHOIS vs RDAP
- **WHOIS**：文本协议，端口 43，格式不统一
- **RDAP**：HTTP+JSON，RFC 9083，结构化，推荐新项目使用
:::

---

## 7️⃣ HTTP API 调用

```bash
curl -X POST http://127.0.0.1:8080/api/ip \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","timeout":10}'
```

📖 详见 [IP 端点](../api/http/endpoint-ip.md)。

---

## ✅ 小结

| 需求 | 推荐方式 |
|------|---------|
| 简单查 IP | `QueryIP` |
| 带超时/代理 | `QueryIPWithOptions` |
| Context 控制 | `QueryIPWithContext` |
| 结构化解析 | `ParseIPWhois` |
| 现代 RDAP | `QueryRDAP_IP` |

---

## 🔗 下一步

- 🔢 [ASN 查询教程](./tutorial-asn.md)
- 🔬 [ipparser.go API](../api/whois/ipparser.md)
- 📡 [RDAP 文档](../api/whois/rdap.md)
