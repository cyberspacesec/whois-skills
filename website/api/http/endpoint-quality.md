# 📊 质量端点 — POST /api/quality

> 📖 WHOIS 数据质量评估端点，查询域名 WHOIS 后调用 `whois.AssessQuality`，从完整性、时效性、可靠性等维度评分。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 路径 | `/api/quality` |
| 方法 | `POST` |
| 处理器 | `handleQuality` |
| Content-Type | `application/json` |
| 底层函数 | `whois.ExecuteQueryWithContext` + `whois.AssessQuality` |

---

## 📝 请求

### 请求体字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain` | `string` | 是 | 待评估域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/quality \
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

```json
{
  "success": true,
  "data": {
    "total_score": 85,
    "completeness": 90,
    "timeliness": 80,
    "reliability": 85,
    "level": "good"
  }
}
```

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `total_score` | `int` | 综合评分 |
| `completeness` | `int` | 完整性评分 |
| `timeliness` | `int` | 时效性评分 |
| `reliability` | `int` | 可靠性评分 |
| `level` | `string` | 质量等级（见下表） |

### level 取值

| level | 含义 |
|-------|------|
| `excellent` | 优秀 |
| `good` | 良好 |
| `fair` | 一般 |
| `poor` | 较差 |
| `critical` | 严重不足 |

下图展示质量评估从 WHOIS 查询到多维评分再到等级判定的处理流程。

```mermaid
flowchart TD
  Req([🌐 POST /api/quality<br/>{domain}]) --> MW[🛡️ 中间件链]
  MW --> V{🔍 domain 非空?}
  V -- 否 --> E[❌ 400 域名不能为空]
  V -- 是 --> Q[🔎 whois.ExecuteQueryWithContext]
  Q --> R{查询成功?}
  R -- 否 --> E2[❌ 500 查询失败]
  R -- 是 --> A[📊 whois.AssessQuality]

  subgraph Score[📐 评分维度]
    SC1[完整性 completeness]
    SC2[时效性 timeliness]
    SC3[可靠性 reliability]
  end

  A --> Score
  Score --> Total[🎯 total_score 综合评分]
  Total --> L{📐 等级判定}
  L --> L1[🟢 excellent]
  L --> L2[🟢 good]
  L --> L3[🟡 fair]
  L --> L4[🟠 poor]
  L --> L5[🔴 critical]

  E & E2 & L1 & L2 & L3 & L4 & L5 --> Resp([📤 HTTP 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp entry
  class MW,Q,A,SC1,SC2,SC3,Total svc
  class V,R,L check
  class E,E2,L5 err
  class L1,L2 entry
  class L3,L4 svc
```

---

## ❌ 错误码

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `500` | 查询失败 | `查询失败: <err>` |

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📑 [endpoints.md](./endpoints.md) — 端点总览
- 🔎 [endpoint-whois.md](./endpoint-whois.md) — WHOIS 查询端点
