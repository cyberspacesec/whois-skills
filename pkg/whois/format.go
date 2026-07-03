package whois

import (
	"strings"
)

// WhoisFormat WHOIS响应格式类型
type WhoisFormat string

const (
	FormatUnknown  WhoisFormat = "unknown"
	FormatARIN     WhoisFormat = "arin"
	FormatRIPE     WhoisFormat = "ripe"
	FormatAPNIC    WhoisFormat = "apnic"
	FormatLACNIC   WhoisFormat = "lacnic"
	FormatAFRINIC  WhoisFormat = "afrinic"
	FormatVerisign WhoisFormat = "verisign"
	FormatPIR      WhoisFormat = "pir"
	FormatGeneric  WhoisFormat = "generic"
)

// DetectWhoisFormat 检测WHOIS响应的格式
func DetectWhoisFormat(response string) WhoisFormat {
	lower := strings.ToLower(response)
	switch {
	case strings.Contains(lower, "arin") && (strings.Contains(lower, "american registry") || strings.Contains(lower, "arin whois")):
		return FormatARIN
	case strings.Contains(lower, "ripe") && (strings.Contains(lower, "ripe network") || strings.Contains(lower, "ripe ncc")):
		return FormatRIPE
	case strings.Contains(lower, "apnic") && strings.Contains(lower, "asia pacific"):
		return FormatAPNIC
	case strings.Contains(lower, "lacnic") && strings.Contains(lower, "latin american"):
		return FormatLACNIC
	case strings.Contains(lower, "afrinic"):
		return FormatAFRINIC
	case strings.Contains(lower, "verisign") || strings.Contains(lower, "verisign, inc"):
		return FormatVerisign
	case strings.Contains(lower, "public interest registry"):
		return FormatPIR
	default:
		return FormatGeneric
	}
}

// FormatRawResponse 格式化原始WHOIS响应（去除注释行和多余空行）
func FormatRawResponse(response string) string {
	lines := strings.Split(response, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过以#或%开头的注释行
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "%") {
			continue
		}
		// 跳过完全空行（但保留段落分隔）
		if trimmed == "" {
			if len(result) > 0 && result[len(result)-1] != "" {
				result = append(result, "")
			}
			continue
		}
		result = append(result, line)
	}
	// 去除尾部空行
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}
