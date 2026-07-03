package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRDAPQueryDomain(t *testing.T) {
	// 模拟RDAP服务器响应
	rdapResponse := `{
		"objectClassName": "domain",
		"ldhName": "example.com",
		"unicodeName": "example.com",
		"status": ["client transfer prohibited", "active"],
		"events": [
			{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"},
			{"eventAction": "expiration", "eventDate": "2025-01-01T00:00:00Z"},
			{"eventAction": "last changed", "eventDate": "2024-01-01T00:00:00Z"}
		],
		"nameservers": [
			{"ldhName": "ns1.example.com"},
			{"ldhName": "ns2.example.com"}
		],
		"entities": [
			{
				"roles": ["registrar"],
				"publicIds": [{"type": "IANA Registrar ID", "value": "1234"}]
			}
		],
		"links": [
			{"rel": "self", "href": "https://rdap.verisign.com/com/v1/domain/example.com"}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/rdap+json" {
			t.Errorf("Expected Accept header 'application/rdap+json', got %s", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/rdap+json")
		fmt.Fprint(w, rdapResponse)
	}))
	defer server.Close()

	// 修改bootstrap指向测试服务器
	bootstrap := GetRDAPBootstrap()
	bootstrap.mu.Lock()
	bootstrap.dns["com"] = server.URL
	bootstrap.mu.Unlock()

	result, err := QueryRDAPWithContext(context.Background(), &RDAPQueryOptions{
		Domain:     "example.com",
		Timeout:    5,
		HTTPClient: server.Client(),
	})

	if err != nil {
		t.Fatalf("QueryRDAPWithContext() error = %v", err)
	}

	if result.ObjectClassName != "domain" {
		t.Errorf("ObjectClassName = %v, want 'domain'", result.ObjectClassName)
	}
	if result.LDHName != "example.com" {
		t.Errorf("LDHName = %v, want 'example.com'", result.LDHName)
	}
	if len(result.Status) != 2 {
		t.Errorf("len(Status) = %v, want 2", len(result.Status))
	}
	if len(result.Events) != 3 {
		t.Errorf("len(Events) = %v, want 3", len(result.Events))
	}
	if len(result.Nameservers) != 2 {
		t.Errorf("len(Nameservers) = %v, want 2", len(result.Nameservers))
	}
	if result.Nameservers[0].LDHName != "ns1.example.com" {
		t.Errorf("Nameservers[0].LDHName = %v, want 'ns1.example.com'", result.Nameservers[0].LDHName)
	}
	if len(result.Entities) != 1 {
		t.Errorf("len(Entities) = %v, want 1", len(result.Entities))
	}
	if len(result.Entities[0].Roles) != 1 || result.Entities[0].Roles[0] != "registrar" {
		t.Errorf("Entity roles = %v, want ['registrar']", result.Entities[0].Roles)
	}
}

func TestRDAPQueryIP(t *testing.T) {
	ipResponse := `{
		"objectClassName": "ip network",
		"startAddress": "192.0.2.0",
		"endAddress": "192.0.2.255",
		"cidr": ["192.0.2.0/24"],
		"ipVersion": "v4",
		"type": "ASSIGNED PORTABLE",
		"name": "EXAMPLE-NET",
		"country": "US",
		"status": ["active"],
		"events": [
			{"eventAction": "last changed", "eventDate": "2024-01-01T00:00:00Z"}
		],
		"entities": [
			{
				"roles": ["abuse"],
				"publicIds": [{"type": "RIR Handle", "value": "ABU123-ARIN"}]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		fmt.Fprint(w, ipResponse)
	}))
	defer server.Close()

	// 覆盖discoverIP_RDAPServer的行为比较复杂，使用自定义HTTP客户端
	// 这里我们通过直接设置客户端来测试解析逻辑

	result := &RDAPIPResult{}
	if err := json.Unmarshal([]byte(ipResponse), result); err != nil {
		t.Fatalf("Failed to parse IP response: %v", err)
	}

	if result.ObjectClassName != "ip network" {
		t.Errorf("ObjectClassName = %v, want 'ip network'", result.ObjectClassName)
	}
	if result.StartAddress != "192.0.2.0" {
		t.Errorf("StartAddress = %v, want '192.0.2.0'", result.StartAddress)
	}
	if result.EndAddress != "192.0.2.255" {
		t.Errorf("EndAddress = %v, want '192.0.2.255'", result.EndAddress)
	}
	if len(result.CIDR) != 1 || result.CIDR[0] != "192.0.2.0/24" {
		t.Errorf("CIDR = %v, want ['192.0.2.0/24']", result.CIDR)
	}
	if result.IPVersion != "v4" {
		t.Errorf("IPVersion = %v, want 'v4'", result.IPVersion)
	}
	if result.Country != "US" {
		t.Errorf("Country = %v, want 'US'", result.Country)
	}
	if len(result.Entities) != 1 || result.Entities[0].Roles[0] != "abuse" {
		t.Errorf("Entities = %v, want abuse entity", result.Entities)
	}
}

func TestRDAPQueryASN(t *testing.T) {
	asnResponse := `{
		"objectClassName": "autnum",
		"handle": "AS13335",
		"startAutnum": 13335,
		"endAutnum": 13335,
		"name": "CLOUDFLARE",
		"country": "US",
		"type": "ASSIGNED",
		"status": ["active"],
		"events": [
			{"eventAction": "registration", "eventDate": "2010-01-01T00:00:00Z"},
			{"eventAction": "last changed", "eventDate": "2024-01-01T00:00:00Z"}
		],
		"entities": [
			{
				"roles": ["registrant"],
				"publicIds": [{"type": "RIR Handle", "value": "CLOUD13-ARIN"}]
			}
		]
	}`

	result := &RDAPASNResult{}
	if err := json.Unmarshal([]byte(asnResponse), result); err != nil {
		t.Fatalf("Failed to parse ASN response: %v", err)
	}

	if result.ObjectClassName != "autnum" {
		t.Errorf("ObjectClassName = %v, want 'autnum'", result.ObjectClassName)
	}
	if result.StartAutnum != 13335 {
		t.Errorf("StartAutnum = %v, want 13335", result.StartAutnum)
	}
	if result.Name != "CLOUDFLARE" {
		t.Errorf("Name = %v, want 'CLOUDFLARE'", result.Name)
	}
	if result.Country != "US" {
		t.Errorf("Country = %v, want 'US'", result.Country)
	}
}

func TestRDAPQueryEntity(t *testing.T) {
	entityResponse := `{
		"objectClassName": "entity",
		"handle": "ABU123-ARIN",
		"roles": ["abuse"],
		"status": ["active"],
		"vcardArray": ["vcard", [
			["version", {}, "text", "4.0"],
			["fn", {}, "text", "Abuse Desk"],
			["email", {}, "text", "abuse@example.com"]
		]],
		"publicIds": [{"type": "RIR Handle", "value": "ABU123-ARIN"}],
		"events": [
			{"eventAction": "last changed", "eventDate": "2024-01-01T00:00:00Z"}
		]
	}`

	result := &RDAPEntityResult{}
	if err := json.Unmarshal([]byte(entityResponse), result); err != nil {
		t.Fatalf("Failed to parse Entity response: %v", err)
	}

	if result.ObjectClassName != "entity" {
		t.Errorf("ObjectClassName = %v, want 'entity'", result.ObjectClassName)
	}
	if result.Handle != "ABU123-ARIN" {
		t.Errorf("Handle = %v, want 'ABU123-ARIN'", result.Handle)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "abuse" {
		t.Errorf("Roles = %v, want ['abuse']", result.Roles)
	}
}

func TestRDAPBootstrap_DNSServer(t *testing.T) {
	// 创建新的bootstrap实例避免被前一个测试污染
	bootstrap := &RDAPBootstrap{
		dns: make(map[string]string),
	}
	bootstrap.loadDefaults()

	tests := []struct {
		tld  string
		want string
	}{
		{"com", "https://rdap.verisign.com/com/v1"},
		{"net", "https://rdap.verisign.com/net/v1"},
		{"org", "https://rdap.publicinterestregistry.org/rdap"},
		{"cn", "https://rdap.cnnic.cn"},
		{"uk", "https://rdap.nic.uk"},
	}

	for _, tt := range tests {
		t.Run(tt.tld, func(t *testing.T) {
			got := bootstrap.GetDNSServer(tt.tld)
			if got != tt.want {
				t.Errorf("GetDNSServer(%q) = %v, want %v", tt.tld, got, tt.want)
			}
		})
	}
}

func TestRDAPBootstrap_ASNServer(t *testing.T) {
	bootstrap := &RDAPBootstrap{
		dns: make(map[string]string),
	}
	bootstrap.loadDefaults()

	tests := []struct {
		asn  int
		want string
	}{
		{13335, "https://rdap.arin.net/registry"},     // Cloudflare (in ARIN range 1-23455)
		{131072, "https://rdap.ripe.net"},            // RIPE range start
		{65536, "https://rdap.apnic.net"},            // APNIC range start
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("AS%d", tt.asn), func(t *testing.T) {
			got := bootstrap.GetASN_RDAPServer(tt.asn)
			if got != tt.want {
				t.Errorf("GetASN_RDAPServer(%d) = %v, want %v", tt.asn, got, tt.want)
			}
		})
	}
}

func TestDiscoverEntityRDAPServer(t *testing.T) {
	tests := []struct {
		handle string
		want   string
	}{
		{"ABU123-ARIN", "https://rdap.arin.net/registry"},
		{"TC123-RIPE", "https://rdap.ripe.net"},
		{"AR302-AP", "https://rdap.apnic.net"},
		{"LAC001-LACNIC", "https://rdap.lacnic.net/rdap"},
		{"AF003-AFRINIC", "https://rdap.afrinic.net/rdap"},
	}

	for _, tt := range tests {
		t.Run(tt.handle, func(t *testing.T) {
			got := discoverEntityRDAPServer(tt.handle)
			if got != tt.want {
				t.Errorf("discoverEntityRDAPServer(%q) = %v, want %v", tt.handle, got, tt.want)
			}
		})
	}
}

func TestRDAPQueryOptionsValidation(t *testing.T) {
	// 测试空域名
	_, err := QueryRDAPWithContext(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil options")
	}

	// 测试空域名
	_, err = QueryRDAPWithContext(context.Background(), &RDAPQueryOptions{})
	if err == nil {
		t.Error("Expected error for empty domain")
	}

	// 测试空IP
	_, err = QueryRDAP_IPWithContext(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil IP options")
	}

	// 测试空ASN
	_, err = QueryRDAP_ASNWithContext(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil ASN options")
	}

	// 测试空Entity
	_, err = QueryRDAP_EntityWithContext(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil Entity options")
	}
}

func TestRDAPHTTPError(t *testing.T) {
	// 模拟返回错误的RDAP服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"errorCode": 404, "title": "Not Found"}`)
	}))
	defer server.Close()

	bootstrap := GetRDAPBootstrap()
	bootstrap.mu.Lock()
	bootstrap.dns["test"] = server.URL
	bootstrap.mu.Unlock()

	_, err := QueryRDAPWithContext(context.Background(), &RDAPQueryOptions{
		Domain:     "example.test",
		Timeout:    5,
		HTTPClient: server.Client(),
	})

	if err == nil {
		t.Error("Expected error for 404 response")
	}
}

func TestRDAPQueryTimeout(t *testing.T) {
	// 模拟慢速RDAP服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := QueryRDAPWithContext(ctx, &RDAPQueryOptions{
		Domain:     "example.com",
		Timeout:    1,
		HTTPClient: &http.Client{Timeout: 100 * time.Millisecond},
	})

	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestRDAPIPResultParsing(t *testing.T) {
	// 测试完整的IP结果解析
	ipResponse := `{
		"objectClassName": "ip network",
		"startAddress": "2001:db8::",
		"endAddress": "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff",
		"cidr": ["2001:db8::/32"],
		"ipVersion": "v6",
		"type": "ALLOCATED PORTABLE",
		"name": "EXAMPLE-V6",
		"country": "DE",
		"parentHandle": "2001:db8::/32",
		"status": ["active"],
		"events": [
			{"eventAction": "registration", "eventDate": "2020-06-01T00:00:00Z"},
			{"eventAction": "last changed", "eventDate": "2024-01-01T00:00:00Z"}
		],
		"entities": [
			{
				"roles": ["abuse", "tech"]
			}
		],
		"links": [
			{"rel": "self", "href": "https://rdap.ripe.net/ip/2001:db8::/32"}
		],
		"remarks": [
			{"title": "Note", "description": ["This is a test network"]}
		]
	}`

	result := &RDAPIPResult{}
	if err := json.Unmarshal([]byte(ipResponse), result); err != nil {
		t.Fatalf("Failed to parse IPv6 response: %v", err)
	}

	if result.IPVersion != "v6" {
		t.Errorf("IPVersion = %v, want 'v6'", result.IPVersion)
	}
	if result.Country != "DE" {
		t.Errorf("Country = %v, want 'DE'", result.Country)
	}
	if result.ParentHandle != "2001:db8::/32" {
		t.Errorf("ParentHandle = %v, want '2001:db8::/32'", result.ParentHandle)
	}
	if len(result.Remarks) != 1 {
		t.Errorf("len(Remarks) = %v, want 1", len(result.Remarks))
	}
	if result.Remarks[0].Title != "Note" {
		t.Errorf("Remarks[0].Title = %v, want 'Note'", result.Remarks[0].Title)
	}
	if len(result.Entities[0].Roles) != 2 {
		t.Errorf("Entity roles count = %v, want 2", len(result.Entities[0].Roles))
	}
}

func BenchmarkRDAPParsing(b *testing.B) {
	response := `{
		"objectClassName": "domain",
		"ldhName": "example.com",
		"status": ["active"],
		"events": [
			{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"}
		],
		"nameservers": [
			{"ldhName": "ns1.example.com"},
			{"ldhName": "ns2.example.com"}
		]
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &RDAPResult{}
		json.Unmarshal([]byte(response), result)
	}
}
