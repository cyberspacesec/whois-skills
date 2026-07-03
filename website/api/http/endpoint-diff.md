# 🔄 对比端点 — POST /api/diff

> 📖 WHOIS 对比端点，分别查询两个域名的 WHOIS 信息后调用 `whois.CompareWhois`，输出差异变更列表。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/diff` |
| 方法 | `POST` |
| 处理器 | `handleDiff` |
| Content-Type | `application/json` |
| 底层函数 | `whois.ExecuteQueryWithContext` + `whois.CompareWhois` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain1` | `string` | 是 | 第一个域名 |
| `domain2` | `string` | 是 | 第二个域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/diff \
  -H "Content-Type: application/json" \
  -d '{"domain1": "example.com", "domain2": "example.org"}'
```

### 请求示例

```json
{
  "domain1": "example.com",
  "domain2": "example.org"
}
```

---

## ✅ 响应示例

```json
{
  "success": true,
  "data": {
    "domain1": "example.com",
    "domain2": "example.org",
    "changes": [
      {
        "field": "registrar",
        "domain1_value": "RESERVED-IANA",
        "domain2_value": "RESERVED-IANA",
        "type": "modified"
      },
      {
        "field": "created_date",
        "domain1_value": "1995-08-14T04:00:00Z",
        "domain2_value": "",
        "type": "added"
      }
    ],
    "count": 2
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `domain1` | `string` | 第一个域名 |
| `domain2` | `string` | 第二个域名 |
| `changes` | `[]Change` | 差异变更列表 |
| `count` | `int` | 差异数量（`len(changes)`） |

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain1` 或 `domain2` 为空 | `两个域名都不能为空` |
| `500` | 查询 `domain1` 失败 | `查询 <domain1> 失败: <err>` |
| `500` | 查询 `domain2` 失败 | `查询 <domain2> 失败: <err>` |

::: tip 串行查询
端点依次查询 `domain1`、`domain2`，任一查询失败即返回 500，不会继续对比。
:::

下图展示 diff 端点串行查询两个域名、任一失败即中断的处理流程。

```mermaid
flowchart TD
  Req([🌐 POST /api/diff<br/>{domain1, domain2}]) --> MW[🛡️ 中间件链]
  MW --> V{🔍 两域名均非空?}
  V -- 否 --> E0[❌ 400 两个域名都不能为空]
  V -- 是 --> Q1[🔎 查询 domain1<br/>ExecuteQueryWithContext]
  Q1 --> R1{成功?}
  R1 -- 否 --> E1[❌ 500 查询 domain1 失败]
  R1 -- 是 --> Q2[🔎 查询 domain2<br/>ExecuteQueryWithContext]
  Q2 --> R2{成功?}
  R2 -- 否 --> E2[❌ 500 查询 domain2 失败]
  R2 -- 是 --> Cmp[🔄 whois.CompareWhois<br/>生成 changes 列表]
  Cmp --> Resp([✅ 200 返回差异])

  E0 & E1 & E2 --> Resp

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class MW,Q1,Q2,Cmp svc
  class V,R1,R2 check
  class E0,E1,E2 err
```

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — 单域名 WHOIS 查询
