# 🎯 quality.go — WHOIS 数据质量评估

> 📖 WHOIS 数据质量评估引擎，从完整性、时效性、可信度三个维度评分，并提供隐私保护检测，是衡量 WHOIS 数据可用性的核心工具。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/quality.go` |
| 核心职责 | 质量评分、隐私检测、字段规范化 |
| 评分维度 | 完整性 / 时效性 / 可信度 |
| 隐私规则 | 13 条规则 + 12 邮箱后缀 + 11 组织关键词 |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: "example.com"})

score := whois.AssessQuality(info)
fmt.Printf("总分 %d，等级 %s\n", score.Total, score.Level)
fmt.Printf("完整性 %d / 时效性 %d / 可信度 %d\n",
    score.Completeness, score.Timeliness, score.Reliability)

if score.PrivacyDetection != nil && score.PrivacyDetection.HasPrivacy {
    fmt.Println("隐私保护服务：", score.PrivacyDetection.Provider)
}
```

---

## 📊 核心类型

### QualityScore

```go
type QualityScore struct {
    Total            int                 // 总分（0-100）
    Completeness     int                 // 完整性
    Timeliness       int                 // 时效性
    Reliability      int                 // 可信度
    Level            QualityLevel        // 等级
    MissingFields    []string            // 缺失字段
    PrivacyDetection *PrivacyDetection   // 隐私检测
    Issues           []QualityIssue      // 问题列表
}
```

### QualityLevel 等级

| 常量 | 分数区间 | 含义 |
|------|----------|------|
| `QualityLevelExcellent` | 80-100 | 优秀 |
| `QualityLevelGood` | 60-79 | 良好 |
| `QualityLevelFair` | 40-59 | 一般 |
| `QualityLevelPoor` | 20-39 | 较差 |
| `QualityLevelUnusable` | 0-19 | 不可用 |

### QualityIssue 与 IssueType

```go
type QualityIssue struct {
    Type        IssueType
    Description string
    Field       string
    Severity    string
}
```

| IssueType 常量 | 含义 |
|----------------|------|
| `IssueMissingField` | 缺失字段 |
| `IssuePrivacyProtected` | 隐私保护 |
| `IssueInvalidFormat` | 格式无效 |
| `IssueStaleData` | 数据过时 |
| `IssueDuplicateData` | 重复数据 |
| `IssueRedactedData` | 数据被抹除 |

### PrivacyDetection

```go
type PrivacyDetection struct {
    HasPrivacy       bool          // 是否有隐私保护
    Types            []PrivacyType // 保护类型
    Provider         string        // 服务商
    ProxyEmail       string
    ProxyOrganization string
    ProtectedFields  []string
    ProtectionLevel  int           // 保护等级（0-100）
}
```

| PrivacyType 常量 | 含义 |
|------------------|------|
| `PrivacyWHOISPrivacy` | WHOIS Privacy |
| `PrivacyDomainsByProxy` | Domains By Proxy |
| `PrivacyRedacted` | 数据抹除 |
| `PrivacyDataProtected` | Data Protected |
| `PrivacyContactPrivacy` | Contact Privacy |
| `PrivacyOrganizationPrivacy` | 组织隐私 |

---

## 🔧 导出函数

| 函数 | 说明 |
|------|------|
| `AssessQuality(info) *QualityScore` | 质量评估（**主入口**） |
| `NormalizeContactField(value, fieldType) string` | 联系人字段规范化 |
| `determineQualityLevel(score) QualityLevel` | 分数转等级（内部） |

### NormalizeContactField 支持的类型

| fieldType | 处理 |
|-----------|------|
| `email` | 转小写、去空格 |
| `phone` | 去非数字字符 |
| `country` | 转大写 |
| `name` | 去多余空格 |
| `organization` | 去多余空格 |

---

## 🔍 关键实现要点

::: details AssessQuality 主流程
1. **assessCompleteness** — 检查 16 个字段，按权重（2-15）加权评分
2. **assessTimeliness** — 时效性评分
3. **assessReliability** — 可信度评分
4. **Total = 三项均值**
5. **detectPrivacy** — 隐私检测
6. **determineQualityLevel** — 转等级
:::

::: details assessCompleteness 字段权重

| 字段 | 权重 |
|------|------|
| `domain` | 15 |
| `created` / `expiration` / `registrar_name` / `registrant_name` / `email` | 8-10 |
| `注册商邮箱` / `组织` | 3-5 |
| `技术联系人` 各字段 | 2 |

权重越高，缺失对该项扣分越多。
:::

::: details assessTimeliness 时效性评分

| 条件 | 分数 |
|------|------|
| 无创建日期 | 50 |
| 有创建无更新 | 70 |
| 都有 | 90 |

:::

::: details assessReliability 可信度评分
起始 100 分，扣分项：

- 隐私保护：扣 `ProtectionLevel / 2`
- 邮箱无效（`mail.ParseAddress` 失败）：扣 5
- 模板数据（`isTemplateData` 正则匹配）：扣 10
:::

::: details detectPrivacy 隐私检测
遍历 4 个联系人区段，检查 `Organization`/`Name`/`Email`：

- 匹配 `privacyRules`（13 条规则）
- 或匹配 `privacyOrgKeywords`（11 个）
- 或邮箱后缀匹配 `privacyEmailSuffixes`（12 个）

每个受保护的联系人加 25 分（上限 100）到 `ProtectionLevel`。

#### privacyRules 规则示例
包含：Domains By Proxy / WHOIS Privacy / Contact Privacy / DATA PROTECTED / GDPR Redacted / Perfect Privacy / eName / Pantheon / Registration Private / Withheld for Privacy / ID Protect / Digital Privacy 等。
:::

---

## 📝 使用示例

### 示例 1：完整质量评估

```go
info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: "example.com"})
score := whois.AssessQuality(info)

fmt.Printf("等级: %s (%d)\n", score.Level, score.Total)
fmt.Printf("缺失字段: %v\n", score.MissingFields)
for _, issue := range score.Issues {
    fmt.Printf("  - [%s] %s\n", issue.Type, issue.Description)
}
```

### 示例 2：隐私检测

```go
if score.PrivacyDetection.HasPrivacy {
    fmt.Println("⚠️ 该域名启用了 WHOIS 隐私保护")
    fmt.Println("服务商:", score.PrivacyDetection.Provider)
    fmt.Println("保护等级:", score.PrivacyDetection.ProtectionLevel)
    fmt.Println("受保护字段:", score.PrivacyDetection.ProtectedFields)
}
```

### 示例 3：批量质量筛选

```go
for _, d := range domains {
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: d})
    score := whois.AssessQuality(info)
    if score.Level == whois.QualityLevelUnusable {
        fmt.Printf("跳过 %s：数据质量不可用\n", d)
        continue
    }
    // 处理高质量数据
}
```

### 示例 4：字段规范化

```go
email := whois.NormalizeContactField("  Admin@Example.COM ", "email")
// email = "admin@example.com"
phone := whois.NormalizeContactField("+1 (650) 123-4567", "phone")
// phone = "+16501234567" 或类似
```

---

## 🔗 相关

- 🔗 [correlation.md](./correlation.md) — 关联分析（复用隐私检测规则）
- 🎨 [format.md](./format.md) — 格式检测
- 📈 [数据质量教程](../../guide/tutorial-quality.md)
