package whois

import (
	"context"
	"testing"
)

func TestQueryASN_InvalidASN(t *testing.T) {
	_, err := QueryASN(0)
	if err == nil {
		t.Error("Expected error for ASN 0")
	}

	_, err = QueryASN(-1)
	if err == nil {
		t.Error("Expected error for negative ASN")
	}
}

func TestQueryASNWithContext_NilOptions(t *testing.T) {
	_, err := QueryASNWithContext(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil options")
	}
}

func TestQueryASNWithContext_InvalidSource(t *testing.T) {
	_, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:    13335,
		Source: "invalid",
	})
	if err == nil {
		t.Error("Expected error for invalid source")
	}
}

func TestASNDetail_Fields(t *testing.T) {
	info := &ASNDetail{
		ASN:          13335,
		ASNString:    "AS13335",
		Name:         "CLOUDFLARE",
		Organization: "Cloudflare, Inc.",
		Country:      "US",
		RIR:          "ARIN",
		Status:       "ASSIGNED",
		IPv4Prefixes: []string{"1.0.0.0/24", "1.1.1.0/24"},
		IPv6Prefixes: []string{"2606:4700::/32"},
	}

	if info.ASN != 13335 {
		t.Errorf("ASN = %d, want 13335", info.ASN)
	}
	if info.ASNString != "AS13335" {
		t.Errorf("ASNString = %s, want AS13335", info.ASNString)
	}
	if len(info.IPv4Prefixes) != 2 {
		t.Errorf("IPv4Prefixes count = %d, want 2", len(info.IPv4Prefixes))
	}
	if len(info.IPv6Prefixes) != 1 {
		t.Errorf("IPv6Prefixes count = %d, want 1", len(info.IPv6Prefixes))
	}
}

func TestExtractRIRFromHandle(t *testing.T) {
	tests := []struct {
		handle string
		want   string
	}{
		{"CLOUD13-ARIN", "ARIN"},
		{"TC123-RIPE", "RIPE NCC"},
		{"AR302-AP", "APNIC"},
		{"LAC001-LACNIC", "LACNIC"},
		{"AF003-AFRINIC", "AFRINIC"},
		{"UNKNOWN-HANDLE", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.handle, func(t *testing.T) {
			got := extractRIRFromHandle(tt.handle)
			if got != tt.want {
				t.Errorf("extractRIRFromHandle(%q) = %q, want %q", tt.handle, got, tt.want)
			}
		})
	}
}

func TestParseASNString(t *testing.T) {
	tests := []struct {
		input string
		want  int
		err   bool
	}{
		{"AS13335", 13335, false},
		{"as13335", 13335, false},
		{"13335", 13335, false},
		{"  AS13335  ", 13335, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseASNString(tt.input)
			if tt.err {
				if err == nil {
					t.Error("Expected error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("ParseASNString(%q) = %d, want %d", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestASNToPrefixCount(t *testing.T) {
	tests := []struct {
		name      string
		info      *ASNDetail
		wantV4    int
		wantV6    int
	}{
		{
			name:   "nil info",
			info:   nil,
			wantV4: 0,
			wantV6: 0,
		},
		{
			name: "with prefixes",
			info: &ASNDetail{
				IPv4Prefixes: []string{"1.0.0.0/24", "1.1.1.0/24", "104.16.0.0/13"},
				IPv6Prefixes: []string{"2606:4700::/32", "2803:f800::/32"},
			},
			wantV4: 3,
			wantV6: 2,
		},
		{
			name: "no prefixes",
			info: &ASNDetail{
				ASN: 12345,
			},
			wantV4: 0,
			wantV6: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v4, v6 := ASNToPrefixCount(tt.info)
			if v4 != tt.wantV4 {
				t.Errorf("IPv4 count = %d, want %d", v4, tt.wantV4)
			}
			if v6 != tt.wantV6 {
				t.Errorf("IPv6 count = %d, want %d", v6, tt.wantV6)
			}
		})
	}
}

func TestASNQueryOptions_Fields(t *testing.T) {
	opts := &ASNQueryOptions{
		ASN:             13335,
		Timeout:         10,
		Source:          ASNSourceAll,
		IncludePrefixes: true,
		IncludeBGP:      false,
	}

	if opts.ASN != 13335 {
		t.Errorf("ASN = %d, want 13335", opts.ASN)
	}
	if opts.Source != ASNSourceAll {
		t.Errorf("Source = %s, want %s", opts.Source, ASNSourceAll)
	}
	if !opts.IncludePrefixes {
		t.Error("IncludePrefixes should be true")
	}
}

func TestASNQuerySource_Constants(t *testing.T) {
	if ASNSourceRADB != "radb" {
		t.Errorf("ASNSourceRADB = %s, want radb", ASNSourceRADB)
	}
	if ASNSourceRDAP != "rdap" {
		t.Errorf("ASNSourceRDAP = %s, want rdap", ASNSourceRDAP)
	}
	if ASNSourceAll != "all" {
		t.Errorf("ASNSourceAll = %s, want all", ASNSourceAll)
	}
}

func TestASNBatchResult_Fields(t *testing.T) {
	result := &ASNBatchResult{
		Results:      make(map[int]*ASNDetail),
		Errors:       make(map[int]error),
		TotalQueried: 5,
		SuccessCount: 3,
		FailureCount: 2,
	}

	if result.TotalQueried != 5 {
		t.Errorf("TotalQueried = %d, want 5", result.TotalQueried)
	}
	if result.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", result.SuccessCount)
	}
}

func TestBatchQueryASN_Empty(t *testing.T) {
	result := BatchQueryASN(context.Background(), []int{}, 5)

	if result.TotalQueried != 0 {
		t.Errorf("TotalQueried = %d, want 0", result.TotalQueried)
	}
	if len(result.Results) != 0 {
		t.Errorf("Results count = %d, want 0", len(result.Results))
	}
}

func TestASNCache(t *testing.T) {
	// Clear cache first
	ClearASNDetailCache()

	// Manually populate cache
	asnDetailCache.mu.Lock()
	asnDetailCache.items[99999] = &ASNDetail{
		ASN:       99999,
		ASNString: "AS99999",
		Name:      "Test AS",
		Country:   "US",
	}
	asnDetailCache.mu.Unlock()

	// Should get from cache
	info, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:    99999,
		Source: ASNSourceRDAP, // Even with RDAP source, should hit cache
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info.Name != "Test AS" {
		t.Errorf("Name = %s, want 'Test AS'", info.Name)
	}

	// Clean up
	ClearASNDetailCache()

	// Verify cache is cleared
	asnDetailCache.mu.RLock()
	_, exists := asnDetailCache.items[99999]
	asnDetailCache.mu.RUnlock()
	if exists {
		t.Error("Cache should be cleared")
	}
}

func TestGetASNDetailCache(t *testing.T) {
	ClearASNDetailCache()

	asnDetailCache.mu.Lock()
	asnDetailCache.items[12345] = &ASNDetail{ASN: 12345, Name: "Test"}
	asnDetailCache.items[67890] = &ASNDetail{ASN: 67890, Name: "Other"}
	asnDetailCache.mu.Unlock()

	cache := GetASNDetailCache()
	if len(cache) != 2 {
		t.Errorf("Cache size = %d, want 2", len(cache))
	}

	ClearASNDetailCache()
}

func TestASNRelation(t *testing.T) {
	relation := &ASNRelation{
		ASN: 13335,
		Upstream: []ASNPeer{
			{ASN: 174, Name: "Cogent", Source: "bgp"},
		},
		Downstream: []ASNPeer{
			{ASN: 62597, Name: "Downstream AS", Source: "bgp"},
		},
		Peers: []ASNPeer{
			{ASN: 15169, Name: "Google", Source: "bgp"},
		},
	}

	if relation.ASN != 13335 {
		t.Errorf("ASN = %d, want 13335", relation.ASN)
	}
	if len(relation.Upstream) != 1 {
		t.Errorf("Upstream count = %d, want 1", len(relation.Upstream))
	}
	if len(relation.Downstream) != 1 {
		t.Errorf("Downstream count = %d, want 1", len(relation.Downstream))
	}
	if len(relation.Peers) != 1 {
		t.Errorf("Peers count = %d, want 1", len(relation.Peers))
	}
}

func TestASNPeer(t *testing.T) {
	peer := ASNPeer{
		ASN:    15169,
		Name:   "Google LLC",
		Source: "bgp",
	}

	if peer.ASN != 15169 {
		t.Errorf("ASN = %d, want 15169", peer.ASN)
	}
	if peer.Name != "Google LLC" {
		t.Errorf("Name = %s, want 'Google LLC'", peer.Name)
	}
}

func TestQueryASNWithContext_DefaultTimeout(t *testing.T) {
	// This tests that a zero timeout gets defaulted
	// We use a cached ASN to avoid actual network call
	ClearASNDetailCache()

	asnDetailCache.mu.Lock()
	asnDetailCache.items[54321] = &ASNDetail{ASN: 54321, Name: "Cached"}
	asnDetailCache.mu.Unlock()

	info, err := QueryASNWithContext(context.Background(), &ASNQueryOptions{
		ASN:     54321,
		Timeout: 0, // Should be defaulted to 10
		Source:  ASNSourceAll,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if info.Name != "Cached" {
		t.Errorf("Name = %s, want 'Cached'", info.Name)
	}

	ClearASNDetailCache()
}

func BenchmarkParseASNString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseASNString("AS13335")
	}
}

func BenchmarkExtractRIRFromHandle(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractRIRFromHandle("CLOUD13-ARIN")
	}
}

func BenchmarkASNToPrefixCount(b *testing.B) {
	info := &ASNDetail{
		IPv4Prefixes: make([]string, 100),
		IPv6Prefixes: make([]string, 20),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ASNToPrefixCount(info)
	}
}
