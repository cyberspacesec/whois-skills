# 🔗 关联端点 — POST /api/correlation

> 📖 域名关联分析端点，对一组域名（至少 2 个）逐个查询 WHOIS 后，通过 `whois.NewCorrelationEngine` 聚合分析其注册商、注册人、DNS 等关联关系。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/correlation` |
| 方法 | `POST` |
| 处理器 | `handleCorrelation` |
| Content-Type | `application/json` |
| 底层函数 | `whois.ExecuteQueryWithContext` + `whois.NewCorrelationEngine` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domains` | `[]string` | 是 | 域名列表，**至少 2 个** |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/correlation \
  -H "Content-Type: application/json" \
  -d '{"domains": ["example.com", "example.org", "example.net"]}'
```

### 请求示例

```json
{
  "domains": ["example.com", "example.org", "example.net"]
}
```

---

## ⚙️ 处理流程

1. 创建关联引擎：`engine := whois.NewCorrelationEngine()`
2. 逐个查询域名 WHOIS：
   - 调用 `whois.ExecuteQueryWithContext`
   - 查询失败则记日志告警（`logrus.Warnf`）并 `continue`，不中断流程
   - 成功则 `engine.AddDomain(domain, info)`
3. 执行分析：`result := engine.Analyze()`
4. 返回分析结果

::: tip 容错策略
单个域名查询失败不会终止整个关联分析，失败域名被跳过，仅记录告警日志。
:::

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": {
    "total_domains": 3,
    "analyzed_domains": 3,
    "shared_registrars": ["RESERVED-IANA"],
    "shared_name_servers": ["a.iana-servers.net", "b.iana-servers.net"],
    "relationships": [
      {
        "type": "shared_registrar",
        "domains": ["example.com", "example.org", "example.net"],
        "value": "RESERVED-IANA"
      }
    ],
    "score": 0.85
  }
}
```

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | 域名数量 < 2 | `至少需要2个域名进行关联分析` |

::: warning 注意
关联端点不会因单个域名查询失败返回 500，失败域名被静默跳过。若所有域名均查询失败，`Analyze()` 仍会返回结果（空关联）。
:::

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔄 [endpoint-diff.md](./endpoint-diff.md) — 两域名对比端点
