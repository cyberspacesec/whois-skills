package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{
			name:       "Bad Request Error",
			statusCode: http.StatusBadRequest,
			message:    "Invalid request format",
		},
		{
			name:       "Not Found Error",
			statusCode: http.StatusNotFound,
			message:    "Resource not found",
		},
		{
			name:       "Internal Server Error",
			statusCode: http.StatusInternalServerError,
			message:    "Server error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建响应记录器
			rr := httptest.NewRecorder()

			// 发送错误响应
			SendErrorResponse(rr, tt.statusCode, tt.message)

			// 检查状态码
			assert.Equal(t, tt.statusCode, rr.Code)

			// 检查Content-Type
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			// 解析响应
			var response APIResponse
			err := json.NewDecoder(rr.Body).Decode(&response)
			assert.NoError(t, err)

			// 验证响应内容
			assert.False(t, response.Success)
			assert.Equal(t, tt.message, response.Error)
			assert.Empty(t, response.Data)
		})
	}
}

func TestSendSuccessResponse(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		message []string
		want    map[string]interface{}
	}{
		{
			name: "Success with data only",
			data: map[string]string{"key": "value"},
			want: map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"key": "value",
				},
			},
		},
		{
			name:    "Success with data and message",
			data:    []int{1, 2, 3},
			message: []string{"Operation successful"},
			want: map[string]interface{}{
				"success": true,
				"message": "Operation successful",
				"data":    []interface{}{float64(1), float64(2), float64(3)},
			},
		},
		{
			name: "Success with nil data",
			data: nil,
			want: map[string]interface{}{
				"success": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建响应记录器
			rr := httptest.NewRecorder()

			// 发送成功响应
			if len(tt.message) > 0 {
				SendSuccessResponse(rr, tt.data, tt.message[0])
			} else {
				SendSuccessResponse(rr, tt.data)
			}

			// 检查状态码
			assert.Equal(t, http.StatusOK, rr.Code)

			// 检查Content-Type
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

			// 解析响应为map[string]interface{}
			var got map[string]interface{}
			err := json.NewDecoder(rr.Body).Decode(&got)
			assert.NoError(t, err)

			// 验证响应内容
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAPIResponse_Structure(t *testing.T) {
	// 测试APIResponse结构体的JSON序列化
	tests := []struct {
		name     string
		response APIResponse
		want     map[string]interface{}
	}{
		{
			name: "Complete response",
			response: APIResponse{
				Success: true,
				Message: "Success message",
				Data:    map[string]string{"key": "value"},
				Error:   "",
			},
			want: map[string]interface{}{
				"success": true,
				"message": "Success message",
				"data":    map[string]interface{}{"key": "value"},
			},
		},
		{
			name: "Error response",
			response: APIResponse{
				Success: false,
				Error:   "Error message",
			},
			want: map[string]interface{}{
				"success": false,
				"error":   "Error message",
			},
		},
		{
			name: "Minimal success response",
			response: APIResponse{
				Success: true,
				Data:    nil,
			},
			want: map[string]interface{}{
				"success": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 序列化响应
			data, err := json.Marshal(tt.response)
			assert.NoError(t, err)

			// 反序列化为map进行比较
			var got map[string]interface{}
			err = json.Unmarshal(data, &got)
			assert.NoError(t, err)

			// 验证结果
			assert.Equal(t, tt.want, got)
		})
	}
}
