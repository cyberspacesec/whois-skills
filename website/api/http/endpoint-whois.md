# 🔎 WHOIS 端点 — POST /api/whois

> 📖 域名 WHOIS 查询端点，调用 `whois.ExecuteQueryWithResult` 执行查询，可配置代理、超时、重试与结果校验，返回完整查询结果。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/whois` |
| 方法 | `POST` |
| 处理器 | `handleWhoisQuery` |
| Content-Type | `application/json` |
| 底层函数 | `whois.ExecuteQueryWithResult` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `domain` | `string` | 是 | — | 待查询域名 |
| `use_proxy` | `bool` | 否 | `false` | 是否走代理 |
| `timeout` | `int` | 否 | `10` | 超时（秒），`<=0` 时取 10 |
| `max_retries` | `int` | 否 | `0` | 最大重试次数 |
| `validate_result` | `bool` | 否 | `false` | 是否校验结果完整性 |
| `required_fields` | `[]string` | 否 | `[]` | 必填字段列表 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/whois \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "example.com",
    "timeout": 15,
    "max_retries": 5,
    "validate_result": true,
    "required_fields": ["registrar", "created_date"]
  }'
```

### 请求示例

```json
{
  "domain": "example.com",
  "timeout": 15,
  "max_retries": 5,
  "validate_result": true,
  "required_fields": ["registrar", "created_date"]
}
```

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": {
    "info": {
      "domain": "example.com",
      "registrar": "RESERVED-Internet Assigned Numbers Authority",
      "created_date": "1995-08-14T04:00:00Z",
      "updated_date": "2024-08-14T07:01:31Z",
      "expiry_date": "2025-08-13T04:00:00Z",
      "name_servers": ["a.iana-servers.net", "b.iana-servers.net"]
    },
    "raw_response": "Domain Name: EXAMPLE.COM\n...",
    "query_time": "2026-07-03T12:00:00Z",
    "latency": 850,
    "server": "whois.iana.org",
    "used_proxy": false,
    "retry_count": 0,
    "validation_result": {
      "valid": true,
      "missing_fields": [],
      "errors": []
    }
  }
}
```

### 响应 data 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `info` | `WhoisInfo` | 解析后的结构化 WHOIS 数据 |
| `raw_response` | `string` | 原始 WHOIS 文本 |
| `query_time` | `time.Time` | 查询时刻 |
| `latency` | `int64` | 延迟（毫秒） |
| `server` | `string` | 实际查询的服务器 |
| `used_proxy` | `bool` | 是否使用代理 |
| `retry_count` | `int` | 重试次数 |
| `validation_result` | `*ValidationResult` | 结果校验信息 |

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `500` | 查询失败 | `查询失败: <err>` |

---

## ⚙️ 指标记录

当 `s.EnableMetrics = true` 时，本端点会调用：

```go
metrics.GetCollector().RecordWHOISQuery(result.Server, err == nil, duration)
```

记录查询服务器、成功与否与耗时。

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🖥️ [server.md](./server.md) — 服务器配置 `EnableMetrics`
