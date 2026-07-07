package whois

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- detectRIR 表驱动覆盖所有分支 ----

func TestDetectRIR_AllBranches(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     string
	}{
		// 主检测分支（需同时含注册库名 + 全称/简称描述）
		{"arin full", "American Registry ARIN whois", "arin"},
		{"arin alt", "arin ARIN WHOIS", "arin"},
		{"ripe full", "RIPE Network ripe", "ripe"},
		{"ripe alt", "ripe RIPE NCC", "ripe"},
		{"apnic full", "APNIC Asia Pacific", "apnic"},
		{"apnic alt", "apnic APNIC whois", "apnic"},
		{"apnic alt2", "apnic information on apnic", "apnic"},
		{"lacnic full", "LACNIC Latin American", "lacnic"},
		{"lacnic alt", "lacnic LACNIC whois", "lacnic"},
		{"afrinic full", "AFRINIC African", "afrinic"},
		{"afrinic alt", "afrinic AFRINIC whois", "afrinic"},
		{"afrinic alt2", "afrinic AFRINIC database", "afrinic"},
		// 仅含注册库名但无全称 → 不匹配主分支，回落到 whois.* 分支
		{"arin host", "whois.arin.net", "arin"},
		{"ripe host", "whois.ripe.net", "ripe"},
		{"apnic host", "whois.apnic.net", "apnic"},
		{"lacnic host", "whois.lacnic.net", "lacnic"},
		{"afrinic host", "whois.afrinic.net", "afrinic"},
		// source 字段检测
		{"source ripe spaced", "source:         ripe", "ripe"},
		{"source ripe nocolon", "source:ripe", "ripe"},
		{"source apnic spaced", "source:         apnic", "apnic"},
		{"source apnic nocolon", "source:apnic", "apnic"},
		{"source lacnic spaced", "source:      lacnic", "lacnic"},
		{"source lacnic nocolon", "source:lacnic", "lacnic"},
		{"source afrinic spaced", "source:         afrinic", "afrinic"},
		{"source afrinic nocolon", "source:afrinic", "afrinic"},
		// 仅含注册库短名但无全称、无 host、无 source → generic
		{"only arin word", "arin", "generic"},
		{"only ripe word", "ripe", "generic"},
		{"empty", "", "generic"},
		{"random", "some random text", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectRIR(tt.response)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---- containsAny ----

func TestContainsAny(t *testing.T) {
	assert.True(t, containsAny("hello world", "world"))
	assert.True(t, containsAny("a b c", "x", "b", "z"))
	assert.False(t, containsAny("a b c", "x", "y", "z"))
	assert.False(t, containsAny("", "x"))
	// 空候选
	assert.False(t, containsAny("abc"))
}

// ---- calculateCIDRv6 ----

func TestCalculateCIDRv6(t *testing.T) {
	// 相同 IP → 全前缀 /128
	got := calculateCIDRv6(
		parseIP("2001:db8::1"),
		parseIP("2001:db8::1"),
	)
	assert.Equal(t, "2001:db8::1/128", got)
}

func TestCalculateCIDRv6_PartialPrefix(t *testing.T) {
	// 同段前缀差异
	got := calculateCIDRv6(
		parseIP("2001:db8::0"),
		parseIP("2001:db8::ff"),
	)
	assert.Contains(t, got, "2001:db8::")
	assert.Contains(t, got, "/")
}

func TestCalculateCIDR_DifferentVersions(t *testing.T) {
	// start IPv4 end IPv6 → 落入 IPv6 分支
	got := calculateCIDR("192.0.2.0", "2001:db8::1")
	assert.NotEmpty(t, got)
}

func TestCalculateCIDR_Invalid(t *testing.T) {
	assert.Equal(t, "", calculateCIDR("not-an-ip", "also-not"))
}

// ---- ipToInt nil (IPv6 input) ----

func TestIPToInt_IPv6(t *testing.T) {
	// IPv6 地址 To4() 返回 nil → 返回 0
	assert.Equal(t, uint32(0), ipToInt(parseIP("2001:db8::1")))
}

func TestIPToInt_IPv4(t *testing.T) {
	assert.Equal(t, uint32(0xC0A80001), ipToInt(parseIP("192.168.0.1")))
}

// ---- calculateCIDRv4 非精确块 ----

func TestCalculateCIDRv4_NonExactBlock(t *testing.T) {
	// 192.168.0.1 - 192.168.0.5 不是合法 CIDR 边界，应返回范围表示
	got := calculateCIDRv4(parseIP("192.168.0.1").To4(), parseIP("192.168.0.5").To4())
	assert.Contains(t, got, " - ")
}

// ---- extractASNName ----

func TestExtractASNName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"AS12345 EXAMPLE-AS", "EXAMPLE-AS"},
		{"AS12345 EXAMPLE AS INC", "EXAMPLE AS INC"},
		{"AS12345", ""}, // 只有 AS 号，无名称
		// "no as number here": "as" 大写后为 "AS"，匹配前缀且非最后段 → 返回 "number here"
		{"no as number here", "number here"},
		{"12345 EXAMPLE", ""}, // 不以 AS 开头
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, extractASNName(tt.input))
		})
	}
}

// ---- parseGenericIPResponse 全字段 ----

func TestParseGenericIPResponse_AllFields(t *testing.T) {
	resp := `NetRange: 192.0.2.0 - 192.0.2.255
CIDR: 192.0.2.0/24
NetName: TESTNET
Country: US
OrgName: Example Org
OrgId: EX-123
Address: 123 Main St
AbuseEmail: abuse@example.com
OriginAS: AS12345`
	info := &IPWhoisInfo{}
	parseGenericIPResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "192.0.2.0/24", info.Network.CIDR)
	assert.Equal(t, "TESTNET", info.Network.Name)
	assert.Equal(t, "US", info.Network.Country)
	assert.NotNil(t, info.Organization)
	assert.Equal(t, "Example Org", info.Organization.Name)
	assert.Equal(t, "EX-123", info.Organization.ID)
	assert.Equal(t, "123 Main St", info.Organization.Address)
	assert.NotNil(t, info.ASN)
	assert.Equal(t, 12345, info.ASN.Number)
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "abuse", info.Contacts[0].Role)
	assert.Equal(t, "abuse@example.com", info.Contacts[0].Email)
}

func TestParseGenericIPResponse_RangeToCIDR(t *testing.T) {
	// 提供 inetnum 范围但不提供 CIDR，应自动计算
	resp := `inetnum: 192.0.2.0 - 192.0.2.255
netname: rng`
	info := &IPWhoisInfo{}
	parseGenericIPResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "192.0.2.0/24", info.Network.CIDR)
}

func TestParseGenericIPResponse_OrgOnly(t *testing.T) {
	// ownerid 含 "owner" 子串，会先匹配 OrgName 分支
	resp := `orgid: EX-1
owner: Example Owner`
	info := &IPWhoisInfo{}
	parseGenericIPResponse(resp, info)
	assert.NotNil(t, info.Organization)
	// orgid 匹配 OrgName? "orgid" 不含 orgname/organisation/organization/owner，故落到 OrgId 分支
	assert.Equal(t, "EX-1", info.Organization.ID)
	// owner 匹配 OrgName 分支
	assert.Equal(t, "Example Owner", info.Organization.Name)
	assert.Nil(t, info.Network)
}

func TestParseGenericIPResponse_AbuseMailbox(t *testing.T) {
	resp := `abuse-mailbox: abuse@example.com`
	info := &IPWhoisInfo{}
	parseGenericIPResponse(resp, info)
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "abuse@example.com", info.Contacts[0].Email)
}

func TestParseGenericIPResponse_Empty(t *testing.T) {
	info := &IPWhoisInfo{}
	parseGenericIPResponse("", info)
	assert.Nil(t, info.Network)
	assert.Nil(t, info.Organization)
}

func TestParseGenericIPResponse_CommentLines(t *testing.T) {
	resp := `# comment
% another comment

NetName: TEST`
	info := &IPWhoisInfo{}
	parseGenericIPResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "TEST", info.Network.Name)
}

// ---- parseKeyValue edge ----

func TestParseKeyValue_NoColon(t *testing.T) {
	k, v := parseKeyValue("no colon here")
	assert.Equal(t, "", k)
	assert.Equal(t, "", v)
}

func TestParseKeyValue_ColonAtStart(t *testing.T) {
	k, v := parseKeyValue(": value")
	assert.Equal(t, "", k)
	assert.Equal(t, "", v)
}

// ---- parseIPRange edge ----

func TestParseIPRange_Empty(t *testing.T) {
	s, e := parseIPRange("not a range")
	assert.Equal(t, "", s)
	assert.Equal(t, "", e)
}

func TestParseIPRange_IPv6(t *testing.T) {
	s, e := parseIPRange("2001:db8::1 - 2001:db8::2")
	assert.Equal(t, "2001:db8::1", s)
	assert.Equal(t, "2001:db8::2", e)
}

// helper
func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
