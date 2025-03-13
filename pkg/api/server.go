package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cyberspacesec/whois-hacker/pkg/metrics"
	"github.com/cyberspacesec/whois-hacker/pkg/whois"
	"github.com/sirupsen/logrus"
)

// Server API服务器
type Server struct {
	// 服务器配置
	Host string
	Port int

	// 功能开关
	EnableProxy   bool
	EnableCache   bool
	EnableMetrics bool
	EnableAlerts  bool

	// 中间件
	middlewares []func(http.Handler) http.Handler
}

// NewServer 创建API服务器
func NewServer(host string, port int) *Server {
	return &Server{
		Host:        host,
		Port:        port,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	logrus.Infof("API服务正在启动，监听地址: %s", addr)

	// 创建路由器
	router := s.createRouter()

	// 启动服务器
	return http.ListenAndServe(addr, router)
}

// createRouter 创建HTTP路由器
func (s *Server) createRouter() http.Handler {
	router := http.NewServeMux()

	// 注册路由
	router.HandleFunc("/api/whois", s.handleWhoisQuery)
	router.HandleFunc("/api/metrics", s.handleMetrics)
	router.HandleFunc("/api/alerts", s.handleAlerts)
	router.HandleFunc("/api/health", s.handleHealth)

	// 添加中间件
	handler := s.addMiddleware(router)

	return handler
}

// addMiddleware 添加中间件
func (s *Server) addMiddleware(next http.Handler) http.Handler {
	// 添加认证中间件
	handler := AuthMiddleware(next)

	// 添加CORS中间件
	handler = CORSMiddleware(handler)

	// 添加请求日志中间件
	handler = LoggingMiddleware(handler)

	// 添加恢复中间件
	handler = RecoveryMiddleware(handler)

	// 添加自定义中间件
	for _, mw := range s.middlewares {
		handler = mw(handler)
	}

	return handler
}

// AddMiddleware 添加自定义中间件
func (s *Server) AddMiddleware(middleware func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middleware)
}

// handleWhoisQuery 处理WHOIS查询请求
func (s *Server) handleWhoisQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	// 解析请求
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	// 验证域名
	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	// 执行查询
	startTime := time.Now()
	result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{
		Domain:         req.Domain,
		UseProxy:       s.EnableProxy,
		Timeout:        10,
		Priority:       1,
		ValidateResult: true,
	})

	// 记录指标
	duration := time.Since(startTime)
	if s.EnableMetrics {
		metrics.GetCollector().RecordWHOISQuery(result.Server, err == nil, duration)
	}

	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	// 返回结果
	SendSuccessResponse(w, result)
}

// handleMetrics 处理指标查询请求
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.EnableMetrics {
		SendErrorResponse(w, http.StatusServiceUnavailable, "监控功能未启用")
		return
	}

	if r.Method != http.MethodGet {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	// 获取指标
	collector := metrics.GetCollector()
	metricsData := collector.GetMetrics()

	// 返回结果
	SendSuccessResponse(w, metricsData)
}

// handleAlerts 处理告警查询请求
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if !s.EnableAlerts {
		SendErrorResponse(w, http.StatusServiceUnavailable, "告警功能未启用")
		return
	}

	if r.Method != http.MethodGet {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	// 获取告警历史
	manager := metrics.GetAlertManager()
	history := manager.GetHistory()

	// 返回结果
	SendSuccessResponse(w, history)
}

// handleHealth 处理健康检查请求
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	// 返回健康状态
	SendSuccessResponse(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now(),
	})
}
