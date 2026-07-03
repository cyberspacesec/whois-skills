# ✅ 可用性端点 — POST /api/availability

> 📖 域名可用性检查端点，调用 `whois.CheckDomainAvailabilityWithContext` 判断域名是否可注册。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/availability` |
| 方法 | `POST` |
| 处理器 | `handleAvailabilityCheck` |
| Content-Type | `application/json` |
| 底层函数 | `whois.CheckDomainAvailabilityWithContext` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain` | `string` | 是 | 待检查域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/availability \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com"}'
```

### 请求示例

```json
{
  "domain": "example.com"
}
```

---

## ✅ 响应示例

### 已注册

```json
{
  "success": true,
  "data": {
    "domain": "example.com",
    "available": false,
    "status": "registered",
    "message": "域名已注册"
  }
}
```

### 可注册

```json
{
  "success": true,
  "data": {
    "domain": "my-new-domain-12345.com",
    "available": true,
    "status": "available",
    "message": "域名可注册"
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `domain` | `string` | 被检查的域名 |
| `available` | `bool` | 是否可注册 |
| `status` | `string` | 状态标识（见下表） |
| `message` | `string` | 状态说明 |

### status 取值

| status | 含义 |
|--------|------|
| `available` | 域名可注册 |
| `registered` | 域名已注册 |
| `reserved` | 域名被保留 |
| `error` | 检查失败或无法判断 |

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `500` | 检查失败 | `可用性检查失败: <err>` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — WHOIS 查询端点
