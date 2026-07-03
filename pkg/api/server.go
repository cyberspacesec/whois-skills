package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cyberspacesec/whois-skills/pkg/mcp"
	"github.com/cyberspacesec/whois-skills/pkg/metrics"
	"github.com/cyberspacesec/whois-skills/pkg/whois"
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

	// 批量查询会话
	batchSessions sync.Map
}

// batchSession 批量查询会话
type batchSession struct {
	ID        string
	Processor *whois.StreamBatchProcessor
	Stats     whois.StreamBatchStats
	Domains   []string
	CreatedAt time.Time
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
	router := s.CreateHandler()

	// 启动服务器
	return http.ListenAndServe(addr, router)
}

// CreateHandler 创建HTTP处理器（包含所有路由和中间件），可供外部http.Server使用
func (s *Server) CreateHandler() http.Handler {
	return s.addMiddleware(s.createRouter())
}

// createRouter 创建HTTP路由器
func (s *Server) createRouter() http.Handler {
	router := http.NewServeMux()

	// === WHOIS 核心查询 ===
	router.HandleFunc("/api/whois", s.handleWhoisQuery)
	router.HandleFunc("/api/ip", s.handleIPQuery)
	router.HandleFunc("/api/asn", s.handleASNQuery)

	// === RDAP 查询 ===
	router.HandleFunc("/api/rdap/domain", s.handleRDAPDomainQuery)
	router.HandleFunc("/api/rdap/ip", s.handleRDAPIPQuery)
	router.HandleFunc("/api/rdap/asn", s.handleRDAPASNQuery)

	// === 域名分析 ===
	router.HandleFunc("/api/availability", s.handleAvailabilityCheck)
	router.HandleFunc("/api/diff", s.handleDiff)
	router.HandleFunc("/api/quality", s.handleQuality)
	router.HandleFunc("/api/correlation", s.handleCorrelation)

	// === 批量查询 ===
	router.HandleFunc("/api/batch", s.handleBatchQuery)
	router.HandleFunc("/api/batch/status", s.handleBatchStatus)

	// === 格式化与导出 ===
	router.HandleFunc("/api/format", s.handleFormat)
	router.HandleFunc("/api/export/json", s.handleExportJSON)
	router.HandleFunc("/api/export/csv", s.handleExportCSV)
	router.HandleFunc("/api/export/markdown", s.handleExportMarkdown)

	// === IDN 与工具 ===
	router.HandleFunc("/api/idn", s.handleIDN)
	router.HandleFunc("/api/servers", s.handleServers)

	// === 系统端点 ===
	router.HandleFunc("/api/metrics", s.handleMetrics)
	router.HandleFunc("/api/alerts", s.handleAlerts)
	router.HandleFunc("/api/health", s.handleHealth)

	// === MCP 端点 ===
	s.registerMCPRoutes(router)

	return router
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

// ============================================================
// WHOIS 核心查询端点
// ============================================================

// handleWhoisQuery 处理WHOIS查询请求
func (s *Server) handleWhoisQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain         string   `json:"domain"`
		UseProxy       bool     `json:"use_proxy,omitempty"`
		Timeout        int      `json:"timeout,omitempty"`
		MaxRetries     int      `json:"max_retries,omitempty"`
		ValidateResult bool     `json:"validate_result,omitempty"`
		RequiredFields []string `json:"required_fields,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	if req.Timeout <= 0 {
		req.Timeout = 10
	}

	startTime := time.Now()
	result, err := whois.ExecuteQueryWithResult(&whois.QueryOptions{
		Domain:         req.Domain,
		UseProxy:       req.UseProxy,
		Timeout:        req.Timeout,
		MaxRetries:     req.MaxRetries,
		ValidateResult: req.ValidateResult,
		RequiredFields: req.RequiredFields,
		Priority:       1,
	})

	duration := time.Since(startTime)
	if s.EnableMetrics {
		metrics.GetCollector().RecordWHOISQuery(result.Server, err == nil, duration)
	}

	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// handleIPQuery 处理IP WHOIS查询请求
func (s *Server) handleIPQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		IP       string `json:"ip"`
		Timeout  int    `json:"timeout,omitempty"`
		UseProxy bool   `json:"use_proxy,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.IP == "" {
		SendErrorResponse(w, http.StatusBadRequest, "IP地址不能为空")
		return
	}

	result, err := whois.QueryIPWithOptions(&whois.IPWhoisOptions{
		IP:       req.IP,
		Timeout:  req.Timeout,
		UseProxy: req.UseProxy,
	})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("IP查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// handleASNQuery 处理ASN查询请求
func (s *Server) handleASNQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		ASN             int    `json:"asn"`
		Timeout         int    `json:"timeout,omitempty"`
		Source          string `json:"source,omitempty"`
		IncludePrefixes bool   `json:"include_prefixes,omitempty"`
		IncludeBGP      bool   `json:"include_bgp,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.ASN <= 0 {
		SendErrorResponse(w, http.StatusBadRequest, "ASN必须为正整数")
		return
	}

	source := whois.ASNSourceAll
	switch req.Source {
	case "radb":
		source = whois.ASNSourceRADB
	case "rdap":
		source = whois.ASNSourceRDAP
	}

	result, err := whois.QueryASNWithContext(r.Context(), &whois.ASNQueryOptions{
		ASN:             req.ASN,
		Timeout:         req.Timeout,
		Source:          source,
		IncludePrefixes: req.IncludePrefixes,
		IncludeBGP:      req.IncludeBGP,
	})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("ASN查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// ============================================================
// RDAP 查询端点
// ============================================================

// handleRDAPDomainQuery 处理RDAP域名查询请求
func (s *Server) handleRDAPDomainQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain  string `json:"domain"`
		Timeout int    `json:"timeout,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	result, err := whois.QueryRDAPWithContext(r.Context(), &whois.RDAPQueryOptions{
		Domain:  req.Domain,
		Timeout: req.Timeout,
	})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("RDAP域名查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// handleRDAPIPQuery 处理RDAP IP查询请求
func (s *Server) handleRDAPIPQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		IP      string `json:"ip"`
		Timeout int    `json:"timeout,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.IP == "" {
		SendErrorResponse(w, http.StatusBadRequest, "IP地址不能为空")
		return
	}

	result, err := whois.QueryRDAP_IPWithContext(r.Context(), &whois.RDAPQueryOptions{
		IP:      req.IP,
		Timeout: req.Timeout,
	})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("RDAP IP查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// handleRDAPASNQuery 处理RDAP ASN查询请求
func (s *Server) handleRDAPASNQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		ASN     string `json:"asn"`
		Timeout int    `json:"timeout,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.ASN == "" {
		SendErrorResponse(w, http.StatusBadRequest, "ASN不能为空")
		return
	}

	result, err := whois.QueryRDAP_ASNWithContext(r.Context(), &whois.RDAPQueryOptions{
		ASN:     req.ASN,
		Timeout: req.Timeout,
	})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("RDAP ASN查询失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// ============================================================
// 域名分析端点
// ============================================================

// handleAvailabilityCheck 处理域名可用性检查请求
func (s *Server) handleAvailabilityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	result, err := whois.CheckDomainAvailabilityWithContext(r.Context(), req.Domain)
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("可用性检查失败: %v", err))
		return
	}

	SendSuccessResponse(w, result)
}

// handleDiff 处理WHOIS对比请求
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain1 string `json:"domain1"`
		Domain2 string `json:"domain2"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain1 == "" || req.Domain2 == "" {
		SendErrorResponse(w, http.StatusBadRequest, "两个域名都不能为空")
		return
	}

	// 查询两个域名的WHOIS信息
	info1, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain1})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询 %s 失败: %v", req.Domain1, err))
		return
	}

	info2, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain2})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询 %s 失败: %v", req.Domain2, err))
		return
	}

	changes := whois.CompareWhois(info1, info2)
	SendSuccessResponse(w, map[string]interface{}{
		"domain1": req.Domain1,
		"domain2": req.Domain2,
		"changes": changes,
		"count":   len(changes),
	})
}

// handleQuality 处理WHOIS质量评估请求
func (s *Server) handleQuality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	info, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	score := whois.AssessQuality(info)
	SendSuccessResponse(w, score)
}

// handleCorrelation 处理关联分析请求
func (s *Server) handleCorrelation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domains []string `json:"domains"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if len(req.Domains) < 2 {
		SendErrorResponse(w, http.StatusBadRequest, "至少需要2个域名进行关联分析")
		return
	}

	engine := whois.NewCorrelationEngine()
	for _, domain := range req.Domains {
		info, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: domain})
		if err != nil {
			logrus.Warnf("关联分析: 查询 %s 失败: %v", domain, err)
			continue
		}
		engine.AddDomain(domain, info)
	}

	result := engine.Analyze()
	SendSuccessResponse(w, result)
}

// ============================================================
// 批量查询端点
// ============================================================

// handleBatchQuery 处理批量查询请求
func (s *Server) handleBatchQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domains       []string `json:"domains"`
		Concurrency   int      `json:"concurrency,omitempty"`
		Timeout       int      `json:"timeout,omitempty"`
		MaxRetries    int      `json:"max_retries,omitempty"`
		QueryDelay    int      `json:"query_delay_ms,omitempty"`
		UseProxy      bool     `json:"use_proxy,omitempty"`
		CheckpointDir string   `json:"checkpoint_dir,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if len(req.Domains) == 0 {
		SendErrorResponse(w, http.StatusBadRequest, "域名列表不能为空")
		return
	}

	config := whois.DefaultStreamBatchConfig()
	if req.Concurrency > 0 {
		config.Concurrency = req.Concurrency
	}
	if req.Timeout > 0 {
		config.Timeout = req.Timeout
	}
	if req.MaxRetries > 0 {
		config.MaxRetries = req.MaxRetries
	}
	if req.QueryDelay > 0 {
		config.QueryDelay = req.QueryDelay
	}
	config.UseProxy = req.UseProxy

	processor := whois.NewStreamBatchProcessor(config)

	// 生成会话ID
	sessionID := fmt.Sprintf("batch-%d", time.Now().UnixNano())

	session := &batchSession{
		ID:        sessionID,
		Processor: processor,
		Domains:   req.Domains,
		CreatedAt: time.Now(),
	}
	s.batchSessions.Store(sessionID, session)

	// 异步启动批量查询
	go func() {
		ctx := context.Background()
		if err := processor.Process(ctx, req.Domains); err != nil {
			logrus.Errorf("批量查询启动失败: %v", err)
		}
	}()

	SendSuccessResponse(w, map[string]interface{}{
		"session_id":  sessionID,
		"total":       len(req.Domains),
		"message":     "批量查询已启动",
		"status_url":  fmt.Sprintf("/api/batch/status?id=%s", sessionID),
	})
}

// handleBatchStatus 处理批量查询状态请求
func (s *Server) handleBatchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		SendErrorResponse(w, http.StatusBadRequest, "缺少会话ID参数")
		return
	}

	val, ok := s.batchSessions.Load(sessionID)
	if !ok {
		SendErrorResponse(w, http.StatusNotFound, "会话不存在")
		return
	}

	session := val.(*batchSession)
	stats := session.Processor.GetStats()

	SendSuccessResponse(w, map[string]interface{}{
		"session_id": sessionID,
		"stats":      stats,
	})
}

// ============================================================
// 格式化与导出端点
// ============================================================

// handleFormat 处理格式化请求
func (s *Server) handleFormat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		RawResponse string `json:"raw_response"`
		DetectOnly  bool   `json:"detect_only,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.RawResponse == "" {
		SendErrorResponse(w, http.StatusBadRequest, "原始响应不能为空")
		return
	}

	format := whois.DetectWhoisFormat(req.RawResponse)
	result := map[string]interface{}{
		"format": format,
	}

	if !req.DetectOnly {
		result["formatted"] = whois.FormatRawResponse(req.RawResponse)
	}

	SendSuccessResponse(w, result)
}

// handleExportJSON 处理JSON导出请求
func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	info, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	var buf bytes.Buffer
	if err := whois.ExportToJSON(info, &buf); err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("导出失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}

// handleExportCSV 处理CSV导出请求
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	info, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	var buf bytes.Buffer
	if err := whois.ExportToCSV(info, &buf); err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("导出失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", req.Domain))
	w.Write(buf.Bytes())
}

// handleExportMarkdown 处理Markdown导出请求
func (s *Server) handleExportMarkdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	info, err := whois.ExecuteQueryWithContext(r.Context(), &whois.QueryOptions{Domain: req.Domain})
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}

	var buf bytes.Buffer
	if err := whois.ExportToMarkdown(info, &buf); err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("导出失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/markdown")
	w.Write(buf.Bytes())
}

// ============================================================
// IDN 与工具端点
// ============================================================

// handleIDN 处理IDN转换请求
func (s *Server) handleIDN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持POST请求")
		return
	}

	var req struct {
		Domain string `json:"domain"`
		Action string `json:"action,omitempty"` // normalize, to_punycode, to_unicode, check
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendErrorResponse(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Domain == "" {
		SendErrorResponse(w, http.StatusBadRequest, "域名不能为空")
		return
	}

	if req.Action == "" {
		req.Action = "normalize"
	}

	result := map[string]interface{}{
		"original": req.Domain,
		"is_idn":   whois.IsIDN(req.Domain),
	}

	switch req.Action {
	case "normalize":
		normalized, err := whois.NormalizeDomain(req.Domain)
		if err != nil {
			SendErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("规范化失败: %v", err))
			return
		}
		result["normalized"] = normalized
	case "to_punycode":
		punycode, err := whois.UnicodeToPunycode(req.Domain)
		if err != nil {
			SendErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("转换失败: %v", err))
			return
		}
		result["punycode"] = punycode
	case "to_unicode":
		unicode, err := whois.PunycodeToUnicode(req.Domain)
		if err != nil {
			SendErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("转换失败: %v", err))
			return
		}
		result["unicode"] = unicode
	case "check":
		// 只返回 is_idn 信息
	default:
		SendErrorResponse(w, http.StatusBadRequest, "无效的action，支持: normalize, to_punycode, to_unicode, check")
		return
	}

	SendSuccessResponse(w, result)
}

// handleServers 处理服务器列表请求
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendErrorResponse(w, http.StatusMethodNotAllowed, "仅支持GET请求")
		return
	}

	manager := whois.GetServerManager()
	servers := manager.GetAllServers()
	stats := manager.GetServerStats()

	SendSuccessResponse(w, map[string]interface{}{
		"servers": servers,
		"stats":   stats,
	})
}

// ============================================================
// 系统端点
// ============================================================

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

// registerMCPRoutes 注册MCP协议路由到http.ServeMux
func (s *Server) registerMCPRoutes(router *http.ServeMux) {
	mcpServer := mcp.NewServer(logrus.StandardLogger())

	router.HandleFunc("/api/mcp/request_planning", mcpServer.HandleRequestPlanning())
	router.HandleFunc("/api/mcp/get_next_task", mcpServer.HandleGetNextTask())
	router.HandleFunc("/api/mcp/mark_task_done", mcpServer.HandleMarkTaskDone())
	router.HandleFunc("/api/mcp/approve_task_completion", mcpServer.HandleApproveTaskCompletion())
	router.HandleFunc("/api/mcp/approve_request_completion", mcpServer.HandleApproveRequestCompletion())
	router.HandleFunc("/api/mcp/open_task_details", mcpServer.HandleOpenTaskDetails())
	router.HandleFunc("/api/mcp/list_requests", mcpServer.HandleListRequests())
	router.HandleFunc("/api/mcp/add_tasks_to_request", mcpServer.HandleAddTasksToRequest())
	router.HandleFunc("/api/mcp/update_task", mcpServer.HandleUpdateTask())
	router.HandleFunc("/api/mcp/delete_task", mcpServer.HandleDeleteTask())
}
