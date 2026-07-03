package whois

import (
	"bytes"
	"strings"
	"testing"

	whoisparser "github.com/likexian/whois-parser"
)

func TestExportToJSON_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToJSON(nil, &buf)
	if err == nil {
		t.Error("Expected error for nil info")
	}
}

func TestExportToJSON_Valid(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
	}
	var buf bytes.Buffer
	err := ExportToJSON(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "example.com") {
		t.Error("JSON output should contain domain name")
	}
	if !strings.Contains(buf.String(), "2020-01-01") {
		t.Error("JSON output should contain created date")
	}
}

func TestExportToJSON_WithContacts(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
		Registrant: &whoisparser.Contact{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}
	var buf bytes.Buffer
	err := ExportToJSON(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "John Doe") {
		t.Error("JSON output should contain registrant name")
	}
}

func TestExportToCSV_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToCSV(nil, &buf)
	if err == nil {
		t.Error("Expected error for nil info")
	}
}

func TestExportToCSV_Valid(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
			NameServers:    []string{"ns1.example.com"},
		},
	}
	var buf bytes.Buffer
	err := ExportToCSV(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Field") {
		t.Error("CSV output should contain header")
	}
	if !strings.Contains(buf.String(), "example.com") {
		t.Error("CSV output should contain domain name")
	}
}

func TestExportToCSV_WithContacts(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
		Registrar: &whoisparser.Contact{
			Name:  "Example Registrar",
			Email: "registrar@example.com",
		},
		Registrant: &whoisparser.Contact{
			Name:  "John Doe",
			Email: "john@example.com",
		},
	}
	var buf bytes.Buffer
	err := ExportToCSV(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "John Doe") {
		t.Error("CSV output should contain registrant name")
	}
	if !strings.Contains(buf.String(), "Example Registrar") {
		t.Error("CSV output should contain registrar name")
	}
}

func TestExportToMarkdown_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := ExportToMarkdown(nil, &buf)
	if err == nil {
		t.Error("Expected error for nil info")
	}
}

func TestExportToMarkdown_Valid(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
			Status:         []string{"clientTransferProhibited"},
		},
	}
	var buf bytes.Buffer
	err := ExportToMarkdown(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "example.com") {
		t.Error("Markdown output should contain domain name")
	}
	if !strings.Contains(buf.String(), "WHOIS") {
		t.Error("Markdown output should contain WHOIS header")
	}
	if !strings.Contains(buf.String(), "|") {
		t.Error("Markdown output should contain table")
	}
}

func TestExportToMarkdown_WithContacts(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
		Registrant: &whoisparser.Contact{
			Name:         "John Doe",
			Organization: "Example Corp",
			Email:        "john@example.com",
		},
	}
	var buf bytes.Buffer
	err := ExportToMarkdown(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "John Doe") {
		t.Error("Markdown output should contain registrant name")
	}
	if !strings.Contains(buf.String(), "Example Corp") {
		t.Error("Markdown output should contain organization")
	}
}

func TestExportToMarkdown_NoDomain(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Name: "John"},
	}
	var buf bytes.Buffer
	err := ExportToMarkdown(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Should still produce output with contact info
	if !strings.Contains(buf.String(), "John") {
		t.Error("Markdown output should contain registrant name")
	}
}

func TestExportToCSV_NoDomain(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Registrant: &whoisparser.Contact{Name: "John"},
	}
	var buf bytes.Buffer
	err := ExportToCSV(info, &buf)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "John") {
		t.Error("CSV output should contain registrant name")
	}
}

func BenchmarkExportToJSON(b *testing.B) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
		Registrant: &whoisparser.Contact{Name: "John", Email: "john@example.com"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		ExportToJSON(info, &buf)
	}
}

func BenchmarkExportToCSV(b *testing.B) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
		Registrant: &whoisparser.Contact{Name: "John", Email: "john@example.com"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		ExportToCSV(info, &buf)
	}
}

func BenchmarkExportToMarkdown(b *testing.B) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{
			Domain:         "example.com",
			CreatedDate:    "2020-01-01",
			ExpirationDate: "2025-01-01",
		},
		Registrant: &whoisparser.Contact{Name: "John", Email: "john@example.com"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		ExportToMarkdown(info, &buf)
	}
}
