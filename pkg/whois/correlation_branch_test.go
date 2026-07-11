package whois

import (
	"testing"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== correlation.go 剩余分支 ====================

// TestGenerateClusterSummary_DomainNotInMap cluster 含不在 domainMap 的域名 → info==nil continue。
// 通过 Correlate → collectSignificantClusters → generateClusterSummary 触发（line 368-369）。
func TestGenerateClusterSummary_DomainNotInMap(t *testing.T) {
	e := NewCorrelationEngine()
	e.AddDomain("a.com", &whoisparser.WhoisInfo{
		Registrar:   &whoisparser.Contact{Name: "RegA"},
		Registrant:  &whoisparser.Contact{Country: "US"},
		Domain:      &whoisparser.Domain{NameServers: []string{"ns1.a.com"}, CreatedDate: "2020-01-01"},
	})
	// 直接操作 emailClusters，加入一个不在 domainMap 的域名
	e.mu.Lock()
	e.emailClusters["shared@x.com"] = &Cluster{
		Type:    ClusterByEmail,
		Key:     "shared@x.com",
		Domains: []string{"a.com", "ghost.com"}, // ghost.com 不在 domainMap
		Count:   2,                               // >=2 才会被 collectSignificantClusters 收集
	}
	e.mu.Unlock()

	// Analyze 会对收集到的 cluster 调 generateClusterSummary
	result := e.Analyze()
	assert.NotNil(t, result)
	// a.com 计入，ghost.com 被跳过
	found := false
	for _, c := range result.Clusters {
		if c.Key == "shared@x.com" {
			found = true
			assert.NotNil(t, c.Summary, "应生成摘要")
			// a.com 的 registrar RegA 应被收集
			assert.Equal(t, "RegA", c.Summary.CommonRegistrar)
		}
	}
	assert.True(t, found, "应找到 shared@x.com cluster 摘要")
}

// TestGetAssetProfile_UnknownEntityType 未知 entityType → 返回 nil。
func TestGetAssetProfile_UnknownEntityType(t *testing.T) {
	e := NewCorrelationEngine()
	assert.Nil(t, e.GetAssetProfile("x", ClusterType("bogus")))
}

// TestGetAssetProfile_NilCluster cluster 不存在 → 返回 nil。
func TestGetAssetProfile_NilCluster(t *testing.T) {
	e := NewCorrelationEngine()
	assert.Nil(t, e.GetAssetProfile("nonexistent@x.com", ClusterByEmail))
}

// TestGetRegistrarStats_NilRegistrar domainMap 有 info 但 Registrar 为 nil → continue。
func TestGetRegistrarStats_NilRegistrar(t *testing.T) {
	e := NewCorrelationEngine()
	// 有 registrar 的域名
	e.AddDomain("a.com", &whoisparser.WhoisInfo{
		Registrar: &whoisparser.Contact{Name: "RegA"},
	})
	// Registrar 为 nil 的域名（直接设 domainMap）
	e.mu.Lock()
	e.domainMap["b.com"] = &whoisparser.WhoisInfo{} // Registrar == nil
	e.domainMap["c.com"] = &whoisparser.WhoisInfo{
		Registrar: &whoisparser.Contact{Name: ""}, // Name 空
	}
	e.mu.Unlock()

	stats := e.GetRegistrarStats()
	assert.Contains(t, stats, "RegA")
	// b.com/c.com 被跳过，不产生空 registrar 条目
	_, hasEmpty := stats[""]
	assert.False(t, hasEmpty)
}

// TestIsPrivacyEmail_SuffixOnlyNotPattern email 匹配 suffix 但不含 pattern 关键词 →
// 命中 suffix 分支（line 638-640）。@ename.com 不含 proxy/privacy/protect/redact/mask/withheld/private。
func TestIsPrivacyEmail_SuffixOnlyNotPattern(t *testing.T) {
	assert.True(t, isPrivacyEmail("user@ename.com"))
}

// TestIsPrivacyEmail_Normal 不匹配 → false。
func TestIsPrivacyEmail_Normal(t *testing.T) {
	assert.False(t, isPrivacyEmail("user@gmail.com"))
}
