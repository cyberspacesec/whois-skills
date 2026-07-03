# 🏷️ ASN 端点 — POST /api/asn

> 📖 ASN（自治系统号）查询端点，调用 `whois.QueryASNWithContext` 获取自治系统的注册信息、前缀与 BGP 数据，支持指定数据源。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/asn` |
| 方法 | `POST` |
| 处理器 | `handleASNQuery` |
| Content-Type | `application/json` |
| 底底函数 | `whois.QueryASNWithContext` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `asn` | `int` | 是 | — | ASN 编号，必须为正整数 |
| `timeout` | `int` | 否 | `0` | 超时（秒） |
| `source` | `string` | 否 | `all` | 数据源：`radb` / `rdap` / `all` |
| `include_prefixes` | `bool` | 否 | `false` | 是否包含前缀列表 |
| `include_bgp` | `bool` | 否 | `false` | 是否包含 BGP 信息 |

### source 取值映射

| 请求值 | 内部常量 | 说明 |
|--------|----------|------|
| `"radb"` | `whois.ASNSourceRADB` | 仅 RADB 数据源 |
| `"rdap"` | `whois.ASNSourceRDAP` | 仅 RDAP 数据源 |
| 其他 / 空 / `"all"` | `whois.ASNSourceAll` | 全部数据源（默认） |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/asn \
  -H "Content-Type: application/json" \
  -d '{
    "asn": 15169,
    "timeout": 15,
    "source": "radb",
    "include_prefixes": true,
    "include_bgp": true
  }'
```

### 请求示例

```json
{
  "asn": 15169,
  "timeout": 15,
  "source": "radb",
  "include_prefixes": true,
  "include_bgp": true
}
```

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": {
    "asn": "15169",
    "organization": "GOOGLE",
    "country": "US",
    "prefixes": ["8.8.8.0/24", "8.8.4.0/24"],
    "bgp": {
      "peers": 100
    }
  }
}
```

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `asn <= 0` | `ASN必须为正整数` |
| `500` | 查询失败 | `ASN查询失败: <err>` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — 域名 WHOIS 端点
