package whois

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// IPWhoisInfo IP WHOIS结构化解析结果
// 支持5大RIR格式：ARIN, RIPE, APNIC, LACNIC, AFRINIC
type IPWhoisInfo struct {
	// IP地址或网段
	Query string `json:"query"`

	// 网络范围
	Network *IPNetwork `json:"network,omitempty"`

	// 组织信息
	Organization *IPOrganization `json:"organization,omitempty"`

	// 联系人信息
	Contacts []*IPContact `json:"contacts,omitempty"`

	// ASN信息
	ASN *ASNInfo `json:"asn,omitempty"`

	// 源RIR
	RIR string `json:"rir"`

	// 原始响应
	RawResponse string `json:"-"`
}

// IPNetwork IP网络信息
type IPNetwork struct {
	// 网络范围 (e.g., "192.0.2.0 - 192.0.2.255")
	Range string `json:"range,omitempty"`

	// CIDR表示 (e.g., "192.0.2.0/24")
	CIDR string `json:"cidr,omitempty"`

	// 起始IP
	StartIP string `json:"start_ip,omitempty"`

	// 结束IP
	EndIP string `json:"end_ip,omitempty"`

	// 网络前缀长度
	PrefixLen int `json:"prefix_len,omitempty"`

	// 网络名称
	Name string `json:"name,omitempty"`

	// 网络类型 (ALLOCATED PI, ASSIGNED PA, etc.)
	Type string `json:"type,omitempty"`

	// 国家代码
	Country string `json:"country,omitempty"`

	// 状态
	Status string `json:"status,omitempty"`
}

// IPOrganization IP组织信息
type IPOrganization struct {
	// 组织ID (e.g., "ORG-1234-RIPE")
	ID string `json:"id,omitempty"`

	// 组织名称
	Name string `json:"name,omitempty"`

	// 组织地址
	Address string `json:"address,omitempty"`

	// 国家
	Country string `json:"country,omitempty"`

	// 注册日期
	CreatedDate string `json:"created_date,omitempty"`

	// 更新日期
	UpdatedDate string `json:"updated_date,omitempty"`
}

// IPContact IP联系人信息
type IPContact struct {
	// 角色 (abuse, admin, tech, noc)
	Role string `json:"role"`

	// 名称
	Name string `json:"name,omitempty"`

	// 邮箱
	Email string `json:"email,omitempty"`

	// 电话
	Phone string `json:"phone,omitempty"`

	// 组织
	Organization string `json:"organization,omitempty"`

	// Handle/ID
	Handle string `json:"handle,omitempty"`
}

// ASNInfo ASN信息
type ASNInfo struct {
	// AS号 (e.g., 13335)
	Number int `json:"number"`

	// AS名称 (e.g., "CLOUDFLARE")
	Name string `json:"name,omitempty"`

	// AS持有者
	Holder string `json:"holder,omitempty"`

	// 国家
	Country string `json:"country,omitempty"`

	// 注册日期
	CreatedDate string `json:"created_date,omitempty"`

	// 更新日期
	UpdatedDate string `json:"updated_date,omitempty"`
}

// ParseIPWhois 解析IP WHOIS响应，自动检测RIR格式
func ParseIPWhois(rawResponse string, query string) (*IPWhoisInfo, error) {
	if rawResponse == "" {
		return nil, fmt.Errorf("WHOIS响应为空")
	}

	info := &IPWhoisInfo{
		Query:       query,
		RawResponse: rawResponse,
	}

	// 检测RIR
	rir := detectRIR(rawResponse)
	info.RIR = rir

	// 根据RIR选择对应的解析器
	switch rir {
	case "arin":
		parseARINResponse(rawResponse, info)
	case "ripe":
		parseRIPEResponse(rawResponse, info)
	case "apnic":
		parseAPNICResponse(rawResponse, info)
	case "lacnic":
		parseLACNICResponse(rawResponse, info)
	case "afrinic":
		parseAFRINICResponse(rawResponse, info)
	default:
		// 通用解析器
		parseGenericIPResponse(rawResponse, info)
	}

	return info, nil
}

// detectRIR 检测WHOIS响应来自哪个RIR
func detectRIR(response string) string {
	lower := strings.ToLower(response)
	switch {
	case strings.Contains(lower, "arin") && (strings.Contains(lower, "american registry") || strings.Contains(lower, "arin whois")):
		return "arin"
	case strings.Contains(lower, "ripe") && (strings.Contains(lower, "ripe network") || strings.Contains(lower, "ripe ncc")):
		return "ripe"
	case strings.Contains(lower, "apnic") && (strings.Contains(lower, "asia pacific") || strings.Contains(lower, "apnic whois") || strings.Contains(lower, "information on apnic")):
		return "apnic"
	case strings.Contains(lower, "lacnic") && (strings.Contains(lower, "latin american") || strings.Contains(lower, "lacnic whois")):
		return "lacnic"
	case strings.Contains(lower, "afrinic") && (strings.Contains(lower, "african") || strings.Contains(lower, "afrinic whois") || strings.Contains(lower, "afrinic database")):
		return "afrinic"
	// 补充检测：通过注册库名称
	case strings.Contains(lower, "whois.arin.net"):
		return "arin"
	case strings.Contains(lower, "whois.ripe.net"):
		return "ripe"
	case strings.Contains(lower, "whois.apnic.net"):
		return "apnic"
	case strings.Contains(lower, "whois.lacnic.net"):
		return "lacnic"
	case strings.Contains(lower, "whois.afrinic.net"):
		return "afrinic"
	// 通过source字段检测
	case strings.Contains(lower, "source:         ripe") || strings.Contains(lower, "source:ripe"):
		return "ripe"
	case strings.Contains(lower, "source:         apnic") || strings.Contains(lower, "source:apnic"):
		return "apnic"
	case strings.Contains(lower, "source:      lacnic") || strings.Contains(lower, "source:lacnic"):
		return "lacnic"
	case strings.Contains(lower, "source:         afrinic") || strings.Contains(lower, "source:afrinic"):
		return "afrinic"
	default:
		return "generic"
	}
}

// parseARINResponse 解析ARIN格式响应
// ARIN格式示例:
// NetRange:       192.0.2.0 - 192.0.2.255
// CIDR:           192.0.2.0/24
// NetName:        EXAMPLE
// NetType:        Allocated PA
// OriginAS:       AS12345
// Organization:   EXAMPLE ORG (EO-123)
// Country:        US
// RegDate:        2020-01-01
// Updated:        2024-01-01
// Ref:            https://whois.arin.net/rest/net/NET-123
//
// OrgName:        Example Organization
// OrgId:          EO-123
// Address:        123 Example St
// Country:        US
//
// Abuse Handle:   ABU123-ARIN
// Abuse Name:     Abuse Desk
// Abuse Email:    abuse@example.com
// Abuse Phone:    +1-123-456-7890
func parseARINResponse(response string, info *IPWhoisInfo) {
	lines := splitWhoisLines(response)

	network := &IPNetwork{}
	org := &IPOrganization{}
	hasNetwork, hasOrg := false, false

	var currentContactRole string
	var currentContact *IPContact

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		key, value := parseKeyValue(line)
		if key == "" {
			continue
		}

		switch strings.ToLower(key) {
		// 网络信息
		case "netrange", "net range":
			network.Range = value
			startIP, endIP := parseIPRange(value)
			network.StartIP = startIP
			network.EndIP = endIP
			hasNetwork = true
		case "cidr":
			network.CIDR = value
			if network.PrefixLen == 0 {
				network.PrefixLen = extractPrefixLen(value)
			}
			hasNetwork = true
		case "netname", "net name":
			network.Name = value
			hasNetwork = true
		case "nettype", "net type":
			network.Type = value
			hasNetwork = true
		case "country":
			if !hasOrg || org.Country != "" {
				network.Country = value
			} else {
				org.Country = value
			}
		case "originas", "origin as":
			if info.ASN == nil {
				info.ASN = &ASNInfo{}
			}
			info.ASN.Number = extractASNNumber(value)
			info.ASN.Name = extractASNName(value)

		// 组织信息
		case "orgname", "organization":
			if strings.Contains(value, "(") {
				// ARIN格式: "EXAMPLE ORG (EO-123)"
				org.Name = extractBeforeParen(value)
				org.ID = extractInParen(value)
			} else {
				org.Name = value
			}
			hasOrg = true
		case "orgid", "org id":
			org.ID = value
			hasOrg = true
		case "address":
			org.Address = value
			hasOrg = true

		// 联系人 - ARIN使用 "Role Handle:", "Role Name:", "Role Email:", "Role Phone:" 模式
		case "abuse handle", "tech handle", "admin handle", "noc handle":
			role := strings.ToLower(strings.SplitN(key, " ", 2)[0])
			if currentContact != nil && currentContactRole != role {
				info.Contacts = append(info.Contacts, currentContact)
			}
			currentContactRole = role
			currentContact = &IPContact{
				Role:   role,
				Handle: value,
			}
		case "abuse name", "tech name", "admin name", "noc name":
			if currentContact != nil {
				currentContact.Name = value
			}
		case "abuse email", "tech email", "admin email", "noc email":
			if currentContact != nil {
				currentContact.Email = value
			}
		case "abuse phone", "tech phone", "admin phone", "noc phone":
			if currentContact != nil {
				currentContact.Phone = value
			}

		// 日期
		case "regdate":
			org.CreatedDate = value
		case "updated":
			org.UpdatedDate = value
		}
	}

	// 添加最后一个联系人
	if currentContact != nil {
		info.Contacts = append(info.Contacts, currentContact)
	}

	if hasNetwork {
		info.Network = network
	}
	if hasOrg {
		info.Organization = org
	}
}

// parseRIPEResponse 解析RIPE格式响应
// RIPE使用键值对格式，但前缀是对象类型:
// inetnum:        192.0.2.0 - 192.0.2.255
// netname:        EXAMPLE-NET
// descr:          Example Network
// country:        NL
// org:            ORG-EX1-RIPE
// admin-c:        AD123-RIPE
// tech-c:         TC123-RIPE
// abuse-c:        AB123-RIPE
// status:         ASSIGNED PA
// mnt-by:         EXAMPLE-MNT
// created:        2020-01-01T00:00:00Z
// last-modified:  2024-01-01T00:00:00Z
// source:         RIPE
func parseRIPEResponse(response string, info *IPWhoisInfo) {
	lines := splitWhoisLines(response)

	network := &IPNetwork{}
	org := &IPOrganization{}
	hasNetwork, hasOrg := false, false
	var descriptions []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		key, value := parseKeyValue(line)
		if key == "" {
			// 可能是续行
			if len(descriptions) > 0 && strings.HasPrefix(line, " ") {
				descriptions[len(descriptions)-1] += " " + strings.TrimSpace(line)
			}
			continue
		}

		switch strings.ToLower(key) {
		// IPv4/IPv6网段
		case "inetnum", "inet6num":
			network.Range = value
			startIP, endIP := parseIPRange(value)
			network.StartIP = startIP
			network.EndIP = endIP
			hasNetwork = true
		case "cidr":
			network.CIDR = value
			network.PrefixLen = extractPrefixLen(value)
			hasNetwork = true
		case "netname":
			network.Name = value
			hasNetwork = true
		case "descr":
			descriptions = append(descriptions, value)
		case "country":
			network.Country = strings.ToUpper(strings.TrimSpace(value))
			hasNetwork = true
		case "status":
			network.Status = value
			network.Type = value // RIPE用status表示类型
			hasNetwork = true
		case "org":
			org.ID = value
			hasOrg = true
		case "organisation":
			org.ID = value
			hasOrg = true
		case "org-name":
			org.Name = value
			hasOrg = true
		case "address":
			if org.Address == "" {
				org.Address = value
			} else {
				org.Address += "; " + value
			}
			hasOrg = true

		// 联系人引用 (RIPE只引用handle，不内联详细信息)
		case "abuse-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "abuse", Handle: value})
		case "admin-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "admin", Handle: value})
		case "tech-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "tech", Handle: value})
		case "noc-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "noc", Handle: value})

		// abuse-mailto (RIPE特有)
		case "abuse-mailbox":
			// 更新abuse联系人的邮箱
			for _, c := range info.Contacts {
				if c.Role == "abuse" {
					c.Email = value
					break
				}
			}

		// 日期
		case "created":
			org.CreatedDate = normalizeDate(value)
		case "last-modified":
			org.UpdatedDate = normalizeDate(value)

		// ASN
		case "origin":
			if info.ASN == nil {
				info.ASN = &ASNInfo{}
			}
			info.ASN.Number = extractASNNumber(value)
		}
	}

	// 把description设为网络名称的补充
	if len(descriptions) > 0 && network.Name == "" {
		network.Name = descriptions[0]
	}

	// 计算CIDR（如果没有提供）
	if hasNetwork && network.CIDR == "" && network.StartIP != "" && network.EndIP != "" {
		network.CIDR = calculateCIDR(network.StartIP, network.EndIP)
		network.PrefixLen = extractPrefixLen(network.CIDR)
	}

	if hasNetwork {
		info.Network = network
	}
	if hasOrg {
		info.Organization = org
	}
}

// parseAPNICResponse 解析APNIC格式响应
// APNIC格式与RIPE非常类似，但有细微差异
func parseAPNICResponse(response string, info *IPWhoisInfo) {
	// APNIC格式与RIPE几乎相同，复用RIPE解析器
	parseRIPEResponse(response, info)
	// 修正RIR标识
	info.RIR = "apnic"
}

// parseLACNICResponse 解析LACNIC格式响应
// LACNIC也类似RIPE，但某些字段名不同
func parseLACNICResponse(response string, info *IPWhoisInfo) {
	lines := splitWhoisLines(response)

	network := &IPNetwork{}
	org := &IPOrganization{}
	hasNetwork, hasOrg := false, false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		key, value := parseKeyValue(line)
		if key == "" {
			continue
		}

		switch strings.ToLower(key) {
		case "inetnum", "inet6num":
			network.Range = value
			startIP, endIP := parseIPRange(value)
			network.StartIP = startIP
			network.EndIP = endIP
			hasNetwork = true
		case "netname":
			network.Name = value
			hasNetwork = true
		case "country":
			network.Country = strings.ToUpper(strings.TrimSpace(value))
			hasNetwork = true
		case "status":
			network.Status = value
			network.Type = value
			hasNetwork = true
		case "owner":
			org.Name = value
			hasOrg = true
		case "ownerid":
			org.ID = value
			hasOrg = true
		case "responsible":
			// LACNIC特有字段
		case "abuse-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "abuse", Handle: value})
		case "tech-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "tech", Handle: value})
		case "admin-c":
			info.Contacts = append(info.Contacts, &IPContact{Role: "admin", Handle: value})
		case "e-mail", "abuse-mailbox":
			email := value
			role := "abuse"
			if strings.ToLower(key) == "e-mail" {
				role = "tech"
			}
			found := false
			for _, c := range info.Contacts {
				if c.Role == role && c.Email == "" {
					c.Email = email
					found = true
					break
				}
			}
			if !found {
				info.Contacts = append(info.Contacts, &IPContact{Role: role, Email: email})
			}
		case "created":
			org.CreatedDate = normalizeDate(value)
		case "changed", "last-modified":
			org.UpdatedDate = normalizeDate(value)
		}
	}

	if hasNetwork && network.CIDR == "" && network.StartIP != "" && network.EndIP != "" {
		network.CIDR = calculateCIDR(network.StartIP, network.EndIP)
		network.PrefixLen = extractPrefixLen(network.CIDR)
	}

	if hasNetwork {
		info.Network = network
	}
	if hasOrg {
		info.Organization = org
	}
}

// parseAFRINICResponse 解析AFRINIC格式响应
// AFRINIC格式与RIPE相同
func parseAFRINICResponse(response string, info *IPWhoisInfo) {
	parseRIPEResponse(response, info)
	info.RIR = "afrinic"
}

// parseGenericIPResponse 通用IP WHOIS解析器
// 当无法识别RIR时，使用正则匹配常见字段
func parseGenericIPResponse(response string, info *IPWhoisInfo) {
	lines := splitWhoisLines(response)

	network := &IPNetwork{}
	org := &IPOrganization{}
	hasNetwork, hasOrg := false, false

	// 常见字段映射
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		key, value := parseKeyValue(line)
		if key == "" {
			continue
		}

		lowerKey := strings.ToLower(key)
		switch {
		case containsAny(lowerKey, "inetnum", "netrange", "net range", "inet6num"):
			network.Range = value
			startIP, endIP := parseIPRange(value)
			network.StartIP = startIP
			network.EndIP = endIP
			hasNetwork = true
		case containsAny(lowerKey, "cidr"):
			network.CIDR = value
			network.PrefixLen = extractPrefixLen(value)
			hasNetwork = true
		case containsAny(lowerKey, "netname", "net name"):
			network.Name = value
			hasNetwork = true
		case lowerKey == "country":
			network.Country = strings.ToUpper(strings.TrimSpace(value))
			hasNetwork = true
		case containsAny(lowerKey, "orgname", "organisation", "organization", "owner"):
			org.Name = value
			hasOrg = true
		case containsAny(lowerKey, "orgid", "org-id", "ownerid"):
			org.ID = value
			hasOrg = true
		case containsAny(lowerKey, "address"):
			org.Address = value
			hasOrg = true
		case containsAny(lowerKey, "abuse"):
			if strings.Contains(lowerKey, "email") || strings.Contains(lowerKey, "mailbox") {
				info.Contacts = append(info.Contacts, &IPContact{Role: "abuse", Email: value})
			}
		case containsAny(lowerKey, "origin", "originas"):
			if info.ASN == nil {
				info.ASN = &ASNInfo{}
			}
			info.ASN.Number = extractASNNumber(value)
		}
	}

	if hasNetwork && network.CIDR == "" && network.StartIP != "" && network.EndIP != "" {
		network.CIDR = calculateCIDR(network.StartIP, network.EndIP)
		network.PrefixLen = extractPrefixLen(network.CIDR)
	}

	if hasNetwork {
		info.Network = network
	}
	if hasOrg {
		info.Organization = org
	}
}

// ========================
// 辅助函数
// ========================

// splitWhoisLines 将WHOIS响应分割为行，去除注释和空行
func splitWhoisLines(response string) []string {
	var result []string
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		// 跳过注释行
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "%") || trimmed == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

// parseKeyValue 解析WHOIS键值对
// 支持格式: "Key: Value", "Key:Value", "Key      :      Value"
func parseKeyValue(line string) (string, string) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", ""
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", ""
	}
	return key, value
}

// parseIPRange 解析IP范围字符串
// 支持格式: "192.0.2.0 - 192.0.2.255", "192.0.2.0/24", "192.0.2.0 - 192.0.2.255 (CIDR)"
var ipRangeRegex = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)\s*-\s*(\d+\.\d+\.\d+\.\d+)`)
var ip6RangeRegex = regexp.MustCompile(`([0-9a-fA-F:]+)\s*-\s*([0-9a-fA-F:]+)`)

func parseIPRange(rangeStr string) (startIP, endIP string) {
	// 先尝试IPv4
	matches := ipRangeRegex.FindStringSubmatch(rangeStr)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}

	// 尝试IPv6
	matches = ip6RangeRegex.FindStringSubmatch(rangeStr)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}

	// 尝试CIDR格式
	if strings.Contains(rangeStr, "/") {
		_, ipnet, err := net.ParseCIDR(rangeStr)
		if err == nil {
			return ipnet.IP.String(), lastIP(ipnet).String()
		}
	}

	return "", ""
}

// lastIP 计算网段的最后一个IP
func lastIP(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP
	mask := ipnet.Mask

	last := make(net.IP, len(ip))
	copy(last, ip)

	for i := 0; i < len(ip); i++ {
		last[i] = ip[i] | ^mask[i]
	}

	return last
}

// extractPrefixLen 从CIDR中提取前缀长度
func extractPrefixLen(cidr string) int {
	if cidr == "" {
		return 0
	}
	parts := strings.SplitN(cidr, "/", 2)
	if len(parts) < 2 {
		return 0
	}
	var prefixLen int
	fmt.Sscanf(parts[1], "%d", &prefixLen)
	return prefixLen
}

// extractASNNumber 从ASN字符串中提取数字
// 支持格式: "AS12345", "12345", "AS12345 AS67890" (取第一个)
var asnRegex = regexp.MustCompile(`AS(\d+)`)

func extractASNNumber(s string) int {
	if s == "" {
		return 0
	}
	matches := asnRegex.FindStringSubmatch(s)
	if len(matches) >= 2 {
		var n int
		fmt.Sscanf(matches[1], "%d", &n)
		return n
	}
	// 纯数字
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// extractASNName 从ASN字符串中提取AS名称
// 格式: "AS12345 EXAMPLE-AS" -> "EXAMPLE-AS"
func extractASNName(s string) string {
	if s == "" {
		return ""
	}
	// 去掉AS号部分，剩余为名称
	parts := strings.Fields(s)
	for i, p := range parts {
		if strings.HasPrefix(strings.ToUpper(p), "AS") && i < len(parts)-1 {
			return strings.Join(parts[i+1:], " ")
		}
	}
	return ""
}

// extractBeforeParen 提取括号前的文本
func extractBeforeParen(s string) string {
	idx := strings.Index(s, "(")
	if idx > 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}

// extractInParen 提取括号内的文本
func extractInParen(s string) string {
	start := strings.Index(s, "(")
	end := strings.Index(s, ")")
	if start >= 0 && end > start {
		return strings.TrimSpace(s[start+1 : end])
	}
	return ""
}

// containsAny 检查字符串是否包含任意一个候选
func containsAny(s string, candidates ...string) bool {
	for _, c := range candidates {
		if strings.Contains(s, c) {
			return true
		}
	}
	return false
}

// normalizeDate 规范化日期格式
// 支持多种格式：2020-01-01T00:00:00Z, 2020-01-01, 20200101, etc.
func normalizeDate(dateStr string) string {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return ""
	}

	// 尝试常见格式
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"20060102",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Format("2006-01-02")
		}
	}

	// 无法解析，原样返回
	return dateStr
}

// calculateCIDR 根据起始和结束IP计算CIDR
func calculateCIDR(startIP, endIP string) string {
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)
	if start == nil || end == nil {
		return ""
	}

	// IPv4
	if start.To4() != nil && end.To4() != nil {
		return calculateCIDRv4(start.To4(), end.To4())
	}

	// IPv6
	return calculateCIDRv6(start, end)
}

func calculateCIDRv4(start, end net.IP) string {
	startInt := ipToInt(start)
	endInt := ipToInt(end)

	// 计算掩码长度
	maskInt := startInt ^ endInt
	prefixLen := 32
	for maskInt > 0 {
		prefixLen--
		maskInt >>= 1
	}

	// 验证这是一个合法的CIDR块
	mask := uint32(0xFFFFFFFF) << uint(32-prefixLen)
	if startInt&mask != startInt {
		// 不是精确的CIDR块，返回范围表示
		return fmt.Sprintf("%s - %s", start.String(), end.String())
	}

	return fmt.Sprintf("%s/%d", start.String(), prefixLen)
}

func calculateCIDRv6(start, end net.IP) string {
	// 简化处理：计算公共前缀长度
	prefixLen := 0
	for i := 0; i < len(start); i++ {
		diff := start[i] ^ end[i]
		if diff == 0 {
			prefixLen += 8
			continue
		}
		for bit := 7; bit >= 0; bit-- {
			if diff&(1<<uint(bit)) != 0 {
				return fmt.Sprintf("%s/%d", start.String(), prefixLen)
			}
			prefixLen++
		}
	}
	return fmt.Sprintf("%s/%d", start.String(), prefixLen)
}

func ipToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}
