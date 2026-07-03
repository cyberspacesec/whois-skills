# 🌐 IP 端点 — POST /api/ip

> 📖 IP 地址 WHOIS 查询端点，调用 `whois.QueryIPWithOptions` 获取 IP 的注册信息、网络段与所属组织等。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/ip` |
| 方法 | `POST` |
| 处理器 | `handleIPQuery` |
| Content-Type | `application/json` |
| 底层函数 | `whois.QueryIPWithOptions` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `ip` | `string` | 是 | — | IP 地址（IPv4/IPv6） |
| `timeout` | `int` | 否 | `0` | 超时（秒） |
| `use_proxy` | `bool` | 否 | `false` | 是否走代理 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/ip \
  -H "Content-Type: application/json" \
  -d '{"ip": "8.8.8.8", "timeout": 15}'
```

### 请求示例

```json
{
  "ip": "8.8.8.8",
  "timeout": 15,
  "use_proxy": false
}
```

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": {
    "ip": "8.8.8.8",
    "range": "8.8.8.0 - 8.8.8.255",
    "cidr": "8.8.8.0/24",
    "asn": "15169",
    "organization": "GOOGLE",
    "netname": "LVLT-GOGL-8-8-8",
    "country": "US"
  }
}
```

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `ip` 为空 | `IP地址不能为空` |
| `500` | 查询失败 | `IP查询失败: <err>` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — 域名 WHOIS 端点
