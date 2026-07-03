# 🌐 RDAP 端点

> 📖 RDAP（Registration Data Access Protocol）查询端点，提供域名、IP、ASN 三类对象的现代化注册数据查询，分别对应三个独立端点。

---

## 📋 概览

| 路径 | 方法 | 处理器 | 底层函数 |
|------|------|--------|----------|
| `/api/rdap/domain` | POST | `handleRDAPDomainQuery` | `whois.QueryRDAPWithContext` |
| `/api/rdap/ip` | POST | `handleRDAPIPQuery` | `whois.QueryRDAP_IPWithContext` |
| `/api/rdap/asn` | POST | `handleRDAPASNQuery` | `whois.QueryRDAP_ASNWithContext` |

::: tip RDAP vs WHOIS
RDAP 是 WHOIS 的现代替代协议，返回结构化 JSON 数据，支持国际化与_BOOTSTRAP_路由。三个端点共享 `RDAPQueryOptions` 结构。
:::

---

## ① POST /api/rdap/domain — 域名 RDAP 查询

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `domain` | `string` | 是 | — | 待查询域名 |
| `timeout` | `int` | 否 | `0` | 超时（秒） |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/rdap/domain \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com", "timeout": 15}'
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "objectClassName": "domain",
    "ldhName": "EXAMPLE.COM",
    "events": [
      {"eventAction": "registration", "eventDate": "1995-08-14T04:00:00Z"}
    ],
    "nameservers": [
      {"ldhName": "A.IANA-SERVERS.NET"},
      {"ldhName": "B.IANA-SERVERS.NET"}
    ]
  }
}
```

### 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `500` | 查询失败 | `RDAP域名查询失败: <err>` |

---

## ② POST /api/rdap/ip — IP RDAP 查询

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `ip` | `string` | 是 | — | IP 地址（IPv4/IPv6） |
| `timeout` | `int` | 否 | `0` | 超时（秒） |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/rdap/ip \
  -H "Content-Type: application/json" \
  -d '{"ip": "8.8.8.8", "timeout": 15}'
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "objectClassName": "ip network",
    "cidr0_cidrs": [{"v4Prefix": "8.8.8.0", "length": 24}],
    "startAddress": "8.8.8.0",
    "endAddress": "8.8.8.255"
  }
}
```

### 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `ip` 为空 | `IP地址不能为空` |
| `500` | 查询失败 | `RDAP IP查询失败: <err>` |

---

## ③ POST /api/rdap/asn — ASN RDAP 查询

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `asn` | `string` | 是 | — | ASN（字符串形式，如 `"15169"`） |
| `timeout` | `int` | 否 | `0` | 超时（秒） |

::: warning 注意类型
此处 `asn` 为**字符串**类型（与 `/api/asn` 的 `int` 不同），如 `"AS15169"` 或 `"15169"`。
:::

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/rdap/asn \
  -H "Content-Type: application/json" \
  -d '{"asn": "AS15169", "timeout": 15}'
```

### 响应示例

```json
{
  "success": true,
  "data": {
    "objectClassName": "autnum",
    "handle": "AS15169",
    "name": "GOOGLE",
    "events": [
      {"eventAction": "registration", "eventDate": "2000-03-30T00:00:00Z"}
    ]
  }
}
```

### 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `asn` 为空 | `ASN不能为空` |
| `500` | 查询失败 | `RDAP ASN查询失败: <err>` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🏷️ [endpoint-asn.md](./endpoint-asn.md) — WHOIS ASN 端点
