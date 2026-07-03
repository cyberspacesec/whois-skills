package whois

import (
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

// 测试数据构建辅助函数
func makeWhoisInfo(domain, registrantName, registrantOrg, registrantEmail, registrantCountry, registrarName string, nameservers []string, createdDate string) *whoisparser.WhoisInfo {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:      domain,
			CreatedDate: createdDate,
			NameServers: nameservers,
		},
	}
	if registrantName != "" || registrantOrg != "" || registrantEmail != "" || registrantCountry != "" {
		info.Registrant = &whoisparser.Contact{
			Name:         registrantName,
			Organization: registrantOrg,
			Email:        registrantEmail,
			Country:      registrantCountry,
		}
	}
	if registrarName != "" {
		info.Registrar = &whoisparser.Contact{
			Name: registrarName,
		}
	}
	return info
}

func TestNewCorrelationEngine(t *testing.T) {
	engine := NewCorrelationEngine()
	if engine == nil {
		t.Fatal("NewCorrelationEngine() returned nil")
	}
	if engine.domainMap == nil {
		t.Error("domainMap should be initialized")
	}
	if engine.emailClusters == nil {
		t.Error("emailClusters should be initialized")
	}
	if engine.registrantClusters == nil {
		t.Error("registrantClusters should be initialized")
	}
	if engine.orgClusters == nil {
		t.Error("orgClusters should be initialized")
	}
	if engine.nsClusters == nil {
		t.Error("nsClusters should be nil")
	}
	if engine.registrarClusters == nil {
		t.Error("registrarClusters should be initialized")
	}
}

func TestCorrelationEngine_AddDomain_EmailClustering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加共享同一邮箱的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "Jane Smith", "Other Corp", "jane@other.com", "UK", "Namecheap", nil, "2022-01-01"))

	result := engine.Analyze()

	// 应该有一个邮箱聚类 (john@acme.com)
	emailClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByEmail {
			emailClusterCount++
			if cluster.Key == "john@acme.com" {
				if cluster.Count != 2 {
					t.Errorf("Email cluster for john@acme.com should have 2 domains, got %d", cluster.Count)
				}
				found1, found2 := false, false
				for _, d := range cluster.Domains {
					if d == "site1.com" {
						found1 = true
					}
					if d == "site2.com" {
						found2 = true
					}
				}
				if !found1 || !found2 {
					t.Errorf("Email cluster should contain site1.com and site2.com, got %v", cluster.Domains)
				}
			}
		}
	}

	if emailClusterCount != 1 {
		t.Errorf("Expected 1 email cluster, got %d", emailClusterCount)
	}
}

func TestCorrelationEngine_AddDomain_RegistrantClustering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加共享同一注册人的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john1@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john2@acme.com", "US", "GoDaddy", nil, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "Jane Smith", "Other Corp", "jane@other.com", "UK", "Namecheap", nil, "2022-01-01"))

	result := engine.Analyze()

	registrantClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByRegistrant {
			registrantClusterCount++
		}
	}

	if registrantClusterCount != 1 {
		t.Errorf("Expected 1 registrant cluster, got %d", registrantClusterCount)
	}
}

func TestCorrelationEngine_AddDomain_OrgClustering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加共享同一组织的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "Person A", "Acme Corp", "a@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "Person B", "Acme Corp", "b@acme.com", "US", "Namecheap", nil, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "Person C", "Other Corp", "c@other.com", "UK", "Namecheap", nil, "2022-01-01"))

	result := engine.Analyze()

	orgClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByOrg {
			orgClusterCount++
			if cluster.Key == "Acme Corp" {
				if cluster.Count != 2 {
					t.Errorf("Org cluster for Acme Corp should have 2 domains, got %d", cluster.Count)
				}
			}
		}
	}

	if orgClusterCount != 1 {
		t.Errorf("Expected 1 org cluster, got %d", orgClusterCount)
	}
}

func TestCorrelationEngine_AddDomain_NSClustering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加共享同一NS基础域名的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "A", "Org A", "a@a.com", "US", "GoDaddy", []string{"ns1.example.com", "ns2.example.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "B", "Org B", "b@b.com", "UK", "Namecheap", []string{"ns1.example.com", "ns3.example.com"}, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "C", "Org C", "c@c.com", "DE", "Cloudflare", []string{"ns1.other.com"}, "2022-01-01"))

	result := engine.Analyze()

	nsClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByNS {
			nsClusterCount++
			if cluster.Key == "example.com" {
				if cluster.Count != 2 {
					t.Errorf("NS cluster for example.com should have 2 domains, got %d", cluster.Count)
				}
			}
		}
	}

	if nsClusterCount < 1 {
		t.Errorf("Expected at least 1 NS cluster, got %d", nsClusterCount)
	}
}

func TestCorrelationEngine_AddDomain_RegistrarClustering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加共享同一注册商的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "A", "Org A", "a@a.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "B", "Org B", "b@b.com", "UK", "GoDaddy", nil, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "C", "Org C", "c@c.com", "DE", "Namecheap", nil, "2022-01-01"))

	result := engine.Analyze()

	registrarClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByRegistrar {
			registrarClusterCount++
			if cluster.Key == "GoDaddy" {
				if cluster.Count != 2 {
					t.Errorf("Registrar cluster for GoDaddy should have 2 domains, got %d", cluster.Count)
				}
			}
		}
	}

	if registrarClusterCount != 1 {
		t.Errorf("Expected 1 registrar cluster, got %d", registrarClusterCount)
	}
}

func TestCorrelationEngine_PrivacyFiltering(t *testing.T) {
	engine := NewCorrelationEngine()

	// 隐私保护的邮箱不应该被聚类
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "Registration Private", "Domains By Proxy, LLC", "proxy@domainsbyproxy.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "Registration Private", "Domains By Proxy, LLC", "proxy@domainsbyproxy.com", "US", "GoDaddy", nil, "2021-01-01"))

	result := engine.Analyze()

	// 隐私保护的邮箱/名称/组织不应该产生聚类
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByEmail {
			t.Errorf("Privacy email should not be clustered, but found cluster: %s", cluster.Key)
		}
		if cluster.Type == ClusterByRegistrant {
			t.Errorf("Privacy name should not be clustered, but found cluster: %s", cluster.Key)
		}
		if cluster.Type == ClusterByOrg {
			t.Errorf("Privacy org should not be clustered, but found cluster: %s", cluster.Key)
		}
	}
}

func TestCorrelationEngine_DuplicateDomain(t *testing.T) {
	engine := NewCorrelationEngine()

	info := makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01")
	engine.AddDomain("site1.com", info)
	engine.AddDomain("site1.com", info) // 重复添加

	result := engine.Analyze()

	// 同一域名不应重复出现在聚类中
	for _, cluster := range result.Clusters {
		count := 0
		for _, d := range cluster.Domains {
			if d == "site1.com" {
				count++
			}
		}
		if count > 1 {
			t.Errorf("Domain site1.com appears %d times in cluster %s, expected at most 1", count, cluster.Key)
		}
	}
}

func TestCorrelationEngine_Analyze_Stats(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com"}, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "Jane Smith", "Other Corp", "jane@other.com", "UK", "Namecheap", []string{"ns1.other.com"}, "2022-01-01"))

	result := engine.Analyze()

	if result.Stats.InputDomains != 3 {
		t.Errorf("InputDomains = %d, want 3", result.Stats.InputDomains)
	}
	if result.Stats.TotalClusters == 0 {
		t.Error("Expected some clusters to be found")
	}
	if result.Stats.TotalEdges == 0 {
		t.Error("Expected some edges in correlation graph")
	}
}

func TestCorrelationEngine_Analyze_Graph(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com"}, "2021-01-01"))

	result := engine.Analyze()

	if result.Graph == nil {
		t.Fatal("Graph should not be nil")
	}
	if len(result.Graph.Nodes) != 2 {
		t.Errorf("Graph nodes = %d, want 2", len(result.Graph.Nodes))
	}
	if len(result.Graph.Edges) == 0 {
		t.Error("Graph should have edges between correlated domains")
	}

	// 检查边的强度 - site1和site2共享邮箱、注册人、组织、NS、注册商
	for _, edge := range result.Graph.Edges {
		if edge.Source == "site1.com" && edge.Target == "site2.com" {
			if edge.Strength < 2 {
				t.Errorf("Edge strength = %d, expected at least 2 (shared email+registrant+org+ns+registrar)", edge.Strength)
			}
		}
	}
}

func TestCorrelationEngine_Analyze_ClusterSummary(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns2.example.com"}, "2022-06-15"))

	result := engine.Analyze()

	for _, cluster := range result.Clusters {
		if cluster.Summary == nil {
			t.Errorf("Cluster %s should have summary", cluster.Key)
			continue
		}
		if cluster.Type == ClusterByEmail && cluster.Key == "john@acme.com" {
			if cluster.Summary.CommonRegistrar != "GoDaddy" {
				t.Errorf("CommonRegistrar = %s, want GoDaddy", cluster.Summary.CommonRegistrar)
			}
			if cluster.Summary.FirstCreated != "2020-01-01" {
				t.Errorf("FirstCreated = %s, want 2020-01-01", cluster.Summary.FirstCreated)
			}
			if cluster.Summary.LastCreated != "2022-06-15" {
				t.Errorf("LastCreated = %s, want 2022-06-15", cluster.Summary.LastCreated)
			}
		}
	}
}

func TestCorrelationEngine_GetAssetProfile(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "Namecheap", nil, "2022-06-15"))

	// 按邮箱获取资产画像
	profile := engine.GetAssetProfile("john@acme.com", ClusterByEmail)
	if profile == nil {
		t.Fatal("GetAssetProfile() returned nil")
	}
	if profile.EntityID != "john@acme.com" {
		t.Errorf("EntityID = %s, want john@acme.com", profile.EntityID)
	}
	if profile.TotalDomains != 2 {
		t.Errorf("TotalDomains = %d, want 2", profile.TotalDomains)
	}
	if len(profile.Domains) != 2 {
		t.Errorf("Domains count = %d, want 2", len(profile.Domains))
	}

	// 检查注册商分布
	if profile.RegistrarDistribution["GoDaddy"] != 1 {
		t.Errorf("GoDaddy count = %d, want 1", profile.RegistrarDistribution["GoDaddy"])
	}
	if profile.RegistrarDistribution["Namecheap"] != 1 {
		t.Errorf("Namecheap count = %d, want 1", profile.RegistrarDistribution["Namecheap"])
	}

	// 检查国家分布
	if profile.CountryDistribution["US"] != 2 {
		t.Errorf("US count = %d, want 2", profile.CountryDistribution["US"])
	}

	// 检查TLD分布
	if profile.TLDistribution["com"] != 2 {
		t.Errorf("com TLD count = %d, want 2", profile.TLDistribution["com"])
	}

	// 检查时间范围
	if profile.TimeRange.Earliest != "2020-01-01" {
		t.Errorf("Earliest = %s, want 2020-01-01", profile.TimeRange.Earliest)
	}
	if profile.TimeRange.Latest != "2022-06-15" {
		t.Errorf("Latest = %s, want 2022-06-15", profile.TimeRange.Latest)
	}
}

func TestCorrelationEngine_GetAssetProfile_NotFound(t *testing.T) {
	engine := NewCorrelationEngine()

	profile := engine.GetAssetProfile("nonexistent@email.com", ClusterByEmail)
	if profile != nil {
		t.Error("Expected nil for non-existent entity")
	}
}

func TestCorrelationEngine_GetAssetProfile_UnsupportedType(t *testing.T) {
	engine := NewCorrelationEngine()

	profile := engine.GetAssetProfile("test", ClusterByNS)
	if profile != nil {
		t.Error("Expected nil for unsupported entity type")
	}
}

func TestCorrelationEngine_GetRegistrarStats(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "Jane Smith", "Other Corp", "jane@other.com", "UK", "GoDaddy", nil, "2021-01-01"))
	engine.AddDomain("site3.com", makeWhoisInfo("site3.com", "Bob Wilson", "Test Corp", "bob@test.com", "DE", "Namecheap", nil, "2022-01-01"))

	stats := engine.GetRegistrarStats()

	if len(stats) != 2 {
		t.Errorf("Expected 2 registrars, got %d", len(stats))
	}

	godaddy, exists := stats["GoDaddy"]
	if !exists {
		t.Fatal("GoDaddy not found in registrar stats")
	}
	if godaddy.TotalDomains != 2 {
		t.Errorf("GoDaddy TotalDomains = %d, want 2", godaddy.TotalDomains)
	}
	if godaddy.CountryDistribution["US"] != 1 {
		t.Errorf("GoDaddy US count = %d, want 1", godaddy.CountryDistribution["US"])
	}
	if godaddy.CountryDistribution["UK"] != 1 {
		t.Errorf("GoDaddy UK count = %d, want 1", godaddy.CountryDistribution["UK"])
	}

	namecheap, exists := stats["Namecheap"]
	if !exists {
		t.Fatal("Namecheap not found in registrar stats")
	}
	if namecheap.TotalDomains != 1 {
		t.Errorf("Namecheap TotalDomains = %d, want 1", namecheap.TotalDomains)
	}
}

func TestCorrelationEngine_GetRegistrarStats_PrivacyDetection(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "Registration Private", "Domains By Proxy, LLC", "proxy@domainsbyproxy.com", "US", "GoDaddy", nil, "2021-01-01"))

	stats := engine.GetRegistrarStats()

	godaddy, exists := stats["GoDaddy"]
	if !exists {
		t.Fatal("GoDaddy not found in registrar stats")
	}
	if godaddy.PrivacyProtected != 1 {
		t.Errorf("GoDaddy PrivacyProtected = %d, want 1", godaddy.PrivacyProtected)
	}
}

func TestCorrelationEngine_Analyze_NoCorrelations(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加完全无关的域名
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "A", "Org A", "a@a.com", "US", "Registrar A", []string{"ns1.a.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "B", "Org B", "b@b.com", "UK", "Registrar B", []string{"ns1.b.com"}, "2021-01-01"))

	result := engine.Analyze()

	// 没有共享属性的域名之间不应产生聚类
	if result.Stats.InputDomains != 2 {
		t.Errorf("InputDomains = %d, want 2", result.Stats.InputDomains)
	}
	// 单域名聚类不应出现在结果中（需要至少2个域名）
	for _, cluster := range result.Clusters {
		if cluster.Count < 2 {
			t.Errorf("Cluster %s has only %d domains, expected at least 2", cluster.Key, cluster.Count)
		}
	}
}

func TestCorrelationEngine_Analyze_EmptyEngine(t *testing.T) {
	engine := NewCorrelationEngine()

	result := engine.Analyze()

	if result.Stats.InputDomains != 0 {
		t.Errorf("InputDomains = %d, want 0", result.Stats.InputDomains)
	}
	if result.Stats.TotalClusters != 0 {
		t.Errorf("TotalClusters = %d, want 0", result.Stats.TotalClusters)
	}
	if len(result.Clusters) != 0 {
		t.Errorf("Clusters count = %d, want 0", len(result.Clusters))
	}
}

func TestCorrelationEngine_AddDomain_NilInfo(t *testing.T) {
	engine := NewCorrelationEngine()

	// 添加nil的WHOIS信息不应崩溃
	engine.AddDomain("site1.com", nil)

	result := engine.Analyze()
	if result.Stats.InputDomains != 1 {
		t.Errorf("InputDomains = %d, want 1", result.Stats.InputDomains)
	}
}

func TestCorrelationEngine_AddDomain_NilRegistrant(t *testing.T) {
	engine := NewCorrelationEngine()

	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:      "site1.com",
			NameServers: []string{"ns1.example.com"},
		},
		Registrar: &whoisparser.Contact{
			Name: "GoDaddy",
		},
	}

	engine.AddDomain("site1.com", info)
	result := engine.Analyze()

	// 不应崩溃，只是没有邮箱/注册人/组织聚类
	if result.Stats.InputDomains != 1 {
		t.Errorf("InputDomains = %d, want 1", result.Stats.InputDomains)
	}
}

// 辅助函数测试

func TestIsPrivacyEmail(t *testing.T) {
	tests := []struct {
		email   string
		private bool
	}{
		{"proxy@domainsbyproxy.com", true},
		{"user@contactprivacy.com", true},
		{"john@example.com", false},
		{"privacy@service.com", true},
		{"redacted@whois.com", true},
		{"normal@gmail.com", false},
		{"PROXY@DOMAINSBYPROXY.COM", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isPrivacyEmail(tt.email)
			if result != tt.private {
				t.Errorf("isPrivacyEmail(%q) = %v, want %v", tt.email, result, tt.private)
			}
		})
	}
}

func TestIsPrivacyName(t *testing.T) {
	tests := []struct {
		name    string
		private bool
	}{
		{"REDACTED FOR PRIVACY", true},
		{"Registration Private", true},
		{"Withheld for Privacy", true},
		{"Not Disclosed", true},
		{"John Doe", false},
		{"PROXY REGISTRANT", true},
		{"Masked User", true},
		{"  protected  ", true},
		{"Normal Person", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPrivacyName(tt.name)
			if result != tt.private {
				t.Errorf("isPrivacyName(%q) = %v, want %v", tt.name, result, tt.private)
			}
		})
	}
}

func TestIsPrivacyOrg(t *testing.T) {
	tests := []struct {
		org     string
		private bool
	}{
		{"Domains By Proxy, LLC", true},
		{"Contact Privacy Inc.", true},
		{"DATA PROTECTED", true},
		{"Acme Corporation", false},
		{"Privacy Shield LLC", true},
		{"Redacted Org", true},
		{"Normal Company Ltd", false},
	}

	for _, tt := range tests {
		t.Run(tt.org, func(t *testing.T) {
			result := isPrivacyOrg(tt.org)
			if result != tt.private {
				t.Errorf("isPrivacyOrg(%q) = %v, want %v", tt.org, result, tt.private)
			}
		})
	}
}

func TestExtractNSBase(t *testing.T) {
	tests := []struct {
		ns   string
		want string
	}{
		{"ns1.example.com", "example.com"},
		{"ns2.example.com", "example.com"},
		{"dns1.google.com", "google.com"},
		{"a.ns.cloudflare.com", "cloudflare.com"},
		{"ns.example.co.uk", "co.uk"},
		{"single", ""},
		{"ns1.example.com.", "example.com"},
		{"  NS1.EXAMPLE.COM  ", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.ns, func(t *testing.T) {
			result := extractNSBase(tt.ns)
			if result != tt.want {
				t.Errorf("extractNSBase(%q) = %q, want %q", tt.ns, result, tt.want)
			}
		})
	}
}

func TestCorrelationEngine_MultipleCorrelationTypes(t *testing.T) {
	engine := NewCorrelationEngine()

	// 两个域名共享多个属性
	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com", "ns2.example.com"}, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", []string{"ns1.example.com", "ns3.example.com"}, "2021-01-01"))

	result := engine.Analyze()

	// 应该有多种类型的聚类
	clusterTypes := make(map[ClusterType]int)
	for _, cluster := range result.Clusters {
		clusterTypes[cluster.Type]++
	}

	if clusterTypes[ClusterByEmail] != 1 {
		t.Errorf("Email clusters = %d, want 1", clusterTypes[ClusterByEmail])
	}
	if clusterTypes[ClusterByRegistrant] != 1 {
		t.Errorf("Registrant clusters = %d, want 1", clusterTypes[ClusterByRegistrant])
	}
	if clusterTypes[ClusterByOrg] != 1 {
		t.Errorf("Org clusters = %d, want 1", clusterTypes[ClusterByOrg])
	}
	if clusterTypes[ClusterByRegistrar] != 1 {
		t.Errorf("Registrar clusters = %d, want 1", clusterTypes[ClusterByRegistrar])
	}

	// 图中应该有边，且强度应该很高
	if len(result.Graph.Edges) == 0 {
		t.Error("Expected edges in correlation graph")
	}
	for _, edge := range result.Graph.Edges {
		if edge.Strength < 3 {
			t.Errorf("Edge strength = %d, expected at least 3 for highly correlated domains", edge.Strength)
		}
	}
}

func TestCorrelationEngine_LargeDataset(t *testing.T) {
	engine := NewCorrelationEngine()

	// 模拟一个较大的数据集
	// 3个组织，每个组织3个域名
	orgs := []struct {
		name     string
		domains  []string
		email    string
		registrar string
	}{
		{"Acme Corp", []string{"acme.com", "acme.net", "acme.org"}, "admin@acme.com", "GoDaddy"},
		{"Beta Inc", []string{"beta.com", "beta.net", "beta.org"}, "admin@beta.com", "Namecheap"},
		{"Gamma LLC", []string{"gamma.com", "gamma.net", "gamma.org"}, "admin@gamma.com", "Cloudflare"},
	}

	for _, org := range orgs {
		for i, domain := range org.domains {
			engine.AddDomain(domain, makeWhoisInfo(domain, org.name+" Admin", org.name, org.email, "US", org.registrar, []string{"ns1." + domain, "ns2." + domain}, "2020-01-01"))
			_ = i
		}
	}

	result := engine.Analyze()

	if result.Stats.InputDomains != 9 {
		t.Errorf("InputDomains = %d, want 9", result.Stats.InputDomains)
	}
	if result.Stats.TotalClusters == 0 {
		t.Error("Expected clusters to be found")
	}

	// 每个组织应该有一个组织聚类
	orgClusterCount := 0
	for _, cluster := range result.Clusters {
		if cluster.Type == ClusterByOrg {
			orgClusterCount++
		}
	}
	if orgClusterCount != 3 {
		t.Errorf("Org clusters = %d, want 3", orgClusterCount)
	}
}

func TestCorrelationEngine_ConcurrentAccess(t *testing.T) {
	engine := NewCorrelationEngine()

	// 并发添加域名
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			domain := "site" + string(rune('0'+idx)) + ".com"
			engine.AddDomain(domain, makeWhoisInfo(domain, "John Doe", "Acme Corp", "john@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	result := engine.Analyze()
	if result.Stats.InputDomains != 10 {
		t.Errorf("InputDomains = %d, want 10", result.Stats.InputDomains)
	}
}

func TestCorrelationEngine_GetAssetProfile_ByOrg(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "Person A", "Acme Corp", "a@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "Person B", "Acme Corp", "b@acme.com", "US", "Namecheap", nil, "2022-06-15"))

	profile := engine.GetAssetProfile("Acme Corp", ClusterByOrg)
	if profile == nil {
		t.Fatal("GetAssetProfile() returned nil for org")
	}
	if profile.EntityType != ClusterByOrg {
		t.Errorf("EntityType = %v, want %v", profile.EntityType, ClusterByOrg)
	}
	if profile.TotalDomains != 2 {
		t.Errorf("TotalDomains = %d, want 2", profile.TotalDomains)
	}
}

func TestCorrelationEngine_GetAssetProfile_ByRegistrant(t *testing.T) {
	engine := NewCorrelationEngine()

	engine.AddDomain("site1.com", makeWhoisInfo("site1.com", "John Doe", "Acme Corp", "a@acme.com", "US", "GoDaddy", nil, "2020-01-01"))
	engine.AddDomain("site2.com", makeWhoisInfo("site2.com", "John Doe", "Other Corp", "b@other.com", "UK", "Namecheap", nil, "2022-06-15"))

	profile := engine.GetAssetProfile("John Doe", ClusterByRegistrant)
	if profile == nil {
		t.Fatal("GetAssetProfile() returned nil for registrant")
	}
	if profile.TotalDomains != 2 {
		t.Errorf("TotalDomains = %d, want 2", profile.TotalDomains)
	}
}

func BenchmarkCorrelationEngine_Analyze(b *testing.B) {
	engine := NewCorrelationEngine()

	// 预填充数据
	for i := 0; i < 100; i++ {
		domain := "site" + string(rune('0'+i%10)) + string(rune('0'+i/10)) + ".com"
		org := "Org" + string(rune('A'+i%5))
		email := "user" + string(rune('0'+i%10)) + "@example.com"
		engine.AddDomain(domain, makeWhoisInfo(domain, "User", org, email, "US", "GoDaddy", []string{"ns1.example.com"}, "2020-01-01"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Analyze()
	}
}

func BenchmarkCorrelationEngine_AddDomain(b *testing.B) {
	engine := NewCorrelationEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := "bench" + string(rune('0'+i%10)) + ".com"
		engine.AddDomain(domain, makeWhoisInfo(domain, "User", "Org", "user@test.com", "US", "GoDaddy", nil, "2020-01-01"))
	}
}
