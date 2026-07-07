package whois

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---- parseRIPEResponse 全分支 ----

func TestParseRIPEResponse_AllBranches(t *testing.T) {
	resp := `inetnum:        192.0.2.0 - 192.0.2.255
netname:        EXAMPLE-NET
descr:          Example Network
  continuation line
country:        NL
status:         ASSIGNED PA
org:            ORG-EX1-RIPE
org-name:       Example Org
address:        Addr1
address:        Addr2
abuse-c:        AB123-RIPE
admin-c:        AD123-RIPE
tech-c:         TC123-RIPE
noc-c:          NOC1-RIPE
abuse-mailbox:  abuse@example.com
created:        2020-01-01T00:00:00Z
last-modified:  2024-01-01T00:00:00Z
origin:         AS12345
source:         RIPE`
	info := &IPWhoisInfo{}
	parseRIPEResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "EXAMPLE-NET", info.Network.Name)
	assert.Equal(t, "NL", info.Network.Country)
	assert.Equal(t, "ASSIGNED PA", info.Network.Status)
	assert.Equal(t, "192.0.2.0/24", info.Network.CIDR)
	assert.NotNil(t, info.Organization)
	assert.Equal(t, "ORG-EX1-RIPE", info.Organization.ID)
	assert.Equal(t, "Example Org", info.Organization.Name)
	assert.Equal(t, "Addr1; Addr2", info.Organization.Address)
	// 4 个 contact
	assert.Len(t, info.Contacts, 4)
	// abuse contact 邮箱应被 abuse-mailbox 更新
	var abuse *IPContact
	for _, c := range info.Contacts {
		if c.Role == "abuse" {
			abuse = c
		}
	}
	assert.NotNil(t, abuse)
	assert.Equal(t, "abuse@example.com", abuse.Email)
	assert.NotNil(t, info.ASN)
	assert.Equal(t, 12345, info.ASN.Number)
}

func TestParseRIPEResponse_DescrAsNetworkName(t *testing.T) {
	// 无 netname，descr 应作为 network.Name
	resp := `inetnum: 192.0.2.0 - 192.0.2.255
descr: From Descr`
	info := &IPWhoisInfo{}
	parseRIPEResponse(resp, info)
	assert.Equal(t, "From Descr", info.Network.Name)
}

func TestParseRIPEResponse_CidrAndInet6num(t *testing.T) {
	resp := `inet6num: 2001:db8::/32
cidr: 2001:db8::/32`
	info := &IPWhoisInfo{}
	parseRIPEResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "2001:db8::/32", info.Network.CIDR)
}

func TestParseRIPEResponse_OrganisationBranch(t *testing.T) {
	resp := `organisation: ORG-EX2-RIPE`
	info := &IPWhoisInfo{}
	parseRIPEResponse(resp, info)
	assert.Equal(t, "ORG-EX2-RIPE", info.Organization.ID)
}

// ---- parseLACNICResponse 全分支 ----

func TestParseLACNICResponse_AllBranches(t *testing.T) {
	resp := `inetnum: 192.0.2.0 - 192.0.2.255
netname: LACNIC-NET
country: BR
status: ASSIGNED
owner: LACNIC Owner
ownerid: BR-LACN-LACNIC
responsible: Someone
abuse-c: AB-LAC
tech-c: TC-LAC
admin-c: AD-LAC
e-mail: tech@example.com
abuse-mailbox: abuse@example.com
created: 2020-01-01
changed: 2024-01-01`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "LACNIC-NET", info.Network.Name)
	assert.Equal(t, "BR", info.Network.Country)
	assert.Equal(t, "192.0.2.0/24", info.Network.CIDR)
	assert.NotNil(t, info.Organization)
	assert.Equal(t, "LACNIC Owner", info.Organization.Name)
	assert.Equal(t, "BR-LACN-LACNIC", info.Organization.ID)
	// e-mail → tech role：tech-c 已存在 tech contact（Email 空），填充其邮箱，不新增
	// abuse-c/tech-c/admin-c 共 3 个 contact
	assert.Len(t, info.Contacts, 3)
	var techContact *IPContact
	for _, c := range info.Contacts {
		if c.Role == "tech" {
			techContact = c
		}
	}
	assert.NotNil(t, techContact)
	assert.Equal(t, "tech@example.com", techContact.Email)
}

func TestParseLACNICResponse_EmailToExistingContact(t *testing.T) {
	// tech-c 已存在 tech contact，e-mail 应填充其邮箱而非新增
	resp := `tech-c: TC1
e-mail: tech@example.com`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	// 1 个 tech contact（来自 tech-c），其邮箱被填充
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "tech@example.com", info.Contacts[0].Email)
}

func TestParseLACNICResponse_AbuseMailboxToExisting(t *testing.T) {
	resp := `abuse-c: AB1
abuse-mailbox: abuse@example.com`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "abuse@example.com", info.Contacts[0].Email)
}

// ---- parseARINResponse 边缘分支 ----

func TestParseARINResponse_ContactRoleSwitchAndCountry(t *testing.T) {
	resp := `NetRange: 192.0.2.0 - 192.0.2.255
NetName: ARIN-NET
Country: US
OrgName: Example Org (EO-123)
Abuse Handle: AB123-ARIN
Abuse Name: Abuse Team
Abuse Email: abuse@example.com
Abuse Phone: +1-555
Tech Handle: TC123-ARIN
Tech Name: Tech Team
RegDate: 2020-01-01
Updated: 2024-01-01
OriginAS: AS12345 Example AS`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.NotNil(t, info.Network)
	assert.Equal(t, "US", info.Network.Country)
	assert.NotNil(t, info.Organization)
	assert.Equal(t, "Example Org", info.Organization.Name)
	assert.Equal(t, "EO-123", info.Organization.ID)
	// 两个 contact（abuse + tech）
	assert.Len(t, info.Contacts, 2)
	assert.Equal(t, "abuse", info.Contacts[0].Role)
	assert.Equal(t, "tech", info.Contacts[1].Role)
	assert.Equal(t, "Tech Team", info.Contacts[1].Name)
	assert.NotNil(t, info.ASN)
	assert.Equal(t, 12345, info.ASN.Number)
	assert.Equal(t, "Example AS", info.ASN.Name)
}

func TestParseARINResponse_CountryGoesToOrg(t *testing.T) {
	// country 在 OrgName 之后出现，且 org.Country 为空 → 填充 org.Country
	resp := `OrgName: Some Org
Country: CA`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.Equal(t, "CA", info.Organization.Country)
}

func TestParseARINResponse_CountryAfterOrgCountrySet(t *testing.T) {
	// org.Country 已被设置 → country 填充 network
	resp := `Country: CA
NetName: n
Country: US`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	// 第一次 country: hasOrg=false → network.Country=CA
	// 第二次 country: hasOrg=false → network.Country=US（覆盖）
	assert.Equal(t, "US", info.Network.Country)
}

func TestParseARINResponse_NetTypeAndCidrPrefixPreserved(t *testing.T) {
	resp := `CIDR: 192.0.2.0/24
NetType: ASSIGNED PORTABLE
OrgId: EX-1`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.Equal(t, "ASSIGNED PORTABLE", info.Network.Type)
	assert.Equal(t, 24, info.Network.PrefixLen)
	assert.Equal(t, "EX-1", info.Organization.ID)
}

func TestParseARINResponse_OrgNameNoParen(t *testing.T) {
	resp := `OrgName: Plain Org Name`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.Equal(t, "Plain Org Name", info.Organization.Name)
}

func TestParseARINResponse_ContactWithoutMatchingName(t *testing.T) {
	// handle 存在但 name/email/phone 字段缺失（currentContact 为 nil 时不填充）
	resp := `Abuse Name: NoHandle`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.Empty(t, info.Contacts)
}

// ---- parseKeyValue idx<=0 分支 ----

func TestParseKeyValue_ColonFirst(t *testing.T) {
	// idx == 0 → 返回 "",""
	k, v := parseKeyValue(": value")
	assert.Equal(t, "", k)
	assert.Equal(t, "", v)
}

func TestParseKeyValue_EmptyKey(t *testing.T) {
	// 冒号前只有空格 → TrimSpace 后 key 为空 → 返回 "",""
	k, v := parseKeyValue("   : value")
	assert.Equal(t, "", k)
	assert.Equal(t, "", v)
}

// ---- ARIN/LACNIC 不可解析行 ----

func TestParseARINResponse_UnparseableLine(t *testing.T) {
	resp := `not a key value line
NetName: ARIN-NET`
	info := &IPWhoisInfo{}
	parseARINResponse(resp, info)
	assert.Equal(t, "ARIN-NET", info.Network.Name)
}

func TestParseLACNICResponse_UnparseableLine(t *testing.T) {
	resp := `garbage line without colon
owner: LACNIC Owner`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	assert.Equal(t, "LACNIC Owner", info.Organization.Name)
}

func TestParseLACNICResponse_EmailNoMatchingContact(t *testing.T) {
	// e-mail 出现但无 tech contact → found=false → 新增 tech contact
	resp := `e-mail: tech@example.com`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "tech", info.Contacts[0].Role)
	assert.Equal(t, "tech@example.com", info.Contacts[0].Email)
}

func TestParseLACNICResponse_AbuseMailboxNoMatchingContact(t *testing.T) {
	// abuse-mailbox 出现但无 abuse contact → 新增 abuse contact
	resp := `abuse-mailbox: abuse@example.com`
	info := &IPWhoisInfo{}
	parseLACNICResponse(resp, info)
	assert.Len(t, info.Contacts, 1)
	assert.Equal(t, "abuse", info.Contacts[0].Role)
	assert.Equal(t, "abuse@example.com", info.Contacts[0].Email)
}
