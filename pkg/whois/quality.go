package whois

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	whoisparser "github.com/likexian/whois-parser"
)

// QualityScore WHOIS数据质量评分
type QualityScore struct {
	// 总评分 (0-100)
	Total int `json:"total"`

	// 完整性评分 (0-100)
	Completeness int `json:"completeness"`

	// 时效性评分 (0-100)
	Timeliness int `json:"timeliness"`

	// 可信度评分 (0-100)
	Reliability int `json:"reliability"`

	// 数据层级
	Level QualityLevel `json:"level"`

	// 缺失的关键字段
	MissingFields []string `json:"missing_fields,omitempty"`

	// 隐私保护检测结果
	PrivacyDetection *PrivacyDetection `json:"privacy_detection,omitempty"`

	// 数据问题列表
	Issues []QualityIssue `json:"issues,omitempty"`
}

// QualityLevel 数据质量层级
type QualityLevel string

const (
	QualityLevelExcellent QualityLevel = "excellent" // 80-100: 数据完整可信
	QualityLevelGood      QualityLevel = "good"      // 60-79:  数据基本完整
	QualityLevelFair      QualityLevel = "fair"      // 40-59:  数据有缺失但可参考
	QualityLevelPoor      QualityLevel = "poor"      // 20-39:  数据大量缺失
	QualityLevelUnusable  QualityLevel = "unusable"  // 0-19:   数据几乎无用
)

// QualityIssue 数据质量问题
type QualityIssue struct {
	// 问题类型
	Type IssueType `json:"type"`

	// 问题描述
	Description string `json:"description"`

	// 受影响字段
	Field string `json:"field,omitempty"`

	// 严重程度 (critical/warning/info)
	Severity string `json:"severity"`
}

// IssueType 问题类型
type IssueType string

const (
	IssueMissingField      IssueType = "missing_field"
	IssuePrivacyProtected  IssueType = "privacy_protected"
	IssueInvalidFormat     IssueType = "invalid_format"
	IssueStaleData         IssueType = "stale_data"
	IssueDuplicateData     IssueType = "duplicate_data"
	IssueRedactedData      IssueType = "redacted_data"
)

// PrivacyDetection 隐私保护检测结果
type PrivacyDetection struct {
	// 是否使用了隐私保护服务
	HasPrivacy bool `json:"has_privacy"`

	// 隐私保护类型
	Types []PrivacyType `json:"types,omitempty"`

	// 隐私保护服务提供商
	Provider string `json:"provider,omitempty"`

	// 隐私保护邮箱
	ProxyEmail string `json:"proxy_email,omitempty"`

	// 隐私保护组织
	ProxyOrganization string `json:"proxy_organization,omitempty"`

	// 受保护的联系人字段
	ProtectedFields []string `json:"protected_fields,omitempty"`

	// 隐私保护程度评分 (0-100，越高保护越强)
	ProtectionLevel int `json:"protection_level"`
}

// PrivacyType 隐私保护类型
type PrivacyType string

const (
	PrivacyWHOISPrivacy    PrivacyType = "whois_privacy"     // WHOIS隐私保护服务
	PrivacyDomainsByProxy  PrivacyType = "domains_by_proxy"  // Domains By Proxy
	PrivacyRedacted        PrivacyType = "redacted"           // GDPR redacted
	PrivacyDataProtected   PrivacyType = "data_protected"    // DATA PROTECTED
	PrivacyContactPrivacy  PrivacyType = "contact_privacy"   // 联系人隐私保护
	PrivacyOrganizationPrivacy PrivacyType = "org_privacy"   // 组织隐私保护
)

// 隐私保护服务识别规则
var privacyRules = []struct {
	name     string
	type_    PrivacyType
	patterns []string
}{
	{
		name:     "Domains By Proxy, LLC",
		type_:    PrivacyDomainsByProxy,
		patterns: []string{"domains by proxy", "domainsbyproxy", "dbp"},
	},
	{
		name:     "WHOIS Privacy Protection Service",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"whois privacy", "whoisprivacy", "privacy protect", "privacyprotect"},
	},
	{
		name:     "Contact Privacy Inc.",
		type_:    PrivacyContactPrivacy,
		patterns: []string{"contact privacy", "contactprivacy"},
	},
	{
		name:     "DATA PROTECTED",
		type_:    PrivacyDataProtected,
		patterns: []string{"data protected", "dataprotected"},
	},
	{
		name:     "GDPR Redacted",
		type_:    PrivacyRedacted,
		patterns: []string{"redacted for privacy", "redacted for gdpr", "gdpr redacted", "statutory masking", "please query the rdds service"},
	},
	{
		name:     "Perfect Privacy, LLC",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"perfect privacy", "perfectprivacy"},
	},
	{
		name:     "eName Co.",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"ename co"},
	},
	{
		name:     "Pantheon",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"pantheon"},
	},
	{
		name:     "Registration Private",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"registration private", "registrationprivate"},
	},
	{
		name:     "Withheld for Privacy",
		type_:    PrivacyWHOISPrivacy,
		patterns: []string{"withheld for privacy", "withheldforprivacy"},
	},
	{
		name:     "ID Protect",
		type_:    PrivacyContactPrivacy,
		patterns: []string{"id protect", "idprotect"},
	},
	{
		name:     "Digital Privacy Corporation",
		type_:    PrivacyContactPrivacy,
		patterns: []string{"digital privacy", "digitalprivacy"},
	},
}

// 隐私保护邮箱后缀
var privacyEmailSuffixes = []string{
	"@domainsbyproxy.com",
	"@contactprivacy.com",
	"@whoisprivacyprotect.com",
	"@perfectprivacy.com",
	"@privacyprotect.org",
	"@registrationprivate.com",
	"@withheldforprivacy.com",
	"@idprotect.com",
	"@digitalprivacy.com",
	"@ename.com",
	"@dataprotected.com",
}

// 隐私保护组织关键词
var privacyOrgKeywords = []string{
	"proxy",
	"privacy",
	"protect",
	"redacted",
	"withheld",
	"masked",
	"private",
	"shield",
	"guard",
	"hidden",
}

// AssessQuality 评估WHOIS数据质量
func AssessQuality(info *whoisparser.WhoisInfo) *QualityScore {
	if info == nil {
		return &QualityScore{
			Total:       0,
			Level:       QualityLevelUnusable,
			MissingFields: []string{"all"},
			Issues: []QualityIssue{
				{Type: IssueMissingField, Description: "WHOIS信息为空", Severity: "critical"},
			},
		}
	}

	score := &QualityScore{
		MissingFields: make([]string, 0),
		Issues:        make([]QualityIssue, 0),
	}

	// 1. 评估完整性
	score.Completeness = assessCompleteness(info, score)

	// 2. 评估时效性
	score.Timeliness = assessTimeliness(info, score)

	// 3. 评估可信度（包含隐私保护检测）
	score.Reliability = assessReliability(info, score)

	// 4. 计算总评分
	score.Total = (score.Completeness + score.Timeliness + score.Reliability) / 3

	// 5. 确定层级
	score.Level = determineQualityLevel(score.Total)

	return score
}

// assessCompleteness 评估数据完整性
func assessCompleteness(info *whoisparser.WhoisInfo, score *QualityScore) int {
	if info == nil {
		return 0
	}

	totalWeight := 0
	achievedWeight := 0

	// 关键字段及其权重
	fields := []struct {
		name    string
		present bool
		weight  int
	}{
		// 域名基本信息 (权重高)
		{"domain", info.Domain != nil && info.Domain.Domain != "", 15},
		{"created_date", info.Domain != nil && info.Domain.CreatedDate != "", 10},
		{"expiration_date", info.Domain != nil && info.Domain.ExpirationDate != "", 10},
		{"updated_date", info.Domain != nil && info.Domain.UpdatedDate != "", 5},
		{"status", info.Domain != nil && len(info.Domain.Status) > 0, 5},
		{"name_servers", info.Domain != nil && len(info.Domain.NameServers) > 0, 5},

		// 注册商信息 (权重高)
		{"registrar_name", info.Registrar != nil && info.Registrar.Name != "", 10},
		{"registrar_email", info.Registrar != nil && info.Registrar.Email != "", 3},

		// 注册人信息 (权重中等)
		{"registrant_name", info.Registrant != nil && info.Registrant.Name != "", 8},
		{"registrant_org", info.Registrant != nil && info.Registrant.Organization != "", 5},
		{"registrant_email", info.Registrant != nil && info.Registrant.Email != "", 8},
		{"registrant_country", info.Registrant != nil && info.Registrant.Country != "", 3},
		{"registrant_phone", info.Registrant != nil && info.Registrant.Phone != "", 2},

		// 管理联系人 (权重低)
		{"admin_name", info.Administrative != nil && info.Administrative.Name != "", 3},
		{"admin_email", info.Administrative != nil && info.Administrative.Email != "", 3},

		// 技术联系人 (权重低)
		{"tech_name", info.Technical != nil && info.Technical.Name != "", 2},
		{"tech_email", info.Technical != nil && info.Technical.Email != "", 2},
	}

	for _, f := range fields {
		totalWeight += f.weight
		if f.present {
			achievedWeight += f.weight
		} else {
			score.MissingFields = append(score.MissingFields, f.name)
			severity := "warning"
			if f.weight >= 10 {
				severity = "critical"
			}
			score.Issues = append(score.Issues, QualityIssue{
				Type:        IssueMissingField,
				Description: fmt.Sprintf("缺失字段: %s", f.name),
				Field:       f.name,
				Severity:    severity,
			})
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return achievedWeight * 100 / totalWeight
}

// assessTimeliness 评估数据时效性
func assessTimeliness(info *whoisparser.WhoisInfo, score *QualityScore) int {
	if info == nil || info.Domain == nil {
		return 0
	}

	// 检查创建日期是否可解析
	if info.Domain.CreatedDate == "" {
		score.Issues = append(score.Issues, QualityIssue{
			Type:        IssueStaleData,
			Description: "缺少创建日期，无法评估时效性",
			Field:       "created_date",
			Severity:    "warning",
		})
		return 50 // 无数据时给中等评分
	}

	// 尝试解析更新日期
	if info.Domain.UpdatedDate == "" {
		return 70 // 有创建日期但无更新日期
	}

	// 如果有创建和更新日期，评估是否较新
	// 简化评估：只要有这些日期就给较高评分
	return 90
}

// assessReliability 评估数据可信度（含隐私保护检测）
func assessReliability(info *whoisparser.WhoisInfo, score *QualityScore) int {
	if info == nil {
		return 0
	}

	baseScore := 100
	detection := detectPrivacy(info)
	score.PrivacyDetection = detection

	if detection.HasPrivacy {
		// 隐私保护会降低可信度
		penalty := detection.ProtectionLevel / 2 // 保护程度越高，可信度越低
		baseScore -= penalty

		score.Issues = append(score.Issues, QualityIssue{
			Type:        IssuePrivacyProtected,
			Description: fmt.Sprintf("使用了隐私保护服务: %s", detection.Provider),
			Severity:    "warning",
		})

		for _, field := range detection.ProtectedFields {
			score.Issues = append(score.Issues, QualityIssue{
				Type:        IssueRedactedData,
				Description: fmt.Sprintf("字段被隐私保护遮蔽: %s", field),
				Field:       field,
				Severity:    "info",
			})
		}
	}

	// 检查邮箱格式有效性
	if info.Registrant != nil && info.Registrant.Email != "" {
		if !isValidEmail(info.Registrant.Email) {
			baseScore -= 5
			score.Issues = append(score.Issues, QualityIssue{
				Type:        IssueInvalidFormat,
				Description: fmt.Sprintf("注册人邮箱格式无效: %s", info.Registrant.Email),
				Field:       "registrant_email",
				Severity:    "warning",
			})
		}
	}

	// 检查是否有明显的数据重复或模板数据
	if isTemplateData(info) {
		baseScore -= 10
		score.Issues = append(score.Issues, QualityIssue{
			Type:        IssueDuplicateData,
			Description: "检测到模板/占位数据",
			Severity:    "warning",
		})
	}

	if baseScore < 0 {
		baseScore = 0
	}

	return baseScore
}

// detectPrivacy 检测隐私保护服务
func detectPrivacy(info *whoisparser.WhoisInfo) *PrivacyDetection {
	detection := &PrivacyDetection{
		HasPrivacy:      false,
		Types:           make([]PrivacyType, 0),
		ProtectedFields: make([]string, 0),
	}

	if info == nil {
		return detection
	}

	// 检查联系人中的隐私保护模式
	contacts := []*whoisparser.Contact{
		info.Registrant,
		info.Administrative,
		info.Technical,
		info.Billing,
	}

	contactNames := []string{"registrant", "administrative", "technical", "billing"}
	privacyScore := 0

	for i, contact := range contacts {
		if contact == nil {
			continue
		}

		isProtected := false

		// 检查组织名称是否匹配隐私保护服务
		if contact.Organization != "" {
			for _, rule := range privacyRules {
				lowerOrg := strings.ToLower(contact.Organization)
				for _, pattern := range rule.patterns {
					if strings.Contains(lowerOrg, pattern) {
						detection.HasPrivacy = true
						detection.Types = appendUniquePrivacyType(detection.Types, rule.type_)
						if detection.Provider == "" {
							detection.Provider = rule.name
						}
						if contact.Organization != "" && i == 0 {
							detection.ProxyOrganization = contact.Organization
						}
						isProtected = true
						break
					}
				}
				if isProtected {
					break
				}
			}

			// 检查隐私保护关键词
			if !isProtected {
				lowerOrg := strings.ToLower(contact.Organization)
				for _, keyword := range privacyOrgKeywords {
					if strings.Contains(lowerOrg, keyword) {
						detection.HasPrivacy = true
						detection.Types = appendUniquePrivacyType(detection.Types, PrivacyOrganizationPrivacy)
						isProtected = true
						break
					}
				}
			}
		}

		// 检查名称是否是隐私保护标识
		if contact.Name != "" && !isProtected {
			lowerName := strings.ToLower(contact.Name)

			// 先检查已知隐私保护规则，以匹配正确的类型
			for _, rule := range privacyRules {
				for _, pattern := range rule.patterns {
					if strings.Contains(lowerName, pattern) {
						detection.HasPrivacy = true
						detection.Types = appendUniquePrivacyType(detection.Types, rule.type_)
						isProtected = true
						break
					}
				}
				if isProtected {
					break
				}
			}

			// 如果未匹配已知规则，使用通用模式检测
			if !isProtected {
				privacyNamePatterns := []string{
					"redacted for privacy",
					"not disclosed",
					"statutory masking",
					"data protected",
					"withheld",
					"privacy",
					"proxy",
					"protected",
				}
				for _, pattern := range privacyNamePatterns {
					if strings.Contains(lowerName, pattern) {
						detection.HasPrivacy = true
						detection.Types = appendUniquePrivacyType(detection.Types, PrivacyRedacted)
						isProtected = true
						break
					}
				}
			}
		}

		// 检查邮箱是否是隐私保护邮箱
		if contact.Email != "" && !isProtected {
			lowerEmail := strings.ToLower(contact.Email)
			for _, suffix := range privacyEmailSuffixes {
				if strings.HasSuffix(lowerEmail, suffix) {
					detection.HasPrivacy = true
					if i == 0 {
						detection.ProxyEmail = contact.Email
					}
					isProtected = true
					break
				}
			}

			// 检查邮箱中的隐私关键词
			if !isProtected {
				emailPatterns := []string{"proxy", "privacy", "protect", "redact", "mask", "withheld", "private"}
				for _, p := range emailPatterns {
					if strings.Contains(lowerEmail, p) {
						detection.HasPrivacy = true
						isProtected = true
						break
					}
				}
			}
		}

		if isProtected {
			detection.ProtectedFields = append(detection.ProtectedFields, contactNames[i])
			privacyScore += 25
		}
	}

	// 计算隐私保护程度评分
	detection.ProtectionLevel = privacyScore
	if detection.ProtectionLevel > 100 {
		detection.ProtectionLevel = 100
	}

	return detection
}

// appendUniquePrivacyType 添加唯一的隐私保护类型
func appendUniquePrivacyType(types []PrivacyType, newType PrivacyType) []PrivacyType {
	for _, t := range types {
		if t == newType {
			return types
		}
	}
	return append(types, newType)
}

// isValidEmail 验证邮箱格式
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// isTemplateData 检测是否是模板/占位数据
var templatePatterns = regexp.MustCompile(`(?i)(^n/a$|^none$|^not available$|^-+$|^\.+$|^xxx$|^test$|^example$|^placeholder$|^sample$)`)

func isTemplateData(info *whoisparser.WhoisInfo) bool {
	if info == nil {
		return false
	}

	contacts := []*whoisparser.Contact{
		info.Registrant,
		info.Administrative,
		info.Technical,
	}

	for _, contact := range contacts {
		if contact == nil {
			continue
		}
		if contact.Name != "" && templatePatterns.MatchString(strings.TrimSpace(contact.Name)) {
			return true
		}
		if contact.Organization != "" && templatePatterns.MatchString(strings.TrimSpace(contact.Organization)) {
			return true
		}
		if contact.Email != "" && templatePatterns.MatchString(strings.TrimSpace(contact.Email)) {
			return true
		}
	}

	return false
}

// determineQualityLevel 根据评分确定质量层级
func determineQualityLevel(score int) QualityLevel {
	switch {
	case score >= 80:
		return QualityLevelExcellent
	case score >= 60:
		return QualityLevelGood
	case score >= 40:
		return QualityLevelFair
	case score >= 20:
		return QualityLevelPoor
	default:
		return QualityLevelUnusable
	}
}

// NormalizeContactField 规范化联系人字段
// 去除前后空格、统一大小写、清洗格式
func NormalizeContactField(value string, fieldType string) string {
	value = strings.TrimSpace(value)

	switch fieldType {
	case "email":
		return strings.ToLower(value)
	case "phone":
		return normalizePhone(value)
	case "country":
		return strings.ToUpper(value)
	case "name", "organization":
		return normalizeName(value)
	default:
		return value
	}
}

// normalizePhone 规范化电话号码
var phoneCleanRegex = regexp.MustCompile(`[^\d+\-()]`)

func normalizePhone(phone string) string {
	// 保留数字、+、-、括号
	return phoneCleanRegex.ReplaceAllString(phone, "")
}

// normalizeName 规范化名称
func normalizeName(name string) string {
	// 去除多余空格
	name = strings.TrimSpace(name)
	// 去除全大写的格式（某些WHOIS服务器返回全大写）
	// 但保留正常大小写的名称
	parts := strings.Fields(name)
	for i, part := range parts {
		// 如果全是小写字母以外的字符且长度>2，可能需要大小写修正
		if len(part) > 2 && isAllUpper(part) {
			parts[i] = strings.Title(strings.ToLower(part))
		}
	}
	return strings.Join(parts, " ")
}

// isAllUpper 检查字符串是否全部大写（忽略非字母）
func isAllUpper(s string) bool {
	hasLetter := false
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			return false
		}
		if c >= 'A' && c <= 'Z' {
			hasLetter = true
		}
	}
	return hasLetter
}
