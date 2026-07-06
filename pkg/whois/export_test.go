package whois

import (
	"bytes"
	"io"
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

// ---- 导出器注册表测试 ----

// TestExporterRegistryBuiltin 验证内置导出器已注册。
func TestExporterRegistryBuiltin(t *testing.T) {
	for _, format := range []string{"json", "csv", "markdown"} {
		e, ok := GetExporter(format)
		if !ok {
			t.Errorf("内置格式 %s 应已注册", format)
		}
		if e.Format() != format {
			t.Errorf("Format() 应返回 %s，得到 %s", format, e.Format())
		}
	}
}

// TestListExporters 验证列出已注册格式。
func TestListExporters(t *testing.T) {
	formats := ListExporters()
	if len(formats) < 3 {
		t.Errorf("应至少注册 3 种格式，得到 %d: %v", len(formats), formats)
	}
}

// TestExportWith 验证按格式名分发导出。
func TestExportWith(t *testing.T) {
	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "example.com"},
	}

	var buf bytes.Buffer
	if err := ExportWith(info, "json", &buf); err != nil {
		t.Fatalf("ExportWith json 失败: %v", err)
	}
	if !strings.Contains(buf.String(), "example.com") {
		t.Errorf("json 导出应包含域名: %s", buf.String())
	}

	buf.Reset()
	if err := ExportWith(info, "markdown", &buf); err != nil {
		t.Fatalf("ExportWith markdown 失败: %v", err)
	}
}

// TestExportWithUnknownFormat 验证未知格式返回错误。
func TestExportWithUnknownFormat(t *testing.T) {
	info := &whoisparser.WhoisInfo{}
	var buf bytes.Buffer
	if err := ExportWith(info, "unknown-format", &buf); err == nil {
		t.Error("未知格式应返回错误")
	}
}

// TestRegisterCustomExporter 验证注册自定义导出器。
func TestRegisterCustomExporter(t *testing.T) {
	// 保存并恢复（注册表是全局的）
	RegisterExporter(&stubExporter{format: "stub", output: "STUB OUTPUT"})
	defer UnregisterExporter("stub")

	e, ok := GetExporter("stub")
	if !ok {
		t.Fatal("自定义导出器应已注册")
	}

	info := &whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "x.com"},
	}
	var buf bytes.Buffer
	if err := e.Export(info, &buf); err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if buf.String() != "STUB OUTPUT" {
		t.Errorf("导出内容不匹配: %s", buf.String())
	}
}

// TestUnregisterExporter 验证注销导出器。
func TestUnregisterExporter(t *testing.T) {
	RegisterExporter(&stubExporter{format: "tmp", output: "x"})
	UnregisterExporter("tmp")
	if _, ok := GetExporter("tmp"); ok {
		t.Error("注销后应不存在")
	}
}

// stubExporter 测试用桩导出器。
type stubExporter struct {
	format string
	output string
}

func (s *stubExporter) Format() string { return s.format }
func (s *stubExporter) Export(info *whoisparser.WhoisInfo, w io.Writer) error {
	_, err := io.WriteString(w, s.output)
	return err
}
