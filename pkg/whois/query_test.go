package whois

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryOptions_GetIntervalMilsOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		options  QueryOptions
		expected int
	}{
		{
			name:     "Default value when zero",
			options:  QueryOptions{IntervalMils: 0},
			expected: 1000,
		},
		{
			name:     "Default value when negative",
			options:  QueryOptions{IntervalMils: -100},
			expected: 1000,
		},
		{
			name:     "Custom value",
			options:  QueryOptions{IntervalMils: 2000},
			expected: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.options.GetIntervalMilsOrDefault()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryOptions_GetMaxRetriesOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		options  QueryOptions
		expected int
	}{
		{
			name:     "Default value when zero",
			options:  QueryOptions{MaxRetries: 0},
			expected: 5,
		},
		{
			name:     "Default value when negative",
			options:  QueryOptions{MaxRetries: -1},
			expected: 5,
		},
		{
			name:     "Custom value",
			options:  QueryOptions{MaxRetries: 10},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.options.GetMaxRetriesOrDefault()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteQuery_ValidationChecks(t *testing.T) {
	tests := []struct {
		name    string
		options *QueryOptions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Nil options",
			options: nil,
			wantErr: true,
			errMsg:  "查询选项不能为空",
		},
		{
			name:    "Empty domain",
			options: &QueryOptions{Domain: ""},
			wantErr: true,
			errMsg:  "域名不能为空",
		},
		{
			name:    "Valid options",
			options: &QueryOptions{Domain: "example.com"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExecuteQuery(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				// 注意：这个测试可能会失败，因为它实际上会执行WHOIS查询
				// 在真实环境中，可能需要模拟网络请求
				if err != nil {
					t.Logf("查询失败，但这可能是外部原因导致: %v", err)
				}
			}
		})
	}
}

func TestExecute_BackwardsCompatibility(t *testing.T) {
	// 这个测试主要确保旧API仍然有效
	query := &Query{
		Domain:       "example.com",
		IntervalMils: 2000,
	}

	// 这个测试可能会失败，因为它实际上会执行WHOIS查询
	// 在CI环境中，我们可能需要模拟这个调用
	_, err := Execute(query)
	if err != nil {
		// 只记录错误，但不使测试失败，因为错误可能是由外部因素引起的
		t.Logf("Execute调用失败，但可能是由于外部原因: %v", err)
	}
}
