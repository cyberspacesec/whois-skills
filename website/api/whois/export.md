# 📤 export.go — WHOIS 信息导出

> 📖 将 WHOIS 信息导出为 JSON、CSV、Markdown 三种格式，便于归档、报表生成与人工查阅。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/export.go` |
| 核心职责 | WHOIS 信息格式化导出 |
| 支持格式 | JSON / CSV / Markdown |
| 输出方式 | `io.Writer`（流式写入） |

---

## 🚀 快速使用

```go
import (
    "bytes"
    "github.com/cyberspacesec/whois-skills/pkg/whois"
)

info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: "example.com"})

// JSON
var buf bytes.Buffer
whois.ExportToJSON(info, &buf)
fmt.Println(buf.String())

// CSV（写入文件）
f, _ := os.Create("example.csv")
defer f.Close()
whois.ExportToCSV(info, f)

// Markdown
whois.ExportToMarkdown(info, os.Stdout)
```

---

## 🔧 导出函数

| 函数 | 说明 |
|------|------|
| `ExportToJSON(info, w io.Writer) error` | 导出 JSON，缩进 2 空格 |
| `ExportToCSV(info, w io.Writer) error` | 导出 CSV，两列表（Field, Value） |
| `ExportToMarkdown(info, w io.Writer) error` | 导出 Markdown，中文标题表格 |

---

## 🔍 关键实现要点

::: details ExportToJSON
使用 `json.MarshalIndent` 生成缩进 2 空格的 JSON，直接写入 `io.Writer`。结构体字段保持 `WhoisInfo` 的原始 JSON tag。
:::

::: details ExportToCSV 格式
CSV 为两列表 `Field, Value`，包含：

- **域名区段**：Domain、Created、Updated、Expiration、Whois Server、DNSSEC、Status、Name Servers
- **5 个联系人区段**：Registrar / Registrant / Administrative / Technical / Billing
- 每个联系人区段 7 个字段：Name / Organization / Email / Phone / Country / City / Street

切片字段（如 Name Servers、Status）用 `strings.Join` 合并为单行。
:::

::: details ExportToMarkdown 格式
生成中文标题的表格，区段标题如：

```markdown
## 域名信息
| 字段 | 值 |
|------|------|
| 域名 | example.com |
...

## 注册商
| 字段 | 值 |
...
```

每个联系人区段同样输出 7 字段表格。
:::

::: details nil 检查
三个函数在 `info == nil` 时返回错误，避免空指针 panic。各联系人区段为 nil 时跳过对应表格。
:::

---

## 📝 使用示例

### 示例 1：导出 JSON

```go
var buf bytes.Buffer
if err := whois.ExportToJSON(info, &buf); err != nil {
    log.Fatal(err)
}
fmt.Println(buf.String())
```

### 示例 2：批量导出 CSV

```go
f, _ := os.Create("whois_report.csv")
defer f.Close()

// CSV 首行
fmt.Fprintln(f, "Domain,Field,Value")

for _, domain := range domains {
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: domain})
    if info != nil {
        whois.ExportToCSV(info, f)
    }
}
```

### 示例 3：生成 Markdown 报告

```go
report, _ := os.Create("report.md")
defer report.Close()

fmt.Fprintln(report, "# WHOIS 报告\n")
for _, domain := range domains {
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: domain})
    fmt.Fprintf(report, "\n# %s\n", domain)
    whois.ExportToMarkdown(info, report)
}
```

### 示例 4：HTTP 响应直接导出

```go
func handler(w http.ResponseWriter, r *http.Request) {
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: r.URL.Query().Get("domain")})
    w.Header().Set("Content-Type", "application/json")
    whois.ExportToJSON(info, w)
}
```

---

## 🔗 相关

- 🎨 [format.md](./format.md) — 原始响应格式检测与清洗
- 🔎 [query.md](./query.md) — 查询引擎
- 📈 [数据导出教程](../../guide/tutorial-export.md)
