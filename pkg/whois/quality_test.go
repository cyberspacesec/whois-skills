package whois

import (
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

func TestAssessQuality_NilInput(t *testing.T) {
	score := AssessQuality(nil)
	if score.Total != 0 {
		t.Errorf("Expected total 0 for nil input, got %d", score.Total)
	}
	if score.Level != QualityLevelUnusable {
		t.Errorf("Expected unusable level, got %s", score.Level)
	}
}

func TestAssessQuality_CompleteData(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			UpdatedDate:    "2024-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com", "ns2.example.com"},
		},
		Registrar: &whoisparser.Contact{
			Name:  "Example Registrar",
			Email: "registrar@example.com",
		},
		Registrant: &whoisparser.Contact{
			Name:         "John Doe",
			Organization: "Example Corp",
			Email:        "john@example.com",
			Country:      "US",
			Phone:        "+1.5551234567",
		},
		Administrative: &whoisparser.Contact{
			Name:  "Admin Contact",
			Email: "admin@example.com",
		},
		Technical: &whoisparser.Contact{
			Name:  "Tech Contact",
			Email: "tech@example.com",
		},
	}

	score := AssessQuality(info)

	if score.Total < 80 {
		t.Errorf("Expected high score for complete data, got %d", score.Total)
	}
	if score.Level != QualityLevelExcellent {
		t.Errorf("Expected excellent level, got %s", score.Level)
	}
	if score.Completeness < 90 {
		t.Errorf("Expected high completeness, got %d", score.Completeness)
	}
	if len(score.MissingFields) > 0 {
		t.Errorf("Expected no missing fields for complete data, got %v", score.MissingFields)
	}
}

func TestAssessQuality_MinimalData(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain: "example.com",
		},
	}

	score := AssessQuality(info)

	if score.Total > 60 {
		t.Errorf("Expected low score for minimal data, got %d", score.Total)
	}
	if len(score.MissingFields) == 0 {
		t.Error("Expected missing fields for minimal data")
	}
}

func TestAssessQuality_PrivacyProtected(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com"},
		},
		Registrar: &whoisparser.Contact{
			Name: "GoDaddy",
		},
		Registrant: &whoisparser.Contact{
			Name:         "Registration Private",
			Organization: "Domains By Proxy, LLC",
			Email:        "proxy@domainsbyproxy.com",
			Country:      "US",
		},
	}

	score := AssessQuality(info)

	if score.PrivacyDetection == nil {
		t.Fatal("PrivacyDetection should not be nil")
	}
	if !score.PrivacyDetection.HasPrivacy {
		t.Error("Expected privacy to be detected")
	}
	if score.PrivacyDetection.Provider == "" {
		t.Error("Expected privacy provider to be identified")
	}
	if len(score.PrivacyDetection.ProtectedFields) == 0 {
		t.Error("Expected protected fields to be identified")
	}
	// Privacy should reduce reliability score
	if score.Reliability >= 100 {
		t.Errorf("Expected reduced reliability due to privacy, got %d", score.Reliability)
	}
}

func TestDetectPrivacy_GDPRRedacted(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{
			Name:         "REDACTED FOR PRIVACY",
			Organization: "REDACTED FOR PRIVACY",
			Email:        "please query the rdds service",
		},
	}

	detection := detectPrivacy(info)

	if !detection.HasPrivacy {
		t.Error("Expected GDPR redaction to be detected")
	}

	found := false
	for _, pt := range detection.Types {
		if pt == PrivacyRedacted {
			found = true
		}
	}
	if !found {
		t.Error("Expected PrivacyRedacted type")
	}
}

func TestDetectPrivacy_NoPrivacy(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{
			Name:         "John Doe",
			Organization: "Acme Corp",
			Email:        "john@acme.com",
		},
	}

	detection := detectPrivacy(info)

	if detection.HasPrivacy {
		t.Error("Expected no privacy detection for normal data")
	}
}

func TestDetectPrivacy_ContactPrivacy(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{
			Name:         "Contact Privacy Inc. Customer",
			Organization: "Contact Privacy Inc.",
			Email:        "user@contactprivacy.com",
		},
	}

	detection := detectPrivacy(info)

	if !detection.HasPrivacy {
		t.Error("Expected contact privacy to be detected")
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"john@example.com", true},
		{"invalid", false},
		{"", false},
		{"test@test.co.uk", true},
		{"@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isValidEmail(tt.email)
			if result != tt.valid {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, result, tt.valid)
			}
		})
	}
}

func TestIsTemplateData(t *testing.T) {
	tests := []struct {
		name     string
		info     *whoisparser.WhoisInfo
		expected bool
	}{
		{
			name: "normal data",
			info: &whoisparser.WhoisInfo{
				Registrant: &whoisparser.Contact{
					Name:  "John Doe",
					Email: "john@example.com",
				},
			},
			expected: false,
		},
		{
			name: "N/A name",
			info: &whoisparser.WhoisInfo{
				Registrant: &whoisparser.Contact{
					Name: "N/A",
				},
			},
			expected: true,
		},
		{
			name: "test email",
			info: &whoisparser.WhoisInfo{
				Registrant: &whoisparser.Contact{
					Email: "test",
				},
			},
			expected: true,
		},
		{
			name: "none organization",
			info: &whoisparser.WhoisInfo{
				Registrant: &whoisparser.Contact{
					Organization: "none",
				},
			},
			expected: true,
		},
		{
			name:     "nil info",
			info:     nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTemplateData(tt.info)
			if result != tt.expected {
				t.Errorf("isTemplateData() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetermineQualityLevel(t *testing.T) {
	tests := []struct {
		score int
		want  QualityLevel
	}{
		{90, QualityLevelExcellent},
		{80, QualityLevelExcellent},
		{70, QualityLevelGood},
		{60, QualityLevelGood},
		{50, QualityLevelFair},
		{40, QualityLevelFair},
		{30, QualityLevelPoor},
		{20, QualityLevelPoor},
		{10, QualityLevelUnusable},
		{0, QualityLevelUnusable},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.score)), func(t *testing.T) {
			result := determineQualityLevel(tt.score)
			if result != tt.want {
				t.Errorf("determineQualityLevel(%d) = %v, want %v", tt.score, result, tt.want)
			}
		})
	}
}

func TestNormalizeContactField(t *testing.T) {
	tests := []struct {
		value     string
		fieldType string
		want      string
	}{
		{"  John@Example.COM  ", "email", "john@example.com"},
		{"us", "country", "US"},
		{" +1 (555) 123-4567 ", "phone", "+1(555)123-4567"},
		{"  Hello   World  ", "name", "Hello World"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			result := NormalizeContactField(tt.value, tt.fieldType)
			if result != tt.want {
				t.Errorf("NormalizeContactField(%q, %q) = %q, want %q", tt.value, tt.fieldType, result, tt.want)
			}
		})
	}
}

func TestAssessQuality_InvalidEmail(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
		Registrant: &whoisparser.Contact{
			Name:  "John Doe",
			Email: "not-an-email",
		},
	}

	score := AssessQuality(info)

	found := false
	for _, issue := range score.Issues {
		if issue.Type == IssueInvalidFormat {
			found = true
		}
	}
	if !found {
		t.Error("Expected invalid format issue for bad email")
	}
}

func TestAssessQuality_AllPrivacyTypes(t *testing.T) {
	// 测试多种隐私保护类型的识别
	tests := []struct {
		name         string
		contact      *whoisparser.Contact
		expectPrivacy bool
		expectType    PrivacyType
	}{
		{
			name: "Domains By Proxy",
			contact: &whoisparser.Contact{
				Organization: "Domains By Proxy, LLC",
			},
			expectPrivacy: true,
			expectType:    PrivacyDomainsByProxy,
		},
		{
			name: "Registration Private",
			contact: &whoisparser.Contact{
				Name: "Registration Private",
			},
			expectPrivacy: true,
			expectType:    PrivacyWHOISPrivacy,
		},
		{
			name: "Withheld for Privacy",
			contact: &whoisparser.Contact{
				Name: "Withheld for Privacy Purposes",
			},
			expectPrivacy: true,
			expectType:    PrivacyWHOISPrivacy,
		},
		{
			name: "Statutory Masking",
			contact: &whoisparser.Contact{
				Name: "Statutory Masking for GDPR",
			},
			expectPrivacy: true,
			expectType:    PrivacyRedacted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &whoisparser.WhoisInfo{
				Registrant: tt.contact,
			}
			detection := detectPrivacy(info)
			if detection.HasPrivacy != tt.expectPrivacy {
				t.Errorf("HasPrivacy = %v, want %v", detection.HasPrivacy, tt.expectPrivacy)
			}
			if tt.expectPrivacy {
				found := false
				for _, pt := range detection.Types {
					if pt == tt.expectType {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected type %v in %v", tt.expectType, detection.Types)
				}
			}
		})
	}
}

func BenchmarkAssessQuality(b *testing.B) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			UpdatedDate:    "2024-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com"},
		},
		Registrar: &whoisparser.Contact{
			Name:  "Example Registrar",
			Email: "registrar@example.com",
		},
		Registrant: &whoisparser.Contact{
			Name:         "John Doe",
			Organization: "Example Corp",
			Email:        "john@example.com",
			Country:      "US",
		},
	}
	for i := 0; i < b.N; i++ {
		AssessQuality(info)
	}
}
