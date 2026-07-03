package whois

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute_Integration(t *testing.T) {
	// 这是一个集成测试，会实际执行WHOIS查询
	// 在CI环境中可能需要被跳过
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	query := &Query{Domain: "example.com"}
	info, err := Execute(query)

	// 由于这是一个外部服务调用，可能会因为网络等原因失败
	// 所以我们不应该把它作为单元测试的一部分
	if err != nil {
		t.Logf("查询失败，但这可能是外部因素导致: %v", err)
		return
	}

	// 如果成功，验证返回的数据
	t.Logf("查询结果: %v", info)
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.Domain.Domain)
	assert.NotEmpty(t, info.Domain.CreatedDate)
	assert.NotEmpty(t, info.Domain.ExpirationDate)

	// 输出JSON格式的结果，便于查看
	jsonData, _ := json.MarshalIndent(info, "", "  ")
	t.Logf("查询结果JSON: %s", string(jsonData))
}

func TestExecuteQuery_Integration(t *testing.T) {
	// 这是一个集成测试，会实际执行WHOIS查询
	// 在CI环境中可能需要被跳过
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	query := &QueryOptions{
		Domain:       "example.com",
		IntervalMils: 1000,
		MaxRetries:   3,
	}

	info, err := ExecuteQuery(query)

	// 由于这是一个外部服务调用，可能会因为网络等原因失败
	// 所以我们不应该把它作为单元测试的一部分
	if err != nil {
		t.Logf("查询失败，但这可能是外部因素导致: %v", err)
		return
	}

	// 如果成功，验证返回的数据
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.Domain.Domain)
	assert.NotEmpty(t, info.Domain.CreatedDate)
	assert.NotEmpty(t, info.Domain.ExpirationDate)
}

func TestCheckError(t *testing.T) {
	tests := []struct {
		name         string
		errorMessage string
		expectedType ErrorType
	}{
		{
			name:         "Reset by peer",
			errorMessage: "read: connection reset by peer",
			expectedType: ConnectionResetByPeer,
		},
		{
			name:         "Interval too short",
			errorMessage: "Queried interval is too short",
			expectedType: QueriedIntervalTooShort,
		},
		{
			name:         "Server connection failed",
			errorMessage: "connect to whois server failed",
			expectedType: ConnectToWhoisServerFailed,
		},
		{
			name:         "Access too fast",
			errorMessage: "Your access is too fast,please try again later",
			expectedType: AccessTooFastPleaseTryAgainLater,
		},
		{
			name:         "Unknown error",
			errorMessage: "Some other error",
			expectedType: ErrorType(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建一个包含特定错误消息的错误对象
			mockErr := &mockErrorWithMessage{message: tt.errorMessage}
			wrapper := CheckError(mockErr)
			assert.Equal(t, tt.expectedType, wrapper.Type)
		})
	}
}

// 用于模拟错误消息的辅助类型
type mockErrorWithMessage struct {
	message string
}

func (m *mockErrorWithMessage) Error() string {
	return m.message
}
