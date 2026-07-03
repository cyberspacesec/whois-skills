package whois

import (
	"strings"
	"testing"
)

func TestDetectWhoisFormat_ARIN(t *testing.T) {
	response := "American Registry for Internet Numbers ARIN Whois"
	format := DetectWhoisFormat(response)
	if format != FormatARIN {
		t.Errorf("Format = %s, want arin", format)
	}
}

func TestDetectWhoisFormat_RIPE(t *testing.T) {
	response := "RIPE Network Coordination Centre"
	format := DetectWhoisFormat(response)
	if format != FormatRIPE {
		t.Errorf("Format = %s, want ripe", format)
	}
}

func TestDetectWhoisFormat_APNIC(t *testing.T) {
	response := "Asia Pacific Network Information Centre APNIC"
	format := DetectWhoisFormat(response)
	if format != FormatAPNIC {
		t.Errorf("Format = %s, want apnic", format)
	}
}

func TestDetectWhoisFormat_LACNIC(t *testing.T) {
	response := "Latin American and Caribbean Internet Addresses Registry LACNIC"
	format := DetectWhoisFormat(response)
	if format != FormatLACNIC {
		t.Errorf("Format = %s, want lacnic", format)
	}
}

func TestDetectWhoisFormat_AFRINIC(t *testing.T) {
	response := "AFRINIC whois database"
	format := DetectWhoisFormat(response)
	if format != FormatAFRINIC {
		t.Errorf("Format = %s, want afrinic", format)
	}
}

func TestDetectWhoisFormat_Verisign(t *testing.T) {
	response := "Verisign, Inc. WHOIS server"
	format := DetectWhoisFormat(response)
	if format != FormatVerisign {
		t.Errorf("Format = %s, want verisign", format)
	}
}

func TestDetectWhoisFormat_PIR(t *testing.T) {
	response := "Public Interest Registry WHOIS"
	format := DetectWhoisFormat(response)
	if format != FormatPIR {
		t.Errorf("Format = %s, want pir", format)
	}
}

func TestDetectWhoisFormat_Generic(t *testing.T) {
	response := "Some random WHOIS response"
	format := DetectWhoisFormat(response)
	if format != FormatGeneric {
		t.Errorf("Format = %s, want generic", format)
	}
}

func TestDetectWhoisFormat_Empty(t *testing.T) {
	format := DetectWhoisFormat("")
	if format != FormatGeneric {
		t.Errorf("Format = %s, want generic for empty", format)
	}
}

func TestDetectWhoisFormat_CaseInsensitive(t *testing.T) {
	response := "ARIN WHOIS and American Registry"
	format := DetectWhoisFormat(strings.ToUpper(response))
	if format != FormatARIN {
		t.Errorf("Format = %s, want arin (case insensitive)", format)
	}
}

func TestFormatRawResponse_Comments(t *testing.T) {
	input := `% This is a comment
domain: example.com
# Another comment
registrar: Example

name: John`
	expected := `domain: example.com
registrar: Example

name: John`
	result := FormatRawResponse(input)
	if result != expected {
		t.Errorf("Result = %q, want %q", result, expected)
	}
}

func TestFormatRawResponse_TrailingEmptyLines(t *testing.T) {
	input := "domain: example.com\n\n\n"
	result := FormatRawResponse(input)
	if strings.HasSuffix(result, "\n\n") {
		t.Error("Should not have trailing empty lines")
	}
}

func TestFormatRawResponse_MultipleEmptyLines(t *testing.T) {
	input := "domain: example.com\n\n\nregistrar: test"
	result := FormatRawResponse(input)
	// Multiple empty lines should be collapsed to one
	if strings.Contains(result, "\n\n\n") {
		t.Error("Should not have multiple consecutive empty lines")
	}
}

func TestFormatRawResponse_Empty(t *testing.T) {
	result := FormatRawResponse("")
	if result != "" {
		t.Errorf("Result = %q, want empty", result)
	}
}

func TestFormatRawResponse_OnlyComments(t *testing.T) {
	input := "% comment1\n% comment2\n# comment3"
	result := FormatRawResponse(input)
	if result != "" {
		t.Errorf("Result = %q, want empty for only comments", result)
	}
}

func TestFormatRawResponse_PreserveData(t *testing.T) {
	input := "domain: example.com\nregistrar: Test\nname: John Doe"
	result := FormatRawResponse(input)
	if result != input {
		t.Errorf("Result = %q, want %q", result, input)
	}
}

func TestWhoisFormat_Constants(t *testing.T) {
	formats := map[WhoisFormat]string{
		FormatUnknown:  "unknown",
		FormatARIN:     "arin",
		FormatRIPE:     "ripe",
		FormatAPNIC:    "apnic",
		FormatLACNIC:   "lacnic",
		FormatAFRINIC:  "afrinic",
		FormatVerisign: "verisign",
		FormatPIR:      "pir",
		FormatGeneric:  "generic",
	}
	for format, want := range formats {
		if string(format) != want {
			t.Errorf("Format = %s, want %s", format, want)
		}
	}
}

func BenchmarkDetectWhoisFormat(b *testing.B) {
	response := "American Registry for Internet Numbers ARIN Whois database"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectWhoisFormat(response)
	}
}

func BenchmarkFormatRawResponse(b *testing.B) {
	input := "% comment\ndomain: example.com\nregistrar: Test\n\nname: John\n% end"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatRawResponse(input)
	}
}
