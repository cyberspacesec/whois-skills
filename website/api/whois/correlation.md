# 🔗 correlation.go — WHOIS 关联分析引擎

> 📖 WHOIS 关联分析引擎，按邮箱、注册人、组织、NS、注册商五个维度对多域名进行聚类，生成关联图与资产画像，是资产测绘与威胁情报分析的核心能力。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 文件 | `pkg/whois/correlation.go` |
| 核心职责 | 多域名聚类、关联图构建、资产画像、注册商统计 |
| 聚类维度 | 邮箱 / 注册人 / 组织 / NS / 注册商 |
| 隐私过滤 | 自动过滤隐私保护联系方式 |

---

## 🚀 快速使用

```go
import "github.com/cyberspacesec/whois-skills/pkg/whois"

engine := whois.NewCorrelationEngine()

// 添加多个域名的 WHOIS 信息
for domain, info := range domainInfoMap {
    engine.AddDomain(domain, info)
}

// 分析
result := engine.Analyze()
for _, cluster := range result.Clusters {
    fmt.Printf("[%s] %s: %d 个域名\n", cluster.Type, cluster.Key, cluster.Count)
}

// 资产画像
profile := engine.GetAssetProfile("admin@example.com", "email")
fmt.Println("该邮箱关联域名数：", profile.TotalDomains)
```

---

## 📊 核心类型

### ClusterType 常量

| 常量 | 值 | 聚类维度 |
|------|----|----------|
| `ClusterByEmail` | — | 按邮箱 |
| `ClusterByRegistrant` | — | 按注册人姓名 |
| `ClusterByOrg` | — | 按组织 |
| `ClusterByNS` | — | 按 NS |
| `ClusterByRegistrar` | — | 按注册商 |

### Cluster

```go
type Cluster struct {
    Key     string         // 聚类键（如邮箱地址）
    Type    ClusterType    // 聚类类型
    Domains []string       // 域名列表
    Count   int            // 域名数
    Summary *ClusterSummary // 聚类摘要
}
```

### ClusterSummary

```go
type ClusterSummary struct {
    CommonRegistrant    string
    CommonOrganization  string
    CommonRegistrar     string
    CommonCountries     []string
    CommonNameServers   []string
    FirstCreated        string // 最早创建日期
    LastCreated         string // 最晚创建日期
}
```

### CorrelationResult

```go
type CorrelationResult struct {
    Clusters []*Cluster         // 显著聚类（≥2 域名）
    Graph    *CorrelationGraph   // 关联图
    Stats    CorrelationStats    // 统计
}
```

### CorrelationGraph

```go
type CorrelationGraph struct {
    Nodes []*GraphNode
    Edges []*GraphEdge
}

type GraphNode struct {
    Domain       string
    Registrant   string
    Organization string
    Registrar    string
}

type GraphEdge struct {
    Source    string
    Target    string
    Type      string   // 关联类型
    Key       string   // 聚类键
    Strength  int      // 关联强度（同对域名共享次数）
}
```

### AssetProfile 资产画像

```go
type AssetProfile struct {
    EntityID              string          // 实体标识（邮箱/姓名/组织）
    EntityType            string          // email/registrant/org
    Domains               []AssetDomain
    TotalDomains          int
    RegistrarDistribution map[string]int  // 注册商分布
    CountryDistribution   map[string]int  // 国家分布
    TLDistribution        map[string]int  // TLD 分布
    TimeRange             TimeRange       // 时间范围
}

type AssetDomain struct {
    Domain         string
    CreatedDate    string
    ExpirationDate string
    Registrar      string
    Status         string
}

type TimeRange struct {
    Earliest string
    Latest   string
}
```

### RegistrarStat

```go
type RegistrarStat struct {
    Registrar           string
    TotalDomains        int
    Domains             []string
    CountryDistribution map[string]int
    PrivacyProtected    int // 隐私保护域名数
}
```

---

## 🔧 方法

| 方法 | 说明 |
|------|------|
| `NewCorrelationEngine() *CorrelationEngine` | 创建引擎 |
| `AddDomain(domain, info)` | 添加域名，按 5 维聚类（过滤隐私） |
| `Analyze() *CorrelationResult` | 收集显著聚类 → 生成摘要 → 构图 → 统计 |
| `GetAssetProfile(entityID, entityType) *AssetProfile` | 资产画像（仅支持 email/registrant/org） |
| `GetRegistrarStats() map[string]*RegistrarStat` | 注册商统计 |

---

## 🔍 关键实现要点

::: details AddDomain 五维聚类
对每个域名，按以下维度建立聚类：

1. **邮箱**：遍历 4 个联系人区段的 Email 字段
2. **注册人**：registrant.Name
3. **组织**：registrant.Organization
4. **NS**：取 name_servers 的**最后两段**作为聚类键（如 `ns1.example.com` → `example.com`）
5. **注册商**：registrar.Name

每个维度维护独立的 `map[string][]string`（键→域名列表）。
:::

::: details 隐私保护过滤
`AddDomain` 在聚类前对每个联系人字段进行隐私检测：

- `isPrivacyEmail` — 邮箱后缀匹配 `privacyEmailSuffixes`（12 个）
- `isPrivacyName` — 姓名匹配 `privacyRules`（13 条规则）
- `isPrivacyOrg` — 组织匹配 `privacyOrgKeywords`（11 个）

匹配到隐私保护的字段不参与聚类，避免误关联。
:::

::: details 显著聚类筛选
`collectSignificantClusters` 只保留 `Count >= 2` 的聚类（即至少有 2 个域名共享该键），单域名聚类无关联意义。
:::

::: details 关联图构建
`buildCorrelationGraph` 对每个聚类内的域名两两建边：

- 同一对域名若通过多个聚类键关联，合并为一条边，`Strength` 累加
- `Strength` 越高表示两域名关联越紧密
:::

::: details ClusterSummary 生成
`generateClusterSummary` 对聚类内所有域名统计：

- 各字段取**频次最高**的值作为 Common 值
- 日期范围取最早/最晚创建日期
- 国家与 NS 收集去重列表
:::

---

## 📝 使用示例

### 示例 1：基础聚类分析

```go
engine := whois.NewCorrelationEngine()
for _, d := range domains {
    info, _ := whois.ExecuteQuery(&whois.QueryOptions{Domain: d})
    engine.AddDomain(d, info)
}

result := engine.Analyze()
for _, c := range result.Clusters {
    if c.Type == whois.ClusterByEmail {
        fmt.Printf("邮箱 %s 关联 %d 域名：%v\n", c.Key, c.Count, c.Domains)
    }
}
```

### 示例 2：资产画像

```go
profile := engine.GetAssetProfile("admin@company.com", "email")
fmt.Printf("注册商分布：%v\n", profile.RegistrarDistribution)
fmt.Printf("TLD 分布：%v\n", profile.TLDistribution)
fmt.Printf("时间范围：%s ~ %s\n",
    profile.TimeRange.Earliest, profile.TimeRange.Latest)
```

### 示例 3：注册商统计

```go
stats := engine.GetRegistrarStats()
for registrar, s := range stats {
    fmt.Printf("%s: %d 域名，隐私保护 %d\n", registrar, s.TotalDomains, s.PrivacyProtected)
}
```

### 示例 4：导出关联图

```go
result := engine.Analyze()
for _, edge := range result.Graph.Edges {
    fmt.Printf("%s --[%s/%s]--> %s (强度 %d)\n",
        edge.Source, edge.Type, edge.Key, edge.Target, edge.Strength)
}
```

---

## ⚠️ 注意事项

- 隐私保护域名会被自动过滤，可能漏掉真实关联（需结合 [reverse.md](./reverse.md) 反查）。
- NS 聚类键取最后两段，对 CDN/托管服务的 NS 可能产生大量误关联。
- 资产画像仅支持 `email`/`registrant`/`org` 三种实体类型。

---

## 🔗 相关

- 📈 [关联分析教程](../../guide/tutorial-correlation.md)
- 🎯 [quality.md](./quality.md) — 数据质量评估（含隐私检测规则）
- 🔄 [reverse.md](./reverse.md) — 反向 WHOIS 查询
