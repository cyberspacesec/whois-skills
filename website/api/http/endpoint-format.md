# 📝 格式化端点 — POST /api/format

> 📖 WHOIS 原始响应的格式检测与格式化端点，调用 `whois.DetectWhoisFormat` 识别格式，可选附加 `whois.FormatRawResponse` 的格式化结果。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/format` |
| 方法 | `POST` |
| 处理器 | `handleFormat` |
| Content-Type | `application/json` |
| 底层函数 | `whois.DetectWhoisFormat` / `whois.FormatRawResponse` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `raw_response` | `string` | 是 | — | WHOIS 原始响应文本 |
| `detect_only` | `bool` | 否 | `false` | 是否仅检测格式，不附加格式化结果 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/format \
  -H "Content-Type: application/json" \
  -d '{
    "raw_response": "Domain Name: EXAMPLE.COM\nRegistrar: RESERVED-IANA\n...",
    "detect_only": false
  }'
```

### 请求示例

```json
{
  "raw_response": "Domain Name: EXAMPLE.COM\nRegistrar: RESERVED-IANA\n...",
  "detect_only": false
}
```

---

## ⚙️ 处理逻辑

```go
format := whois.DetectWhoisFormat(req.RawResponse)
result := map[string]interface{}{"format": format}
if !req.DetectOnly {
    result["formatted"] = whois.FormatRawResponse(req.RawResponse)
}
```

| `detect_only` | 返回字段 |
|---------------|----------|
| `true` | 仅 `format` |
| `false` | `format` + `formatted` |

---

## ✅ 响应示例

### detect_only = false

```json
{
  "success": true,
  "data": {
    "format": "iana",
    "formatted": "Domain Name: example.com\nRegistrar: RESERVED-IANA\n..."
  }
}
```

### detect_only = true

```json
{
  "success": true,
  "data": {
    "format": "iana"
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `format` | `string` | 检测到的格式类型 |
| `formatted` | `string` | 格式化后的文本（`detect_only=false` 时存在） |

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `raw_response` 为空 | `原始响应不能为空` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 📤 [endpoint-export.md](./endpoint-export.md) — 导出端点
