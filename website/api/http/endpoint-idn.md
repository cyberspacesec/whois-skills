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
