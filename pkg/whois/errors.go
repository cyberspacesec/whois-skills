package whois

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorType 枚举类型，表示不同的错误情况
type ErrorType int

// 定义错误类型枚举值
const (
	ErrConnectionReset ErrorType = iota + 1 // 连接被对方重置
	ErrIntervalTooShort                     // 查询间隔太短
	ErrServerConnectFailed                  // 连接到WHOIS服务器失败
	ErrAccessTooFast                        // 访问太快，请稍后再试
	ErrQueryTimeout                         // 查询超时
	ErrDomainEmpty                          // 域名为空
	ErrServerNotFound                       // WHOIS服务器未找到
	ErrParseFailed                          // WHOIS信息解析失败
	ErrValidationFailed                     // 查询结果验证失败
	ErrProxyFailed                          // 代理连接失败
	ErrCacheMiss                            // 缓存未命中
	ErrRateLimited                          // 被限速
	ErrReferralFailed                       // 引导查询失败
	ErrDomainNotRegistered                  // 域名未注册
	ErrDomainReserved                       // 域名被保留
	ErrDomainBlocked                        // 域名被屏蔽
)

// WhoisError 实现error接口的WHOIS错误类型
type WhoisError struct {
	// 错误类型
	Type ErrorType

	// 错误描述信息
	Message string

	// 原始错误（支持errors.Is/As）
	Cause error
}

// Error 实现error接口
func (e *WhoisError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap 支持errors.Is/As链式错误查找
func (e *WhoisError) Unwrap() error {
	return e.Cause
}

// IsRetryable 判断错误是否可重试
func (e *WhoisError) IsRetryable() bool {
	switch e.Type {
	case ErrConnectionReset, ErrIntervalTooShort, ErrAccessTooFast,
		ErrServerConnectFailed, ErrRateLimited, ErrQueryTimeout:
		return true
	default:
		return false
	}
}

// NewWhoisError 创建新的WhoisError
func NewWhoisError(errType ErrorType, message string, cause error) *WhoisError {
	return &WhoisError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// CheckError 检查错误并返回对应的WhoisError
func CheckError(err error) *WhoisError {
	if err == nil {
		return nil
	}
	// 如果已经是WhoisError，直接返回
	var whoisErr *WhoisError
	if errors.As(err, &whoisErr) {
		return whoisErr
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection reset by peer"):
		return NewWhoisError(ErrConnectionReset, "连接被对方重置", err)
	case strings.Contains(msg, "queried interval is too short"):
		return NewWhoisError(ErrIntervalTooShort, "查询间隔太短", err)
	case strings.Contains(msg, "connect to whois server failed"):
		return NewWhoisError(ErrServerConnectFailed, "连接WHOIS服务器失败", err)
	case strings.Contains(msg, "access is too fast") || strings.Contains(msg, "your access is too fast"):
		return NewWhoisError(ErrAccessTooFast, "访问太快", err)
	case strings.Contains(msg, "query limit exceeded") || strings.Contains(msg, "limit exceeded"):
		return NewWhoisError(ErrRateLimited, "查询被限速", err)
	case strings.Contains(msg, "查询超时") || strings.Contains(msg, "timeout") || strings.Contains(msg, "context deadline"):
		return NewWhoisError(ErrQueryTimeout, "查询超时", err)
	default:
		return NewWhoisError(ErrorType(0), "未知错误", err)
	}
}

// 为了保持向后兼容性，保留旧的类型别名和常量别名

// Deprecated: 使用WhoisError代替
type ErrorWrapper = WhoisError

// Deprecated: 使用NewWhoisError代替
func NewErrorWrapper(err error, errType ErrorType) WhoisError {
	return WhoisError{
		Type:    errType,
		Message: err.Error(),
		Cause:   err,
	}
}

// Deprecated: 使用ErrConnectionReset代替
const ConnectionResetByPeer = ErrConnectionReset

// Deprecated: 使用ErrIntervalTooShort代替
const QueriedIntervalTooShort = ErrIntervalTooShort

// Deprecated: 使用ErrServerConnectFailed代替
const ConnectToWhoisServerFailed = ErrServerConnectFailed

// Deprecated: 使用ErrAccessTooFast代替
const AccessTooFastPleaseTryAgainLater = ErrAccessTooFast