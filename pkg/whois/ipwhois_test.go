package whois

import (
	"testing"
)

func TestQueryIP_EmptyIP(t *testing.T) {
	_, err := QueryIP("")
	if err == nil {
		t.Error("Expected error for empty IP")
	}
}

func TestQueryIPWithOptions_NilOpts(t *testing.T) {
	_, err := QueryIPWithOptions(nil)
	if err == nil {
		t.Error("Expected error for nil options")
	}
}

func TestQueryIPWithContext_EmptyIP(t *testing.T) {
	_, err := QueryIPWithContext(nil, &IPWhoisOptions{IP: ""})
	if err == nil {
		t.Error("Expected error for empty IP")
	}
}

func TestQueryIPWithContext_InvalidIP(t *testing.T) {
	_, err := QueryIPWithOptions(&IPWhoisOptions{IP: "not-an-ip"})
	if err == nil {
		t.Error("Expected error for invalid IP")
	}
}

func TestIPWhoisResult_Fields(t *testing.T) {
	result := &IPWhoisResult{
		IP:          "8.8.8.8",
		RawResponse: "raw data",
		Server:      "whois.arin.net",
		Latency:     150,
	}

	if result.IP != "8.8.8.8" {
		t.Errorf("IP = %s, want 8.8.8.8", result.IP)
	}
	if result.Server != "whois.arin.net" {
		t.Errorf("Server = %s, want whois.arin.net", result.Server)
	}
	if result.Latency != 150 {
		t.Errorf("Latency = %d, want 150", result.Latency)
	}
}

func TestIPWhoisOptions_Fields(t *testing.T) {
	opts := &IPWhoisOptions{
		IP:       "8.8.8.8",
		Timeout:  5,
		UseProxy: true,
	}

	if opts.IP != "8.8.8.8" {
		t.Errorf("IP = %s, want 8.8.8.8", opts.IP)
	}
	if opts.Timeout != 5 {
		t.Errorf("Timeout = %d, want 5", opts.Timeout)
	}
	if !opts.UseProxy {
		t.Error("UseProxy should be true")
	}
}

func TestIPWhoisOptions_DefaultTimeout(t *testing.T) {
	opts := &IPWhoisOptions{IP: "8.8.8.8"}
	if opts.Timeout != 0 {
		t.Errorf("Default Timeout = %d, want 0 (will be set to 10 in function)", opts.Timeout)
	}
}
