package whois

import (
	"strings"
)

// ErrorType 枚举类型，表示不同的错误情况
type ErrorType int

// 定义枚举值
const (
	ConnectionResetByPeer            ErrorType = iota + 1 // 读：连接被对方重置
	QueriedIntervalTooShort                               // 查询间隔太短
	ConnectToWhoisServerFailed                            // 连接到WHOIS服务器失败
	AccessTooFastPleaseTryAgainLater                      // 访问太快，请稍后再试
)

// ErrorWrapper 封装错误信息和对应的枚举值
type ErrorWrapper struct {
	ErrorType ErrorType
	Message   string
}

// NewErrorWrapper 创建一个新的ErrorWrapper实例
func NewErrorWrapper(err error, errType ErrorType) ErrorWrapper {
	return ErrorWrapper{
		ErrorType: errType,
		Message:   err.Error(),
	}
}

// CheckError 检查错误并返回对应的枚举值
func CheckError(err error) ErrorWrapper {
	if strings.Contains(err.Error(), "read: connection reset by peer") {
		return NewErrorWrapper(err, ConnectionResetByPeer)
	} else if strings.Contains(err.Error(), "Queried interval is too short") {
		return NewErrorWrapper(err, QueriedIntervalTooShort)
	} else if strings.Contains(err.Error(), "connect to whois server failed") {
		return NewErrorWrapper(err, ConnectToWhoisServerFailed)
	} else if strings.Contains(err.Error(), "Your access is too fast,please try again later") {
		return NewErrorWrapper(err, AccessTooFastPleaseTryAgainLater)
	}
	return NewErrorWrapper(err, 0) // 0 表示未知错误
}
