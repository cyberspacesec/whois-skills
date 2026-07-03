# 📤 数据导出教程

> 📖 将 WHOIS 查询结果导出为 JSON、CSV、Markdown 三种格式。

---

## 1️⃣ 三种导出格式

| 格式 | 函数 | 适用场景 |
|------|------|---------|
| 📄 JSON | `ExportToJSON` | 程序间数据交换、API 集成 |
| 📊 CSV | `ExportToCSV` | Excel/表格软件查看、数据分析 |
| 📝 Markdown | `ExportToMarkdown` | 文档生成、报告分享 |

---

## 2️⃣ 基础导出

```go
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

func main() {
	result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{Domain: "example.com"})
	if err != nil {
		panic(err)
	}

	// 导出 JSON
	var buf bytes.Buffer
	whois.ExportToJSON(result.Info, &buf)
	os.WriteFile("example.json", buf.Bytes(), 0644)

	// 导出 CSV
	buf.Reset()
	whois.ExportToCSV(result.Info, &buf)
	os.WriteFile("example.csv", buf.Bytes(), 0644)

	// 导出 Markdown
	buf.Reset()
	whois.ExportToMarkdown(result.Info, &buf)
	os.WriteFile("example.md", buf.Bytes(), 0644)

	fmt.Println("导出完成")
}
```

三个函数签名均为 `func ExportToXxx(info *whoisparser.WhoisInfo, w io.Writer) error`，接受任意 `io.Writer`。

---

## 3️⃣ JSON 格式

缩进 2 空格的结构化 JSON：

```json
{
  "domain": {
    "domain": "example.com",
    "created_date": "1995-08-14T04:00:00Z",
    "expiration_date": "2025-08-13T04:00:00Z"
  },
  "registrar": {
    "name": "Reserved",
    "email": ""
  },
  "registrant": { "...": "..." }
}
```

---

## 4️⃣ CSV 格式

两列表（Field, Value），含域名与 5 个联系人区段：

```csv
Field,Value
domain,example.com
created_date,1995-08-14
expiration_date,2025-08-13
registrar_name,Reserved
registrant_name,...
registrant_email,...
administrative_name,...
technical_name,...
billing_name,...
```

联系人区段：Registrar / Registrant / Administrative / Technical / Billing，每区段 7 字段（Name/Organization/Email/Phone/Country/City/Street）。切片字段（Status/NameServers）用 `strings.Join` 合并为单值。

---

## 5️⃣ Markdown 格式

中文标题表格，便于阅读：

```markdown
# 域名信息

| 字段 | 值 |
|------|-----|
| 域名 | example.com |
| 创建时间 | 1995-08-14 |
| ... | ... |

## 注册商

| 字段 | 值 |
|------|-----|
| 名称 | Reserved |
| ... | ... |

## 注册人
...
## 管理联系人
...
## 技术联系人
...
## 账单联系人
...
```

---

## 6️⃣ 导出流到标准输出

```go
whois.ExportToMarkdown(result.Info, os.Stdout)
```

---

## 7️⃣ 批量导出

```go
domains := []string{"a.com", "b.com", "c.com"}
for _, d := range domains {
	result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{Domain: d})
	if err != nil {
		continue
	}
	f, _ := os.Create(fmt.Sprintf("exports/%s.json", d))
	whois.ExportToJSON(result.Info, f)
	f.Close()
}
```

---

## 8️⃣ HTTP API 导出

导出端点**直接返回文件内容**，而非 APIResponse 包装：

```bash
# JSON
curl -X POST http://127.0.0.1:8080/api/export/json \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com"}' \
  -o example.json

# CSV（自动触发下载）
curl -X POST http://127.0.0.1:8080/api/export/csv \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com"}' \
  -o example.csv

# Markdown
curl -X POST http://127.0.0.1:8080/api/export/markdown \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com"}' \
  -o example.md
```

| 端点 | Content-Type |
|------|-------------|
| `/api/export/json` | `application/json` |
| `/api/export/csv` | `text/csv` + `Content-Disposition: attachment; filename=<domain>.csv` |
| `/api/export/markdown` | `text/markdown` |

::: tip 📥 CSV 自动下载
CSV 端点设置 `Content-Disposition: attachment`，浏览器会自动触发文件下载。
:::

📖 详见 [导出端点](../api/http/endpoint-export.md)。

---

## ✅ 小结

| 需求 | 推荐 |
|------|------|
| 程序交换 | JSON |
| 表格分析 | CSV |
| 文档报告 | Markdown |
| HTTP 触发 | `/api/export/*` |

---

## 🔗 相关

- 📤 [export.go API](../api/whois/export.md)
- 📝 [format.go 格式化](../api/whois/format.md)
- 📥 [导出端点](../api/http/endpoint-export.md)
