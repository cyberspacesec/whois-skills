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

下图展示可用性检查的判定流程与 `status` 各取值的产生路径。

```mermaid
flowchart TD
  Req([🌐 POST /api/availability<br/>{domain}]) --> MW[🛡️ 中间件链]
  MW --> V{🔍 domain 非空?}
  V -- 否 --> E[❌ 400 域名不能为空]
  V -- 是 --> Q[🔎 whois.CheckDomainAvailabilityWithContext]
  Q --> R{查询结果}
  R -- 可注册 --> S1[✅ status=available]
  R -- 已注册 --> S2[🟦 status=registered]
  R -- 被保留 --> S3[🟧 status=reserved]
  R -- 失败/无法判断 --> S4[❌ status=error]
  E & S1 & S2 & S3 & S4 --> Resp([📤 HTTP 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp,S1 entry
  class MW,Q svc
  class V,R check
  class E,S4 err
  class S2,S3 svc
```

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
