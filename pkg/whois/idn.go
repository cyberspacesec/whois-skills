package whois

import (
	"fmt"
	"strings"

	"golang.org/x/net/idna"
)

// PunycodeToUnicode 将Punycode域名转换为Unicode
func PunycodeToUnicode(domain string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("域名不能为空")
	}
	return idna.ToUnicode(domain)
}

// UnicodeToPunycode 将Unicode域名转换为Punycode
func UnicodeToPunycode(domain string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("域名不能为空")
	}
	return idna.ToASCII(domain)
}

// NormalizeDomain 规范化域名（处理IDN、去除协议前缀和尾部点）
func NormalizeDomain(domain string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("域名不能为空")
	}
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if idx := strings.Index(domain, "/"); idx > 0 {
		domain = domain[:idx]
	}
	domain = strings.Trim(domain, ".")
	domain = strings.ToLower(domain)

	// 如果域名包含非ASCII字符，转换为Punycode以便WHOIS查询
	if !isASCII(domain) {
		ascii, err := idna.ToASCII(domain)
		if err != nil {
			return "", fmt.Errorf("域名IDN转换失败: %w", err)
		}
		domain = ascii
	}
	return domain, nil
}

// IsIDN 判断域名是否是国际化域名
func IsIDN(domain string) bool {
	return strings.HasPrefix(domain, "xn--") || !isASCII(domain)
}

// isASCII 检查字符串是否全部为ASCII字符
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}
