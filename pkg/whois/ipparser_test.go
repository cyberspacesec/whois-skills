package whois

import (
	"testing"
)

// ARIN格式的典型响应
var arinResponse = `#
# ARIN WHOIS data and services are subject to the Terms of Use
# available at: https://www.arin.net/resources/registry/whois/tou/
#
# If you see inaccuracies in the results, please report at
# https://www.arin.net/resources/registry/whois/inaccuracy_reporting/
#
# Copyright 1997-2024, American Registry for Internet Numbers, Ltd.
#

NetRange:       192.0.2.0 - 192.0.2.255
CIDR:           192.0.2.0/24
NetName:        EXAMPLE-NET
NetType:        Allocated PA
OriginAS:       AS12345
Organization:   Example Organization (EO-123)
Country:        US
RegDate:        2020-01-01
Updated:        2024-01-01
Ref:            https://whois.arin.net/rest/net/NET-192-0-2-0-1

OrgName:        Example Organization
OrgId:          EO-123
Address:        123 Example Street
City:           Reston
StateProv:      VA
Country:        US

Abuse Handle:   ABU123-ARIN
Abuse Name:     Abuse Desk
Abuse Email:    abuse@example.com
Abuse Phone:    +1-703-555-1234
Tech Handle:    TECH123-ARIN
Tech Name:      Tech Support
Tech Email:     tech@example.com
Tech Phone:     +1-703-555-5678`

// RIPE格式的典型响应
var ripeResponse = `% This is the RIPE Database query service.
% The objects are in RPSL format.
%
% The RIPE Database is subject to Terms and Conditions.
% See http://www.ripe.net/db/support/db-terms-conditions.pdf

inetnum:        193.0.0.0 - 193.0.0.255
netname:        RIPE-NCC-NET
descr:          RIPE Network Coordination Centre
descr:          Amsterdam, Netherlands
country:        NL
org:            ORG-RIEN1-RIPE
admin-c:        AD123-RIPE
tech-c:         TC123-RIPE
abuse-c:        AB123-RIPE
abuse-mailbox:  abuse@ripe.net
status:         ASSIGNED PA
mnt-by:         RIPE-NCC-MNT
created:        2020-01-01T00:00:00Z
last-modified:  2024-06-01T00:00:00Z
source:         RIPE

organisation:   ORG-RIEN1-RIPE
org-name:       RIPE Network Coordination Centre
org-type:       LIR
address:        Stationsplein 11
address:        Amsterdam
country:        NL
phone:          +31 20 535 4444
e-mail:         info@ripe.net
created:        2010-01-01T00:00:00Z
last-modified:  2024-01-01T00:00:00Z
source:         RIPE`

// APNIC格式的典型响应
var apnicResponse = `% Information on APNIC objects

inetnum:        203.0.113.0 - 203.0.113.255
netname:        APNIC-LABS-NET
descr:          APNIC and Laboratory
country:        AU
org:            ORG-ARAO1-AP
admin-c:        AR302-AP
tech-c:         AR302-AP
abuse-c:        AR302-AP
status:         ASSIGNED PORTABLE
mnt-by:         APNIC-HM
created:        2019-01-01T00:00:00Z
last-modified:  2023-12-01T00:00:00Z
source:         APNIC

organisation:   ORG-ARAO1-AP
org-name:       APNIC Research and Development
org-type:       LIR
address:        6 Cordelia St
country:        AU
phone:          +61-7-3858-3100
e-mail:         helpdesk@apnic.net
created:        2010-01-01T00:00:00Z
last-modified:  2024-01-01T00:00:00Z
source:         APNIC`

// LACNIC格式的典型响应
var lacnicResponse = `% LACNIC WHOIS data

inetnum:     200.0.0.0 - 200.0.0.255
owner:       LACNIC Labs
ownerid:     BR-LALA-LACNI
responsible: LACNIC Admin
country:     BR
abuse-c:     LAC001
tech-c:      LAC002
e-mail:      abuse@lacnic.net
created:     20190501
changed:     20240101
source:      LACNIC`

// AFRINIC格式的典型响应
var afrinicResponse = `% AFRINIC WHOIS Database

inetnum:        196.0.0.0 - 196.0.0.255
netname:        AFRINIC-TEST-NET
descr:          AFRINIC Test Network
country:        ZA
org:            ORG-AF1-AFRINIC
admin-c:        AF001-AFRINIC
tech-c:         AF002-AFRINIC
abuse-c:        AF003-AFRINIC
abuse-mailbox:  abuse@afrinic.net
status:         ASSIGNED PA
mnt-by:         AFRINIC-HM
created:        2020-06-01T00:00:00Z
last-modified:  2024-01-01T00:00:00Z
source:         AFRINIC`

func TestDetectRIR(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected string
	}{
		{"ARIN", arinResponse, "arin"},
		{"RIPE", ripeResponse, "ripe"},
		{"APNIC", apnicResponse, "apnic"},
		{"LACNIC", lacnicResponse, "lacnic"},
		{"AFRINIC", afrinicResponse, "afrinic"},
		{"Unknown", "some random text", "generic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectRIR(tt.response)
			if result != tt.expected {
				t.Errorf("detectRIR() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseARINResponse(t *testing.T) {
	info, err := ParseIPWhois(arinResponse, "192.0.2.0")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}

	// 验证RIR
	if info.RIR != "arin" {
		t.Errorf("RIR = %v, want arin", info.RIR)
	}

	// 验证网络信息
	if info.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if info.Network.Range != "192.0.2.0 - 192.0.2.255" {
		t.Errorf("Network.Range = %v, want '192.0.2.0 - 192.0.2.255'", info.Network.Range)
	}
	if info.Network.CIDR != "192.0.2.0/24" {
		t.Errorf("Network.CIDR = %v, want '192.0.2.0/24'", info.Network.CIDR)
	}
	if info.Network.Name != "EXAMPLE-NET" {
		t.Errorf("Network.Name = %v, want 'EXAMPLE-NET'", info.Network.Name)
	}
	if info.Network.StartIP != "192.0.2.0" {
		t.Errorf("Network.StartIP = %v, want '192.0.2.0'", info.Network.StartIP)
	}
	if info.Network.EndIP != "192.0.2.255" {
		t.Errorf("Network.EndIP = %v, want '192.0.2.255'", info.Network.EndIP)
	}
	if info.Network.PrefixLen != 24 {
		t.Errorf("Network.PrefixLen = %v, want 24", info.Network.PrefixLen)
	}
	if info.Network.Country != "US" {
		t.Errorf("Network.Country = %v, want 'US'", info.Network.Country)
	}

	// 验证组织信息
	if info.Organization == nil {
		t.Fatal("Organization should not be nil")
	}
	if info.Organization.Name != "Example Organization" {
		t.Errorf("Organization.Name = %v, want 'Example Organization'", info.Organization.Name)
	}
	if info.Organization.ID != "EO-123" {
		t.Errorf("Organization.ID = %v, want 'EO-123'", info.Organization.ID)
	}
	if info.Organization.Address != "123 Example Street" {
		t.Errorf("Organization.Address = %v, want '123 Example Street'", info.Organization.Address)
	}

	// 验证ASN信息
	if info.ASN == nil {
		t.Fatal("ASN should not be nil")
	}
	if info.ASN.Number != 12345 {
		t.Errorf("ASN.Number = %v, want 12345", info.ASN.Number)
	}

	// 验证联系人
	if len(info.Contacts) < 2 {
		t.Fatalf("Expected at least 2 contacts, got %d", len(info.Contacts))
	}
	abuseContact := findContactByRole(info.Contacts, "abuse")
	if abuseContact == nil {
		t.Fatal("Abuse contact not found")
	}
	if abuseContact.Email != "abuse@example.com" {
		t.Errorf("Abuse email = %v, want 'abuse@example.com'", abuseContact.Email)
	}
	if abuseContact.Handle != "ABU123-ARIN" {
		t.Errorf("Abuse handle = %v, want 'ABU123-ARIN'", abuseContact.Handle)
	}
}

func TestParseRIPEResponse(t *testing.T) {
	info, err := ParseIPWhois(ripeResponse, "193.0.0.0")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}

	if info.RIR != "ripe" {
		t.Errorf("RIR = %v, want ripe", info.RIR)
	}

	if info.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if info.Network.Range != "193.0.0.0 - 193.0.0.255" {
		t.Errorf("Network.Range = %v, want '193.0.0.0 - 193.0.0.255'", info.Network.Range)
	}
	if info.Network.Name != "RIPE-NCC-NET" {
		t.Errorf("Network.Name = %v, want 'RIPE-NCC-NET'", info.Network.Name)
	}
	if info.Network.Country != "NL" {
		t.Errorf("Network.Country = %v, want 'NL'", info.Network.Country)
	}
	if info.Network.Status != "ASSIGNED PA" {
		t.Errorf("Network.Status = %v, want 'ASSIGNED PA'", info.Network.Status)
	}

	// 验证组织
	if info.Organization == nil {
		t.Fatal("Organization should not be nil")
	}
	if info.Organization.ID != "ORG-RIEN1-RIPE" {
		t.Errorf("Organization.ID = %v, want 'ORG-RIEN1-RIPE'", info.Organization.ID)
	}
	if info.Organization.Name != "RIPE Network Coordination Centre" {
		t.Errorf("Organization.Name = %v, want 'RIPE Network Coordination Centre'", info.Organization.Name)
	}

	// 验证abuse联系人邮箱
	abuseContact := findContactByRole(info.Contacts, "abuse")
	if abuseContact == nil {
		t.Fatal("Abuse contact not found")
	}
	if abuseContact.Email != "abuse@ripe.net" {
		t.Errorf("Abuse email = %v, want 'abuse@ripe.net'", abuseContact.Email)
	}

	// 验证日期 - RIPE响应中组织块的created会覆盖网络块的created
	// 组织块中的created: 2010-01-01T00:00:00Z 会成为最终值
	if info.Organization.CreatedDate != "2010-01-01" {
		t.Errorf("CreatedDate = %v, want '2010-01-01'", info.Organization.CreatedDate)
	}
}

func TestParseAPNICResponse(t *testing.T) {
	info, err := ParseIPWhois(apnicResponse, "203.0.113.0")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}

	if info.RIR != "apnic" {
		t.Errorf("RIR = %v, want apnic", info.RIR)
	}

	if info.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if info.Network.Name != "APNIC-LABS-NET" {
		t.Errorf("Network.Name = %v, want 'APNIC-LABS-NET'", info.Network.Name)
	}
	if info.Network.Country != "AU" {
		t.Errorf("Network.Country = %v, want 'AU'", info.Network.Country)
	}
}

func TestParseLACNICResponse(t *testing.T) {
	info, err := ParseIPWhois(lacnicResponse, "200.0.0.0")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}

	if info.RIR != "lacnic" {
		t.Errorf("RIR = %v, want lacnic", info.RIR)
	}

	if info.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if info.Network.Country != "BR" {
		t.Errorf("Network.Country = %v, want 'BR'", info.Network.Country)
	}

	if info.Organization == nil {
		t.Fatal("Organization should not be nil")
	}
	if info.Organization.Name != "LACNIC Labs" {
		t.Errorf("Organization.Name = %v, want 'LACNIC Labs'", info.Organization.Name)
	}
	if info.Organization.ID != "BR-LALA-LACNI" {
		t.Errorf("Organization.ID = %v, want 'BR-LALA-LACNI'", info.Organization.ID)
	}
}

func TestParseAFRINICResponse(t *testing.T) {
	info, err := ParseIPWhois(afrinicResponse, "196.0.0.0")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}

	if info.RIR != "afrinic" {
		t.Errorf("RIR = %v, want afrinic", info.RIR)
	}

	if info.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if info.Network.Name != "AFRINIC-TEST-NET" {
		t.Errorf("Network.Name = %v, want 'AFRINIC-TEST-NET'", info.Network.Name)
	}
	if info.Network.Country != "ZA" {
		t.Errorf("Network.Country = %v, want 'ZA'", info.Network.Country)
	}

	abuseContact := findContactByRole(info.Contacts, "abuse")
	if abuseContact == nil {
		t.Fatal("Abuse contact not found")
	}
	if abuseContact.Email != "abuse@afrinic.net" {
		t.Errorf("Abuse email = %v, want 'abuse@afrinic.net'", abuseContact.Email)
	}
}

func TestParseIPWhois_EmptyResponse(t *testing.T) {
	_, err := ParseIPWhois("", "192.0.2.0")
	if err == nil {
		t.Error("Expected error for empty response")
	}
}

func TestParseIPWhois_UnknownFormat(t *testing.T) {
	info, err := ParseIPWhois("some random text without RIR markers", "1.2.3.4")
	if err != nil {
		t.Fatalf("ParseIPWhois() error = %v", err)
	}
	if info.RIR != "generic" {
		t.Errorf("RIR = %v, want generic", info.RIR)
	}
}

func TestParseIPRange(t *testing.T) {
	tests := []struct {
		input   string
		wantS   string
		wantE   string
	}{
		{"192.0.2.0 - 192.0.2.255", "192.0.2.0", "192.0.2.255"},
		{"10.0.0.0/8", "10.0.0.0", "10.255.255.255"},
		{"172.16.0.0 - 172.31.255.255", "172.16.0.0", "172.31.255.255"},
		{"invalid", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			start, end := parseIPRange(tt.input)
			if start != tt.wantS {
				t.Errorf("start = %v, want %v", start, tt.wantS)
			}
			if end != tt.wantE {
				t.Errorf("end = %v, want %v", end, tt.wantE)
			}
		})
	}
}

func TestExtractPrefixLen(t *testing.T) {
	tests := []struct {
		cidr string
		want int
	}{
		{"192.0.2.0/24", 24},
		{"10.0.0.0/8", 8},
		{"", 0},
		{"192.0.2.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			result := extractPrefixLen(tt.cidr)
			if result != tt.want {
				t.Errorf("extractPrefixLen(%q) = %v, want %v", tt.cidr, result, tt.want)
			}
		})
	}
}

func TestExtractASNNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"AS12345", 12345},
		{"AS13335 CLOUDFLARE", 13335},
		{"12345", 12345},
		{"", 0},
		{"AS0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractASNNumber(tt.input)
			if result != tt.want {
				t.Errorf("extractASNNumber(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestCalculateCIDR(t *testing.T) {
	tests := []struct {
		start string
		end   string
		want  string
	}{
		{"192.0.2.0", "192.0.2.255", "192.0.2.0/24"},
		{"10.0.0.0", "10.255.255.255", "10.0.0.0/8"},
		{"192.168.1.0", "192.168.1.63", "192.168.1.0/26"},
	}

	for _, tt := range tests {
		t.Run(tt.start, func(t *testing.T) {
			result := calculateCIDR(tt.start, tt.end)
			if result != tt.want {
				t.Errorf("calculateCIDR(%q, %q) = %v, want %v", tt.start, tt.end, result, tt.want)
			}
		})
	}
}

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2020-01-01T00:00:00Z", "2020-01-01"},
		{"2020-01-01", "2020-01-01"},
		{"20200101", "2020-01-01"},
		{"", ""},
		{"invalid-date", "invalid-date"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeDate(tt.input)
			if result != tt.want {
				t.Errorf("normalizeDate(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestExtractBeforeParen(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example Org (EO-123)", "Example Org"},
		{"No parens", "No parens"},
		{"(starts with paren)", "(starts with paren)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractBeforeParen(tt.input)
			if result != tt.want {
				t.Errorf("extractBeforeParen(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestExtractInParen(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example Org (EO-123)", "EO-123"},
		{"No parens", ""},
		{"(only paren)", "only paren"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractInParen(tt.input)
			if result != tt.want {
				t.Errorf("extractInParen(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

func TestSplitWhoisLines(t *testing.T) {
	input := `# comment
% another comment
inetnum: 192.0.2.0 - 192.0.2.255

netname: TEST

country: US`
	lines := splitWhoisLines(input)

	expected := []string{
		"inetnum: 192.0.2.0 - 192.0.2.255",
		"netname: TEST",
		"country: US",
	}

	if len(lines) != len(expected) {
		t.Fatalf("Expected %d lines, got %d", len(expected), len(lines))
	}

	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Line %d: got %q, want %q", i, line, expected[i])
		}
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		line    string
		wantKey string
		wantVal string
	}{
		{"NetRange:       192.0.2.0 - 192.0.2.255", "NetRange", "192.0.2.0 - 192.0.2.255"},
		{"country:NL", "country", "NL"},
		{"  key  :  value  ", "key", "value"},
		{"no colon here", "", ""},
		{": value only", "", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			key, val := parseKeyValue(tt.line)
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
			if val != tt.wantVal {
				t.Errorf("value = %q, want %q", val, tt.wantVal)
			}
		})
	}
}

// 辅助函数
func findContactByRole(contacts []*IPContact, role string) *IPContact {
	for _, c := range contacts {
		if c.Role == role {
			return c
		}
	}
	return nil
}

// Benchmark测试
func BenchmarkParseARINResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseIPWhois(arinResponse, "192.0.2.0")
	}
}

func BenchmarkParseRIPEResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseIPWhois(ripeResponse, "193.0.0.0")
	}
}

func BenchmarkParseAPNICResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseIPWhois(apnicResponse, "203.0.113.0")
	}
}
