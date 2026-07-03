# 🌍 IDN 端点 — POST /api/idn

> 📖 国际化域名（IDN）转换端点，支持规范化、Unicode→Punycode、Punycode→Unicode、检测四种操作，始终返回原始域名与是否为 IDN 的标识。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/idn` |
| 方法 | `POST` |
| 处理器 | `handleIDN` |
| Content-Type | `application/json` |
| 底层函数 | `whois.NormalizeDomain` / `UnicodeToPunycode` / `PunycodeToUnicode` / `IsIDN` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `domain` | `string` | 是 | — | 待处理域名 |
| `action` | `string` | 否 | `normalize` | 操作类型（见下表） |

### action 取值

| action | 调用函数 | 说明 |
|--------|----------|------|
| `normalize`（默认） | `whois.NormalizeDomain` | 规范化域名 |
| `to_punycode` | `whois.UnicodeToPunycode` | Unicode 转 Punycode |
| `to_unicode` | `whois.PunycodeToUnicode` | Punycode 转 Unicode |
| `check` | — | 仅返回 `is_idn` 检测信息，不做转换 |

### curl 示例

```bash
# 转 Punycode
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain": "例子.测试", "action": "to_punycode"}'

# 检测是否为 IDN
curl -X POST http://127.0.0.1:8080/api/idn \
  -H "Content-Type: application/json" \
  -d '{"domain": "xn--fsq092h.com", "action": "check"}'
```

---

## ✅ 响应示例

### action = normalize

```json
{
  "success": true,
  "data": {
    "original": "例子.测试",
    "is_idn": true,
    "normalized": "例子.测试"
  }
}
```

### action = to_punycode

```json
{
  "success": true,
  "data": {
    "original": "例子.测试",
    "is_idn": true,
    "punycode": "xn--fsq092h.xn--3hzr70a"
  }
}
```

### action = to_unicode

```json
{
  "success": true,
  "data": {
    "original": "xn--fsq092h.xn--3hzr70a",
    "is_idn": true,
    "unicode": "例子.测试"
  }
}
```

### action = check

```json
{
  "success": true,
  "data": {
    "original": "example.com",
    "is_idn": false
  }
}
```

### 响应字段

| 字段 | 类型 | 始终返回 | 说明 |
|------|------|----------|------|
| `original` | `string` | 是 | 原始输入域名 |
| `is_idn` | `bool` | 是 | 是否为 IDN（`whois.IsIDN`） |
| `normalized` | `string` | `normalize` 时 | 规范化结果 |
| `punycode` | `string` | `to_punycode` 时 | Punycode 结果 |
| `unicode` | `string` | `to_unicode` 时 | Unicode 结果 |

下图展示 IDN 端点根据 `action` 分派到不同转换函数的处理流程，`original` 与 `is_idn` 始终返回。

```mermaid
flowchart TD
  Req([🌐 POST /api/idn<br/>{domain, action}]) --> MW[🛡️ 中间件链]
  MW --> V{🔍 domain 非空?}
  V -- 否 --> E[❌ 400 域名不能为空]
  V -- 是 --> Idn[🔎 whois.IsIDN<br/>判定 is_idn]
  Idn --> Sw{🔀 action}
  Sw -- normalize/默认 --> N[🔧 NormalizeDomain]
  Sw -- to_punycode --> P[🔧 UnicodeToPunycode]
  Sw -- to_unicode --> U[🔧 PunycodeToUnicode]
  Sw -- check --> C[✅ 仅返回检测结果]
  Sw -- 非法 --> E2[❌ 400 无效的action]
  N & P & U & C --> Out[📦 组装响应<br/>original/is_idn + 转换结果]
  E & E2 & Out --> Resp([📤 HTTP 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class MW,Idn,N,P,U,C,Out svc
  class V,Sw check
  class E,E2 err
```

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `400` | `normalize` 失败 | `规范化失败: <err>` |
| `400` | `to_punycode` / `to_unicode` 失败 | `转换失败: <err>` |
| `400` | `action` 非法 | `无效的action，支持: normalize, to_punycode, to_unicode, check` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — WHOIS 查询端点
