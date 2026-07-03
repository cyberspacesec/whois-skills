package whois

import (
	"strings"
	"sync"

	whoisparser "github.com/likexian/whois-parser"
)

// CorrelationEngine WHOIS关联分析引擎
// 用于从多个域名的WHOIS数据中发现关联关系
type CorrelationEngine struct {
	mu sync.RWMutex

	// 域名到WHOIS信息的映射
	domainMap map[string]*whoisparser.WhoisInfo

	// 按邮箱的域名聚类
	emailClusters map[string]*Cluster

	// 注册人名称的域名聚类
	registrantClusters map[string]*Cluster

	// 按组织的域名聚类
	orgClusters map[string]*Cluster

	// 按NS的域名聚类
	nsClusters map[string]*Cluster

	// 按注册商的域名聚类
	registrarClusters map[string]*Cluster}

// Cluster 聚类结果
type Cluster struct {
	// 聚类键 (邮箱、注册人名称、组织等)
	Key string `json:"key"`

	// 聚类类型
	Type ClusterType `json:"type"`

	// 聚类内的域名列表
	Domains []string `json:"domains"`

	// 域名数量
	Count int `json:"count"`

	// 共同属性摘要
	Summary *ClusterSummary `json:"summary,omitempty"`
}

// ClusterType 聚类类型
type ClusterType string

const (
	ClusterByEmail      ClusterType = "email"
	ClusterByRegistrant ClusterType = "registrant"
	ClusterByOrg        ClusterType = "organization"
	ClusterByNS         ClusterType = "nameserver"
	ClusterByRegistrar  ClusterType = "registrar"
)

// ClusterSummary 聚类摘要信息
type ClusterSummary struct {
	// 共同的注册人名称
	CommonRegistrant string `json:"common_registrant,omitempty"`

	// 共同的组织
	CommonOrganization string `json:"common_organization,omitempty"`

	// 共同的注册商
	CommonRegistrar string `json:"common_registrar,omitempty"`

	// 共同的国家
	CommonCountries []string `json:"common_countries,omitempty"`

	// 共同的NS
	CommonNameServers []string `json:"common_nameservers,omitempty"`

	// 时间跨度
	FirstCreated string `json:"first_created,omitempty"`
	LastCreated  string `json:"last_created,omitempty"`
}

// CorrelationResult 关联分析结果
type CorrelationResult struct {
	// 所有聚类
	Clusters []*Cluster `json:"clusters"`

	// 域名关联图
	Graph *CorrelationGraph `json:"graph,omitempty"`

	// 统计信息
	Stats CorrelationStats `json:"stats"`
}

// CorrelationGraph 域名关联图
type CorrelationGraph struct {
	// 节点 (域名)
	Nodes []*GraphNode `json:"nodes"`

	// 边 (关联关系)
	Edges []*GraphEdge `json:"edges"`
}

// GraphNode 图节点
type GraphNode struct {
	// 域名
	Domain string `json:"domain"`

	// 注册人
	Registrant string `json:"registrant,omitempty"`

	// 组织
	Organization string `json:"organization,omitempty"`

	// 注册商
	Registrar string `json:"registrar,omitempty"`
}

// GraphEdge 图边
type GraphEdge struct {
	// 源域名
	Source string `json:"source"`

	// 目标域名
	Target string `json:"target"`

	// 关联类型
	Type ClusterType `json:"type"`

	// 关联键
	Key string `json:"key"`

	// 关联强度 (1-N, 表示有多少个共同属性)
	Strength int `json:"strength"`
}

// CorrelationStats 关联分析统计
type CorrelationStats struct {
	// 输入域名数量
	InputDomains int `json:"input_domains"`

	// 按邮箱聚类数
	EmailClusters int `json:"email_clusters"`

	// 按注册人聚类数
	RegistrantClusters int `json:"registrant_clusters"`

	// 按组织聚类数
	OrgClusters int `json:"org_clusters"`

	// 按NS聚类数
	NSClusters int `json:"ns_clusters"`

	// 按注册商聚类数
	RegistrarClusters int `json:"registrar_clusters"`

	// 总聚类数
	TotalClusters int `json:"total_clusters"`

	// 关联边数
	TotalEdges int `json:"total_edges"`
}

// AssetProfile 资产画像
// 表示一个实体（如某注册人/组织）拥有的所有域名资产
type AssetProfile struct {
	// 实体标识 (邮箱或组织名称)
	EntityID string `json:"entity_id"`

	// 实体类型
	EntityType ClusterType `json:"entity_type"`

	// 所属域名列表
	Domains []AssetDomain `json:"domains"`

	// 域名总数
	TotalDomains int `json:"total_domains"`

	// 注册商分布
	RegistrarDistribution map[string]int `json:"registrar_distribution"`

	// 国家分布
	CountryDistribution map[string]int `json:"country_distribution"`

	// TLD分布
	TLDistribution map[string]int `json:"tld_distribution"`

	// 时间分布
	TimeRange TimeRange `json:"time_range"`
}

// AssetDomain 资产域名信息
type AssetDomain struct {
	// 域名
	Domain string `json:"domain"`

	// 创建日期
	CreatedDate string `json:"created_date,omitempty"`

	// 过期日期
	ExpirationDate string `json:"expiration_date,omitempty"`

	// 注册商
	Registrar string `json:"registrar,omitempty"`

	// 状态
	Status []string `json:"status,omitempty"`
}

// TimeRange 时间范围
type TimeRange struct {
	Earliest string `json:"earliest,omitempty"`
	Latest   string `json:"latest,omitempty"`
}

// NewCorrelationEngine 创建新的关联分析引擎
func NewCorrelationEngine() *CorrelationEngine {
	return &CorrelationEngine{
		domainMap:           make(map[string]*whoisparser.WhoisInfo),
		emailClusters:       make(map[string]*Cluster),
		registrantClusters:  make(map[string]*Cluster),
		orgClusters:         make(map[string]*Cluster),
		nsClusters:         make(map[string]*Cluster),
		registrarClusters:  make(map[string]*Cluster),
	}
}

// AddDomain 向引擎添加域名的WHOIS信息
func (e *CorrelationEngine) AddDomain(domain string, info *whoisparser.WhoisInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.domainMap[domain] = info

	if info == nil {
		return
	}

	// 按邮箱聚类
	if info.Registrant != nil && info.Registrant.Email != "" {
		email := NormalizeContactField(info.Registrant.Email, "email")
		if !isPrivacyEmail(email) {
			e.addToCluster(e.emailClusters, ClusterByEmail, email, domain)
		}
	}

	// 按注册人名称聚类
	if info.Registrant != nil && info.Registrant.Name != "" {
		name := NormalizeContactField(info.Registrant.Name, "name")
		if !isPrivacyName(name) {
			e.addToCluster(e.registrantClusters, ClusterByRegistrant, name, domain)
		}
	}

	// 按组织聚类
	if info.Registrant != nil && info.Registrant.Organization != "" {
		org := NormalizeContactField(info.Registrant.Organization, "organization")
		if !isPrivacyOrg(org) {
			e.addToCluster(e.orgClusters, ClusterByOrg, org, domain)
		}
	}

	// 按NS聚类
	if info.Domain != nil && len(info.Domain.NameServers) > 0 {
		for _, ns := range info.Domain.NameServers {
			ns = strings.ToLower(strings.TrimSpace(ns))
			// 使用NS的基础域名作为聚类键（去掉子域名）
			nsBase := extractNSBase(ns)
			if nsBase != "" {
				e.addToCluster(e.nsClusters, ClusterByNS, nsBase, domain)
			}
		}
	}

	// 按注册商聚类
	if info.Registrar != nil && info.Registrar.Name != "" {
		registrar := strings.TrimSpace(info.Registrar.Name)
		e.addToCluster(e.registrarClusters, ClusterByRegistrar, registrar, domain)
	}
}

// addToCluster 向指定聚类添加域名
func (e *CorrelationEngine) addToCluster(clusters map[string]*Cluster, clusterType ClusterType, key string, domain string) {
	cluster, exists := clusters[key]
	if !exists {
		cluster = &Cluster{
			Key:     key,
			Type:    clusterType,
			Domains: make([]string, 0),
		}
		clusters[key] = cluster
	}

	// 检查域名是否已存在
	for _, d := range cluster.Domains {
		if d == domain {
			return
		}
	}

	cluster.Domains = append(cluster.Domains, domain)
	cluster.Count = len(cluster.Domains)
}

// Analyze 执行关联分析
func (e *CorrelationEngine) Analyze() *CorrelationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &CorrelationResult{
		Clusters: make([]*Cluster, 0),
	}

	// 收集所有有意义的聚类（至少包含2个域名）
	result.Clusters = append(result.Clusters, e.collectSignificantClusters(e.emailClusters)...)
	result.Clusters = append(result.Clusters, e.collectSignificantClusters(e.registrantClusters)...)
	result.Clusters = append(result.Clusters, e.collectSignificantClusters(e.orgClusters)...)
	result.Clusters = append(result.Clusters, e.collectSignificantClusters(e.nsClusters)...)
	result.Clusters = append(result.Clusters, e.collectSignificantClusters(e.registrarClusters)...)

	// 为每个聚类生成摘要
	for _, cluster := range result.Clusters {
		cluster.Summary = e.generateClusterSummary(cluster)
	}

	// 构建关联图
	result.Graph = e.buildCorrelationGraph(result.Clusters)

	// 统计信息
	result.Stats = CorrelationStats{
		InputDomains:        len(e.domainMap),
		EmailClusters:       len(e.emailClusters),
		RegistrantClusters:  len(e.registrantClusters),
		OrgClusters:         len(e.orgClusters),
		NSClusters:         len(e.nsClusters),
		RegistrarClusters:  len(e.registrarClusters),
		TotalClusters:       len(result.Clusters),
		TotalEdges:          len(result.Graph.Edges),
	}

	return result
}

// collectSignificantClusters 收集包含至少2个域名的聚类
func (e *CorrelationEngine) collectSignificantClusters(clusters map[string]*Cluster) []*Cluster {
	var result []*Cluster
	for _, cluster := range clusters {
		if cluster.Count >= 2 {
			result = append(result, cluster)
		}
	}
	return result
}

// generateClusterSummary 为聚类生成摘要信息
func (e *CorrelationEngine) generateClusterSummary(cluster *Cluster) *ClusterSummary {
	summary := &ClusterSummary{}

	registrarCount := make(map[string]int)
	countryCount := make(map[string]int)
	nsSet := make(map[string]bool)
	firstCreated := ""
	lastCreated := ""

	for _, domain := range cluster.Domains {
		info := e.domainMap[domain]
		if info == nil {
			continue
		}

		// 注册商
		if info.Registrar != nil && info.Registrar.Name != "" {
			registrarCount[info.Registrar.Name]++
		}

		// 国家
		if info.Registrant != nil && info.Registrant.Country != "" {
			countryCount[info.Registrant.Country]++
		}

		// NS
		if info.Domain != nil {
			for _, ns := range info.Domain.NameServers {
				nsSet[strings.ToLower(ns)] = true
			}
		}

		// 创建日期
		if info.Domain != nil && info.Domain.CreatedDate != "" {
			if firstCreated == "" || info.Domain.CreatedDate < firstCreated {
				firstCreated = info.Domain.CreatedDate
			}
			if lastCreated == "" || info.Domain.CreatedDate > lastCreated {
				lastCreated = info.Domain.CreatedDate
			}
		}
	}

	// 找最常见的注册商
	maxRegistrar := ""
	maxCount := 0
	for r, c := range registrarCount {
		if c > maxCount {
			maxRegistrar = r
			maxCount = c
		}
	}
	summary.CommonRegistrar = maxRegistrar

	// 收集国家
	for c := range countryCount {
		summary.CommonCountries = append(summary.CommonCountries, c)
	}

	// 收集NS
	for ns := range nsSet {
		summary.CommonNameServers = append(summary.CommonNameServers, ns)
	}

	summary.FirstCreated = firstCreated
	summary.LastCreated = lastCreated

	return summary
}

// buildCorrelationGraph 构建域名关联图
func (e *CorrelationEngine) buildCorrelationGraph(clusters []*Cluster) *CorrelationGraph {
	graph := &CorrelationGraph{
		Nodes: make([]*GraphNode, 0),
		Edges: make([]*GraphEdge, 0),
	}

	// 构建节点
	domainSeen := make(map[string]bool)
	for domain, info := range e.domainMap {
		if domainSeen[domain] {
			continue
		}
		domainSeen[domain] = true

		node := &GraphNode{
			Domain:       domain,
			Registrant:   "",
			Organization: "",
			Registrar:    "",
		}
		if info != nil {
			if info.Registrant != nil {
				node.Registrant = info.Registrant.Name
				node.Organization = info.Registrant.Organization
			}
			if info.Registrar != nil {
				node.Registrar = info.Registrar.Name
			}
		}
		graph.Nodes = append(graph.Nodes, node)
	}

	// 构建边：通过聚类关系连接域名
	for _, cluster := range clusters {
		domains := cluster.Domains
		for i := 0; i < len(domains); i++ {
			for j := i + 1; j < len(domains); j++ {
				edge := &GraphEdge{
					Source:   domains[i],
					Target:   domains[j],
					Type:     cluster.Type,
					Key:      cluster.Key,
					Strength: 1,
				}
				graph.Edges = append(graph.Edges, edge)
			}
		}
	}

	// 计算边的强度（同一对域名有多少个共同属性）
	edgeStrength := make(map[string]int) // source+target -> count
	edgeType := make(map[string]*GraphEdge) // source+target -> first edge
	for _, edge := range graph.Edges {
		key := edge.Source + "|" + edge.Target
		edgeStrength[key]++
		if _, exists := edgeType[key]; !exists {
			edgeType[key] = edge
		}
	}

	// 合并边，更新强度
	mergedEdges := make([]*GraphEdge, 0)
	for key, edge := range edgeType {
		edge.Strength = edgeStrength[key]
		mergedEdges = append(mergedEdges, edge)
	}
	graph.Edges = mergedEdges

	return graph
}

// GetAssetProfile 获取指定实体的资产画像
func (e *CorrelationEngine) GetAssetProfile(entityID string, entityType ClusterType) *AssetProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var cluster *Cluster
	switch entityType {
	case ClusterByEmail:
		cluster = e.emailClusters[entityID]
	case ClusterByRegistrant:
		cluster = e.registrantClusters[entityID]
	case ClusterByOrg:
		cluster = e.orgClusters[entityID]
	default:
		return nil
	}

	if cluster == nil {
		return nil
	}

	profile := &AssetProfile{
		EntityID:             entityID,
		EntityType:           entityType,
		Domains:              make([]AssetDomain, 0),
		RegistrarDistribution: make(map[string]int),
		CountryDistribution:  make(map[string]int),
		TLDistribution:       make(map[string]int),
	}

	for _, domain := range cluster.Domains {
		info := e.domainMap[domain]
		if info == nil {
			continue
		}

		asset := AssetDomain{
			Domain: domain,
		}

		if info.Domain != nil {
			asset.CreatedDate = info.Domain.CreatedDate
			asset.ExpirationDate = info.Domain.ExpirationDate
			asset.Status = info.Domain.Status

			tld := extractTLD(domain)
			profile.TLDistribution[tld]++
		}

		if info.Registrar != nil {
			asset.Registrar = info.Registrar.Name
			profile.RegistrarDistribution[info.Registrar.Name]++
		}

		if info.Registrant != nil {
			profile.CountryDistribution[info.Registrant.Country]++
		}

		// 更新时间范围
		if asset.CreatedDate != "" {
			if profile.TimeRange.Earliest == "" || asset.CreatedDate < profile.TimeRange.Earliest {
				profile.TimeRange.Earliest = asset.CreatedDate
			}
			if profile.TimeRange.Latest == "" || asset.CreatedDate > profile.TimeRange.Latest {
				profile.TimeRange.Latest = asset.CreatedDate
			}
		}

		profile.Domains = append(profile.Domains, asset)
	}

	profile.TotalDomains = len(profile.Domains)

	return profile
}

// GetRegistrarStats 获取注册商维度的统计
func (e *CorrelationEngine) GetRegistrarStats() map[string]*RegistrarStat {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := make(map[string]*RegistrarStat)

	for domain, info := range e.domainMap {
		if info.Registrar == nil || info.Registrar.Name == "" {
			continue
		}

		registrar := info.Registrar.Name
		stat, exists := stats[registrar]
		if !exists {
			stat = &RegistrarStat{
				Registrar:           registrar,
				Domains:             make([]string, 0),
				CountryDistribution: make(map[string]int),
			}
			stats[registrar] = stat
		}

		stat.TotalDomains++
		stat.Domains = append(stat.Domains, domain)

		if info.Registrant != nil && info.Registrant.Country != "" {
			stat.CountryDistribution[info.Registrant.Country]++
		}

		// 检查是否使用了隐私保护
		if info.Registrant != nil {
			detection := detectPrivacy(info)
			if detection.HasPrivacy {
				stat.PrivacyProtected++
			}
		}
	}

	return stats
}

// RegistrarStat 注册商统计信息
type RegistrarStat struct {
	Registrar         string         `json:"registrar"`
	TotalDomains      int            `json:"total_domains"`
	Domains           []string       `json:"domains"`
	CountryDistribution map[string]int `json:"country_distribution"`
	PrivacyProtected  int            `json:"privacy_protected"`
}

// 辅助函数

// isPrivacyEmail 检查是否是隐私保护邮箱
func isPrivacyEmail(email string) bool {
	lower := strings.ToLower(email)
	privacyPatterns := []string{"proxy", "privacy", "protect", "redact", "mask", "withheld", "private"}
	for _, p := range privacyPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	for _, suffix := range privacyEmailSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// isPrivacyName 检查是否是隐私保护名称
func isPrivacyName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	privacyPatterns := []string{"redacted", "privacy", "private", "protected", "proxy", "withheld", "masked", "not disclosed"}
	for _, p := range privacyPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isPrivacyOrg 检查是否是隐私保护组织
func isPrivacyOrg(org string) bool {
	lower := strings.ToLower(strings.TrimSpace(org))
	for _, rule := range privacyRules {
		for _, pattern := range rule.patterns {
			if strings.Contains(lower, pattern) {
				return true
			}
		}
	}
	privacyOrgPatterns := []string{"proxy", "privacy", "protect", "redact", "withheld", "masked", "private"}
	for _, p := range privacyOrgPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// extractNSBase 提取NS服务器的基础域名
// ns1.example.com -> example.com
func extractNSBase(ns string) string {
	ns = strings.ToLower(strings.TrimSpace(ns))
	// 去掉尾部点号
	ns = strings.TrimSuffix(ns, ".")

	parts := strings.Split(ns, ".")
	if len(parts) < 2 {
		return ""
	}

	// 保留最后两部分作为基础域名
	return strings.Join(parts[len(parts)-2:], ".")
}
