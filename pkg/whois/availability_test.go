package whois

import (
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

func TestIsParserError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{"nil err", nil, whoisparser.ErrNotFoundDomain, false},
		{"nil target", whoisparser.ErrNotFoundDomain, nil, false},
		{"both nil", nil, nil, false},
		{"matching", whoisparser.ErrNotFoundDomain, whoisparser.ErrNotFoundDomain, true},
		{"not matching", whoisparser.ErrNotFoundDomain, whoisparser.ErrReservedDomain, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isParserError(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("isParserError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDomainAvailability_Fields(t *testing.T) {
	avail := &DomainAvailability{
		Domain:    "example.com",
		Available: true,
		Status:    "available",
		Message:   "域名可以注册",
	}

	if avail.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", avail.Domain)
	}
	if !avail.Available {
		t.Error("Available should be true")
	}
	if avail.Status != "available" {
		t.Errorf("Status = %s, want available", avail.Status)
	}
}

func TestCheckDomainAvailability_EmptyDomain(t *testing.T) {
	_, err := CheckDomainAvailability("")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func TestCheckDomainAvailabilityWithContext_EmptyDomain(t *testing.T) {
	_, err := CheckDomainAvailabilityWithContext(nil, "")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func TestDomainAvailability_Statuses(t *testing.T) {
	statuses := []string{"available", "registered", "reserved", "premium", "blocked", "unknown"}
	for _, status := range statuses {
		avail := &DomainAvailability{Status: status}
		if avail.Status != status {
			t.Errorf("Status = %s, want %s", avail.Status, status)
		}
	}
}

func BenchmarkIsParserError(b *testing.B) {
	err := whoisparser.ErrNotFoundDomain
	target := whoisparser.ErrNotFoundDomain
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isParserError(err, target)
	}
}
