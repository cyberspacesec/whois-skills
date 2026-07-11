package whois

import (
	"testing"

	"github.com/stretchr/testify/assert"
	whoisparser "github.com/likexian/whois-parser"
)

// ==================== assessCompleteness / assessTimeliness / assessReliability 边界 ====================

// TestAssessCompleteness_NilInfo info 为 nil → 返回 0。
func TestAssessCompleteness_NilInfo(t *testing.T) {
	score := &QualityScore{}
	assert.Equal(t, 0, assessCompleteness(nil, score))
}

// TestAssessTimeliness_NilDomain info 非 nil 但 Domain 为 nil → 返回 0。
func TestAssessTimeliness_NilDomain(t *testing.T) {
	score := &QualityScore{}
	assert.Equal(t, 0, assessTimeliness(&whoisparser.WhoisInfo{}, score))
}

// TestAssessTimeliness_NoCreatedDate 有 Domain 但无 CreatedDate → 返回 50 + Issue。
func TestAssessTimeliness_NoCreatedDate(t *testing.T) {
	score := &QualityScore{}
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com"}}
	got := assessTimeliness(info, score)
	assert.Equal(t, 50, got)
	assert.NotEmpty(t, score.Issues)
}

// TestAssessTimeliness_CreatedNoUpdated 有 CreatedDate 无 UpdatedDate → 返回 70。
func TestAssessTimeliness_CreatedNoUpdated(t *testing.T) {
	score := &QualityScore{}
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{Domain: "x.com", CreatedDate: "2020-01-01"}}
	assert.Equal(t, 70, assessTimeliness(info, score))
}

// TestAssessTimeliness_BothDates 有 Created + Updated → 返回 90。
func TestAssessTimeliness_BothDates(t *testing.T) {
	score := &QualityScore{}
	info := &whoisparser.WhoisInfo{Domain: &whoisparser.Domain{
		Domain:      "x.com",
		CreatedDate: "2020-01-01",
		UpdatedDate: "2024-01-01",
	}}
	assert.Equal(t, 90, assessTimeliness(info, score))
}

// TestAssessReliability_NilInfo info 为 nil → 返回 0。
func TestAssessReliability_NilInfo(t *testing.T) {
	score := &QualityScore{}
	assert.Equal(t, 0, assessReliability(nil, score))
}

// TestAssessReliability_TemplateData 触发 isTemplateData → 扣 10 + Issue。
func TestAssessReliability_TemplateData(t *testing.T) {
	score := &QualityScore{}
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Name: "test"}, // 匹配 templatePatterns ^test$
	}
	got := assessReliability(info, score)
	assert.Less(t, got, 100)
	found := false
	for _, iss := range score.Issues {
		if iss.Type == IssueDuplicateData {
			found = true
		}
	}
	assert.True(t, found, "应检测到模板数据")
}

// TestAssessReliability_InvalidEmail 触发无效邮箱扣 5 分。
func TestAssessReliability_InvalidEmail(t *testing.T) {
	score := &QualityScore{}
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Email: "not-an-email"},
	}
	got := assessReliability(info, score)
	assert.Less(t, got, 100)
	found := false
	for _, iss := range score.Issues {
		if iss.Type == IssueInvalidFormat {
			found = true
		}
	}
	assert.True(t, found)
}

// ==================== detectPrivacy 各未覆盖子分支 ====================

// TestDetectPrivacy_OrgKeywordNotInRules org 不匹配 privacyRules 但匹配 privacyOrgKeywords
// → PrivacyOrganizationPrivacy 类型（line 460-464）。
func TestDetectPrivacy_OrgKeywordNotInRules(t *testing.T) {
	// "masked" 在 privacyOrgKeywords 但不在 privacyRules 的 patterns
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Organization: "Masked Identity Corp"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
	foundOrg := false
	for _, ty := range d.Types {
		if ty == PrivacyOrganizationPrivacy {
			foundOrg = true
		}
	}
	assert.True(t, foundOrg, "应检测到 PrivacyOrganizationPrivacy")
	// ProxyOrganization 只在 privacyRules 匹配时设置，org 关键词分支不设置
}

// TestDetectPrivacy_NameGenericPattern name 不匹配 privacyRules 但匹配通用 privacyNamePatterns
// → PrivacyRedacted（line 502-506）。
func TestDetectPrivacy_NameGenericPattern(t *testing.T) {
	// "withheld" 在通用 privacyNamePatterns 但 org 为空、不匹配 privacyRules 的 name
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Name: "withheld registrant"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
	foundRedacted := false
	for _, ty := range d.Types {
		if ty == PrivacyRedacted {
			foundRedacted = true
		}
	}
	assert.True(t, foundRedacted, "通用模式应检测到 PrivacyRedacted")
}

// TestDetectPrivacy_EmailSuffix email 匹配 privacyEmailSuffixes → 设 ProxyEmail（line 516-522）。
func TestDetectPrivacy_EmailSuffix(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Email: "user@domainsbyproxy.com"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
	assert.Equal(t, "user@domainsbyproxy.com", d.ProxyEmail)
	assert.Contains(t, d.ProtectedFields, "registrant")
}

// TestDetectPrivacy_ProxyOrganization org 匹配 privacyRules → 设 ProxyOrganization（line 444-446）。
func TestDetectPrivacy_ProxyOrganization(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Organization: "Domains By Proxy, LLC"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
	assert.Equal(t, "Domains By Proxy, LLC", d.ProxyOrganization)
}

// TestDetectPrivacy_EmailKeywordNotSuffix email 不匹配 suffix 但匹配 emailPatterns
// → HasPrivacy=true（line 530-533）。
func TestDetectPrivacy_EmailKeywordNotSuffix(t *testing.T) {
	// "mask" 在 emailPatterns，邮箱域名不在 suffix 列表
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Email: "user@somemaskservice.example"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
}

// TestDetectPrivacy_MultiContactsProtectionLevelCap 4 个联系人都 protected
// → privacyScore=100，触发 ProtectionLevel>100 截断（line 547-549）。
func TestDetectPrivacy_MultiContactsProtectionLevelCap(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant:    &whoisparser.Contact{Email: "a@domainsbyproxy.com"},
		Administrative: &whoisparser.Contact{Email: "b@domainsbyproxy.com"},
		Technical:     &whoisparser.Contact{Email: "c@domainsbyproxy.com"},
		Billing:       &whoisparser.Contact{Email: "d@domainsbyproxy.com"},
	}
	d := detectPrivacy(info)
	assert.True(t, d.HasPrivacy)
	assert.Equal(t, 100, d.ProtectionLevel, "应被截断到 100")
	assert.Len(t, d.ProtectedFields, 4)
}

// TestDetectPrivacy_NilInfo info 为 nil → 返回空 detection（line 411-413）。
func TestDetectPrivacy_NilInfo(t *testing.T) {
	d := detectPrivacy(nil)
	assert.NotNil(t, d)
	assert.False(t, d.HasPrivacy)
	assert.Empty(t, d.Types)
}

// ==================== appendUniquePrivacyType 重复分支 ====================

// TestAppendUniquePrivacyType_Duplicate 已存在类型 → 返回原切片不追加（line 556-559）。
func TestAppendUniquePrivacyType_Duplicate(t *testing.T) {
	types := []PrivacyType{PrivacyRedacted}
	got := appendUniquePrivacyType(types, PrivacyRedacted)
	assert.Len(t, got, 1, "重复类型不应追加")
}

// TestAppendUniquePrivacyType_New 新类型 → 追加。
func TestAppendUniquePrivacyType_New(t *testing.T) {
	types := []PrivacyType{PrivacyRedacted}
	got := appendUniquePrivacyType(types, PrivacyWHOISPrivacy)
	assert.Len(t, got, 2)
}

// ==================== NormalizeContactField organization/default 分支 ====================

// TestNormalizeContactField_Organization organization 分支 → normalizeName。
func TestNormalizeContactField_Organization(t *testing.T) {
	got := NormalizeContactField("  ACME   CORP  ", "organization")
	// normalizeName 会去多余空格；全大写长度>2 可能转首字母大写
	assert.NotEmpty(t, got)
}

// TestNormalizeContactField_Default 未知 fieldType → 原样返回（去首尾空格）。
func TestNormalizeContactField_Default(t *testing.T) {
	got := NormalizeContactField("  some value  ", "unknown_field")
	assert.Equal(t, "some value", got)
}
