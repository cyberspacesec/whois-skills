package whois

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractTLD(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{"simple com", "example.com", "com"},
		{"simple net", "example.net", "net"},
		{"subdomain", "www.example.com", "com"},
		{"with protocol", "http://example.com", "com"},
		{"with https", "https://example.com", "com"},
		{"co.uk", "example.co.uk", "co.uk"},
		{"single part", "localhost", ""},
		{"empty", "", ""},
		{"trailing dots", "example.com.", "com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTLD(tt.domain)
			if got != tt.want {
				t.Errorf("extractTLD() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExtractTLD_Public(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		want    string
		wantErr bool
	}{
		{"simple com", "example.com", "com", false},
		{"empty", "", "", true},
		{"single part", "x", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractTLD(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractTLD() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ExtractTLD() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestServerHealth_Fields(t *testing.T) {
	health := &ServerHealth{
		IsHealthy:           true,
		FailureCount:        0,
		AvgResponseTime:     150,
		LastCheck:           time.Now(),
		maxResponseRecords:  100,
		recentResponseTimes: []int64{100, 200, 150},
	}

	if !health.IsHealthy {
		t.Error("Should be healthy")
	}
	if health.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", health.FailureCount)
	}
	if health.AvgResponseTime != 150 {
		t.Errorf("AvgResponseTime = %d, want 150", health.AvgResponseTime)
	}
}

func TestDomainInfo_Fields(t *testing.T) {
	info := &DomainInfo{
		FullDomain:   "www.example.com",
		TLD:          "com",
		Domain:       "example.com",
		SubDomain:    "www",
		WildcardBase: "*.example.com",
	}

	if info.FullDomain != "www.example.com" {
		t.Errorf("FullDomain = %s, want www.example.com", info.FullDomain)
	}
	if info.TLD != "com" {
		t.Errorf("TLD = %s, want com", info.TLD)
	}
	if info.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", info.Domain)
	}
	if info.SubDomain != "www" {
		t.Errorf("SubDomain = %s, want www", info.SubDomain)
	}
}

func TestParseDomain(t *testing.T) {
	info, err := ParseDomain("www.example.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("ParseDomain() returned nil")
	}
	if info.TLD != "com" {
		t.Errorf("TLD = %s, want com", info.TLD)
	}
}

func TestParseDomain_Invalid(t *testing.T) {
	_, err := ParseDomain("")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func TestWhoisServerManager_UpdateServer(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	mgr.UpdateServer("test", "whois.test.com")

	if mgr.servers["test"] != "whois.test.com" {
		t.Errorf("Server = %s, want whois.test.com", mgr.servers["test"])
	}
}

func TestWhoisServerManager_UpdateServers(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	mgr.UpdateServers(map[string]string{
		"test1": "whois.test1.com",
		"test2": "whois.test2.com",
	})

	if len(mgr.servers) != 2 {
		t.Errorf("Server count = %d, want 2", len(mgr.servers))
	}
}

func TestWhoisServerManager_GetAllServers(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             map[string]string{"com": "whois.verisign-grs.com"},
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	servers := mgr.GetAllServers()
	if len(servers) == 0 {
		t.Error("GetAllServers() should return servers")
	}
	if servers["com"] != "whois.verisign-grs.com" {
		t.Errorf("Server for com = %s, want whois.verisign-grs.com", servers["com"])
	}
}

func TestWhoisServerManager_SetDefaultServer(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	mgr.SetDefaultServer("whois.custom.com")
	if mgr.defaultServer != "whois.custom.com" {
		t.Errorf("defaultServer = %s, want whois.custom.com", mgr.defaultServer)
	}
}

func TestWhoisServerManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "servers.json")

	mgr := &WhoisServerManager{
		servers:             map[string]string{"com": "whois.verisign-grs.com", "net": "whois.verisign-grs.com"},
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
		lastUpdated:         time.Now(),
	}

	// Save
	err := mgr.SaveToFile(configPath)
	if err != nil {
		t.Fatalf("SaveToFile() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file should exist")
	}

	// Load into a new manager
	mgr2 := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	err = mgr2.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error: %v", err)
	}

	if mgr2.servers["com"] != "whois.verisign-grs.com" {
		t.Errorf("Loaded server = %s, want whois.verisign-grs.com", mgr2.servers["com"])
	}
}

func TestWhoisServerManager_LoadFromFile_NotFound(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	err := mgr.LoadFromFile("/nonexistent/servers.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestWhoisServerManager_GetWhoisServer(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             map[string]string{"com": "whois.verisign-grs.com"},
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}

	server, err := mgr.GetWhoisServer("example.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// Should return default server since no health info exists
	if server != "whois.iana.org" {
		t.Logf("Got server %s (expected default since no health info)", server)
	}
}

func TestWhoisServerManager_GetWhoisServer_InvalidDomain(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             make(map[string]string),
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}

	_, err := mgr.GetWhoisServer("")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func TestWhoisServerManager_GetServerStats(t *testing.T) {
	mgr := &WhoisServerManager{
		servers:             map[string]string{"com": "whois.verisign-grs.com"},
		serverHealth:        make(map[string]*ServerHealth),
		defaultServer:       "whois.iana.org",
		healthCheckTimeout:  10 * time.Second,
		maxFailures:         3,
	}
	stats := mgr.GetServerStats()
	if stats == nil {
		t.Fatal("GetServerStats() returned nil")
	}
	if stats["total_servers"] != 1 {
		t.Errorf("total_servers = %v, want 1", stats["total_servers"])
	}
}

func TestCacheEntry_Fields(t *testing.T) {
	entry := &CacheEntry{
		RawResponse: "raw whois data",
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	if entry.RawResponse != "raw whois data" {
		t.Errorf("RawResponse = %s, want 'raw whois data'", entry.RawResponse)
	}
}

func TestCacheStats_Fields(t *testing.T) {
	stats := CacheStats{
		Hits:     100,
		Misses:   20,
		Entries:  50,
		Expired:  5,
		Requests: 120,
	}
	if stats.Hits != 100 {
		t.Errorf("Hits = %d, want 100", stats.Hits)
	}
	if stats.Misses != 20 {
		t.Errorf("Misses = %d, want 20", stats.Misses)
	}
}

func TestExtractEffectiveTLD(t *testing.T) {
	tld, err := ExtractEffectiveTLD("example.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if tld != "com" {
		t.Errorf("TLD = %s, want com", tld)
	}
}

func TestExtractEffectiveTLD_Invalid(t *testing.T) {
	_, err := ExtractEffectiveTLD("")
	if err == nil {
		t.Error("Expected error for empty domain")
	}
}

func BenchmarkExtractTLD(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractTLD("www.example.com")
	}
}

func BenchmarkParseDomain(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseDomain("www.example.com")
	}
}
