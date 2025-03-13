package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleHealth(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/api/health", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	server.handleHealth(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusOK, rr.Code)

	// 解析响应
	var response APIResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// 检查响应内容
	assert.True(t, response.Success)
	data, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "ok", data["status"])
	assert.NotNil(t, data["time"])
}

func TestHandleMetrics(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)
	server.EnableMetrics = true

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/api/metrics", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	server.handleMetrics(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusOK, rr.Code)

	// 解析响应
	var response APIResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// 检查响应内容
	assert.True(t, response.Success)
	assert.NotNil(t, response.Data)
}

func TestHandleMetricsDisabled(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)
	server.EnableMetrics = false

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/api/metrics", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	server.handleMetrics(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	// 解析响应
	var response APIResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// 检查响应内容
	assert.False(t, response.Success)
	assert.Equal(t, "监控功能未启用", response.Error)
}

func TestHandleAlertsDisabled(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)
	server.EnableAlerts = false

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/api/alerts", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	server.handleAlerts(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	// 解析响应
	var response APIResponse
	err = json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)

	// 检查响应内容
	assert.False(t, response.Success)
	assert.Equal(t, "告警功能未启用", response.Error)
}

func TestMiddleware(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)

	// 创建一个测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// 应用中间件
	handler := server.addMiddleware(testHandler)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	handler.ServeHTTP(rr, req)

	// 检查状态码
	assert.Equal(t, http.StatusOK, rr.Code)

	// 检查CORS头
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
}

func TestAddMiddleware(t *testing.T) {
	// 创建服务器实例
	server := NewServer("localhost", 8080)

	// 创建一个测试中间件
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "test")
			next.ServeHTTP(w, r)
		})
	}

	// 添加中间件
	server.AddMiddleware(testMiddleware)

	// 创建一个测试处理函数
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 应用中间件
	handler := server.addMiddleware(testHandler)

	// 创建一个测试HTTP请求
	req, err := http.NewRequest("GET", "/test", nil)
	assert.NoError(t, err)

	// 创建一个ResponseRecorder来记录响应
	rr := httptest.NewRecorder()

	// 调用处理函数
	handler.ServeHTTP(rr, req)

	// 检查自定义头
	assert.Equal(t, "test", rr.Header().Get("X-Test"))
}
