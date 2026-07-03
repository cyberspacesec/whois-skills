package whois

import (
	"context"
	"fmt"

	whoisparser "github.com/likexian/whois-parser"
)

// DomainAvailability 域名可用性检查结果
type DomainAvailability struct {
	// 域名
	Domain string `json:"domain"`

	// 是否可注册
	Available bool `json:"available"`

	// 状态 (available/registered/reserved/premium/blocked/unknown)
	Status string `json:"status"`

	// 说明
	Message string `json:"message,omitempty"`
}

// CheckDomainAvailability 检查域名是否可注册
func CheckDomainAvailability(domain string) (*DomainAvailability, error) {
	return CheckDomainAvailabilityWithContext(context.Background(), domain)
}

// CheckDomainAvailabilityWithContext 使用上下文检查域名是否可注册
func CheckDomainAvailabilityWithContext(ctx context.Context, domain string) (*DomainAvailability, error) {
	if domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	result := &DomainAvailability{
		Domain: domain,
		Status: "unknown",
	}

	info, err := ExecuteQueryWithContext(ctx, &QueryOptions{
		Domain: domain,
	})

	if err != nil {
		// 检查特定的解析器错误
		switch {
		case isParserError(err, whoisparser.ErrNotFoundDomain):
			result.Available = true
			result.Status = "available"
			result.Message = "域名可以注册"
			return result, nil
		case isParserError(err, whoisparser.ErrReservedDomain):
			result.Status = "reserved"
			result.Message = "域名已被保留"
			return result, nil
		case isParserError(err, whoisparser.ErrPremiumDomain):
			result.Status = "premium"
			result.Message = "域名可以溢价注册"
			return result, nil
		case isParserError(err, whoisparser.ErrBlockedDomain):
			result.Status = "blocked"
			result.Message = "域名已被品牌保护屏蔽"
			return result, nil
		case isParserError(err, whoisparser.ErrDomainLimitExceed):
			result.Status = "rate_limited"
			result.Message = "查询被限速，请稍后再试"
			return result, err
		}
		return nil, err
	}

	// 如果能获取到WHOIS信息，说明域名已注册
	if info != nil && info.Domain != nil {
		result.Available = false
		result.Status = "registered"
		result.Message = "域名已注册"
	}

	return result, nil
}

// isParserError 检查错误是否匹配特定的解析器错误
func isParserError(err error, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return err.Error() == target.Error()
}
