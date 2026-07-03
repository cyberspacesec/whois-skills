# 📤 导出端点

> 📖 WHOIS 数据导出端点，提供 JSON / CSV / Markdown 三种格式。**注意：这三个端点直接返回文件内容字节，不使用 `APIResponse` 包装。**

---

## 📋 概览

| 路径 | 方法 | 处理器 | 底层函数 | Content-Type |
|------|------|--------|----------|--------------|
| `/api/export/json` | POST | `handleExportJSON` | `whois.ExportToJSON` | `application/json` |
| `/api/export/csv` | POST | `handleExportCSV` | `whois.ExportToCSV` | `text/csv` |
| `/api/export/markdown` | POST | `handleExportMarkdown` | `whois.ExportToMarkdown` | `text/markdown` |

::: warning 非 APIResponse 包装
三个导出端点直接 `w.Write(buf.Bytes())` 返回原始字节流，**不**经过 `SendSuccessResponse`，响应体不是 `{success, data}` 结构。查询失败时仍走 `SendErrorResponse` 返回 APIResponse。
:::

---

## ① POST /api/export/json

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain` | `string` | 是 | 待导出域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/export/json \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com"}' \
  -o example.json
```

### 响应

直接返回 WHOIS 数据的 JSON 字节流，`Content-Type: application/json`。响应体即结构化 WHOIS 数据本身：

```json
{
  "domain": "example.com",
  "registrar": "RESERVED-IANA",
  "created_date": "1995-08-14T04:00:00Z"
}
```

---

## ② POST /api/export/csv

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain` | `string` | 是 | 待导出域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/export/csv \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com"}' \
  -o example.csv
```

### 响应

响应头：

```
Content-Type: text/csv
Content-Disposition: attachment; filename=example.com.csv
```

响应体为 CSV 文本，浏览器会触发文件下载，文件名为 `<domain>.csv`。

---

## ③ POST /api/export/markdown

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `domain` | `string` | 是 | 待导出域名 |

### curl 示例

```bash
curl -X POST http://127.0.0.1:8080/api/export/markdown \
  -H "Content-Type: application/json" \
  -d '{"domain": "example.com"}' \
  -o example.md
```

### 响应

响应头：

```
Content-Type: text/markdown
```

响应体为 Markdown 文本。

---

## ⚙️ 通用处理流程

三个端点共享相同流程：

1. 解析请求体，校验 `domain` 非空
2. `whois.ExecuteQueryWithContext` 查询 WHOIS
3. 写入 `bytes.Buffer`：`whois.ExportToXxx(info, &buf)`
4. 设置 `Content-Type`（CSV 额外设置 `Content-Disposition`）
5. `w.Write(buf.Bytes())` 输出原始字节

下图展示三个导出端点共享的处理流程，成功时直接返回原始字节（非 APIResponse），失败时才走错误响应封装。

```mermaid
flowchart TD
  Req([🌐 POST /api/export/{json,csv,markdown}<br/>{domain}]) --> MW[🛡️ 中间件链]
  MW --> V1{⚙️ 校验<br/>方法/JSON/domain?}
  V1 -- 失败 --> E1[❌ 400/405<br/>SendErrorResponse]
  V1 -- 通过 --> Q[🔎 whois.ExecuteQueryWithContext]
  Q --> V2{查询成功?}
  V2 -- 否 --> E2[❌ 500 查询失败<br/>APIResponse]
  V2 -- 是 --> Exp[📦 whois.ExportToXxx<br/>写入 bytes.Buffer]
  Exp --> V3{导出成功?}
  V3 -- 否 --> E3[❌ 500 导出失败<br/>APIResponse]
  V3 -- 是 --> CTF[🔧 设置 Content-Type<br/>CSV 额外设 Content-Disposition]
  CTF --> Raw[✅ w.Write 原始字节<br/>非 APIResponse]

  E1 & E2 & E3 & Raw --> Resp([📤 HTTP 响应])

  classDef entry fill:#41b883,color:#fff,stroke:#2b7a4b
  classDef svc fill:#647eff,color:#fff,stroke:#4a5fd6
  classDef check fill:#e6a23c,color:#fff,stroke:#b7821c
  classDef err fill:#f56c6c,color:#fff,stroke:#c04040

  class Req,Resp,Raw entry
  class MW,Q,Exp,CTF svc
  class V1,V2,V3 check
  class E1,E2,E3 err
```

---

## ❌ 错误码

三个端点错误码一致：

| HTTP 状态码 | 触发条件 | 错误信息 |
|------------|----------|----------|
| `405` | 非 POST 方法 | `仅支持POST请求` |
| `400` | JSON 解码失败 | `无效的请求格式` |
| `400` | `domain` 为空 | `域名不能为空` |
| `500` | 查询失败 | `查询失败: <err>` |
| `500` | 导出失败 | `导出失败: <err>` |

::: tip 错误响应仍为 APIResponse
查询或导出失败时返回的是标准 `APIResponse`（`success:false`），仅成功时返回原始文件字节。
:::

---

## 🔗 相关

- 🌐 [overview.md](./overview.md) — API 概览
- 📦 [response.md](./response.md) — 统一响应结构（导出端点例外）
- 📝 [endpoint-format.md](./endpoint-format.md) — 格式化端点
