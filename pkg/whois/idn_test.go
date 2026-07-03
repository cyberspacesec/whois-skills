package whois

import (
	"testing"
)

func TestPunycodeToUnicode(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"ascii domain", "example.com", "example.com", false},
		{"punycode domain", "xn--1xao.com", "πχ.com", false},
		{"punycode tld", "example.xn--p1ai", "example.рф", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PunycodeToUnicode(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("PunycodeToUnicode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("PunycodeToUnicode() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestUnicodeToPunycode(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"ascii domain", "example.com", "example.com", false},
		{"unicode domain", "πχ.com", "xn--1xao.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnicodeToPunycode(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnicodeToPunycode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("UnicodeToPunycode() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"simple", "example.com", "example.com", false},
		{"with http", "http://example.com", "example.com", false},
		{"with https", "https://example.com", "example.com", false},
		{"with path", "example.com/path", "example.com", false},
		{"with trailing dot", "example.com.", "example.com", false},
		{"uppercase", "EXAMPLE.COM", "example.com", false},
		{"unicode", "πχ.com", "xn--1xao.com", false},
		{"with http and path", "http://example.com/page", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeDomain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizeDomain() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestIsIDN(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{"ascii domain", "example.com", false},
		{"punycode domain", "xn--nxasmq6b.com", true},
		{"unicode domain", "πχ.com", true},
		{"punycode tld", "test.xn--p1ai", false}, // IsIDN checks prefix and non-ASCII, not TLD
		{"simple ascii", "google.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsIDN(tt.domain)
			if got != tt.want {
				t.Errorf("IsIDN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsASCII(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", true},
		{"ascii", "hello", true},
		{"unicode", "héllo", false},
		{"mixed", "hello世界", false},
		{"numbers", "123", true},
		{"symbols", "!@#$%", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isASCII(tt.input)
			if got != tt.want {
				t.Errorf("isASCII() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPunycodeRoundTrip(t *testing.T) {
	unicode := "πχ.com"
	punycode, err := UnicodeToPunycode(unicode)
	if err != nil {
		t.Fatalf("UnicodeToPunycode() error: %v", err)
	}
	back, err := PunycodeToUnicode(punycode)
	if err != nil {
		t.Fatalf("PunycodeToUnicode() error: %v", err)
	}
	if back != unicode {
		t.Errorf("Round trip: got %s, want %s", back, unicode)
	}
}

func BenchmarkNormalizeDomain(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeDomain("https://EXAMPLE.COM/path")
	}
}

func BenchmarkIsIDN(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsIDN("xn--1xao.com")
	}
}
