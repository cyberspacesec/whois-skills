package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	// 创建一个测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 应用中间件
	handler := AuthMiddleware(testHandler)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	handler.ServeHTTP(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCORSMiddleware(t *testing.T) {
	// 创建一个测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 应用中间件
	handler := CORSMiddleware(testHandler)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		checkHeaders   bool
	}{
		{
			name:           "Normal GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
		{
			name:           "OPTIONS request",
			method:         "OPTIONS",
			expectedStatus: http.StatusOK,
			checkHeaders:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建一个测试HTTP请求
			req, err := http.NewRequest(tt.method, "/test", nil)
			assert.NoError(t, err)

			// 创建一个ResponseRecorder来记录响应
			rr := httptest.NewRecorder()

			// 调用处理函数
			handler.ServeHTTP(rr, req)

			// 检查状态码
			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.checkHeaders {
				// 检查CORS头
				assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "GET, POST, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
				assert.Equal(t, "Content-Type, Authorization", rr.Header().Get("Access-Control-Allow-Headers"))
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// 创建一个测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 应用中间件
	handler := LoggingMiddleware(testHandler)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	handler.ServeHTTP(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRecoveryMiddleware(t *testing.T) {
	// 创建一个会panic的测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// 应用中间件
	handler := RecoveryMiddleware(testHandler)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	handler.ServeHTTP(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// 检查响应内容
	var response APIResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Equal(t, "服务器内部错误", response.Error)
}

func TestResponseWriter(t *testing.T) {
	// 创建一个ResponseRecorder
	rr := httptest.NewRecorder()

	// 创建一个responseWriter
	rw := &responseWriter{ResponseWriter: rr}

	// 测试WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)

	// 测试Write
	_, err := rw.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, "test", rr.Body.String())

	// 测试默认状态码
	rw = &responseWriter{ResponseWriter: rr}
	_, err = rw.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rw.statusCode)
}
