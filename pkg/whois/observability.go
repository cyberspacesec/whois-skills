package whois

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsProvider 指标提供者接口
// 可由 Prometheus 或 OpenTelemetry 实现
type MetricsProvider interface {
	// 记录WHOIS查询
	RecordWHOISQuery(server string, success bool, duration time.Duration)

	// 记录缓存操作
	RecordCacheOperation(operation string, hit bool)

	// 记录API请求
	RecordAPIRequest(method, path string, statusCode int, duration time.Duration)

	// 记录限流事件
	RecordRateLimit(server string)

	// 记录活跃查询数
	RecordActiveQueries(count int)

	// 获取提供者名称
	Name() string
}

// CompositeMetrics 组合指标提供者
// 支持同时使用多个指标后端
type CompositeMetrics struct {
	mu        sync.RWMutex
	providers []MetricsProvider

	// 内置统计（即使没有外部提供者也能工作）
	whoisQueryCount    int64
	whoisSuccessCount  int64
	whoisFailureCount  int64
	cacheHitCount      int64
	cacheMissCount     int64
	apiRequestCount    int64
	rateLimitCount     int64
	totalQueryDuration int64
}

// NewCompositeMetrics 创建组合指标
func NewCompositeMetrics(providers ...MetricsProvider) *CompositeMetrics {
	return &CompositeMetrics{
		providers: providers,
	}
}

// AddProvider 添加指标提供者
func (cm *CompositeMetrics) AddProvider(provider MetricsProvider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.providers = append(cm.providers, provider)
}

// RecordWHOISQuery 记录WHOIS查询
func (cm *CompositeMetrics) RecordWHOISQuery(server string, success bool, duration time.Duration) {
	atomic.AddInt64(&cm.whoisQueryCount, 1)
	if success {
		atomic.AddInt64(&cm.whoisSuccessCount, 1)
	} else {
		atomic.AddInt64(&cm.whoisFailureCount, 1)
	}
	atomic.AddInt64(&cm.totalQueryDuration, duration.Milliseconds())

	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.providers {
		p.RecordWHOISQuery(server, success, duration)
	}
}

// RecordCacheOperation 记录缓存操作
func (cm *CompositeMetrics) RecordCacheOperation(operation string, hit bool) {
	if hit {
		atomic.AddInt64(&cm.cacheHitCount, 1)
	} else {
		atomic.AddInt64(&cm.cacheMissCount, 1)
	}

	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.providers {
		p.RecordCacheOperation(operation, hit)
	}
}

// RecordAPIRequest 记录API请求
func (cm *CompositeMetrics) RecordAPIRequest(method, path string, statusCode int, duration time.Duration) {
	atomic.AddInt64(&cm.apiRequestCount, 1)

	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.providers {
		p.RecordAPIRequest(method, path, statusCode, duration)
	}
}

// RecordRateLimit 记录限流事件
func (cm *CompositeMetrics) RecordRateLimit(server string) {
	atomic.AddInt64(&cm.rateLimitCount, 1)

	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.providers {
		p.RecordRateLimit(server)
	}
}

// RecordActiveQueries 记录活跃查询数
func (cm *CompositeMetrics) RecordActiveQueries(count int) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.providers {
		p.RecordActiveQueries(count)
	}
}

// GetBuiltInStats 获取内置统计信息
func (cm *CompositeMetrics) GetBuiltInStats() BuiltInStats {
	return BuiltInStats{
		TotalQueries:      atomic.LoadInt64(&cm.whoisQueryCount),
		SuccessfulQueries: atomic.LoadInt64(&cm.whoisSuccessCount),
		FailedQueries:     atomic.LoadInt64(&cm.whoisFailureCount),
		CacheHits:         atomic.LoadInt64(&cm.cacheHitCount),
		CacheMisses:       atomic.LoadInt64(&cm.cacheMissCount),
		APIRequests:       atomic.LoadInt64(&cm.apiRequestCount),
		RateLimitEvents:   atomic.LoadInt64(&cm.rateLimitCount),
		TotalQueryTimeMs:  atomic.LoadInt64(&cm.totalQueryDuration),
	}
}

// BuiltInStats 内置统计信息
type BuiltInStats struct {
	TotalQueries      int64 `json:"total_queries"`
	SuccessfulQueries int64 `json:"successful_queries"`
	FailedQueries     int64 `json:"failed_queries"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
	APIRequests       int64 `json:"api_requests"`
	RateLimitEvents   int64 `json:"rate_limit_events"`
	TotalQueryTimeMs  int64 `json:"total_query_time_ms"`
}

// PrometheusMetricsProvider Prometheus指标提供者
// 通过接口暴露metrics，用户可注册到自己的prometheus registry
type PrometheusMetricsProvider struct {
	mu sync.RWMutex

	// 指标存储（简化实现，用户可替换为真正的prometheus.Counter等）
	counters   map[string]int64
	histograms map[string]*SimpleHistogram
	gauges     map[string]int64
}

// SimpleHistogram 简化直方图
type SimpleHistogram struct {
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
}

// NewPrometheusMetricsProvider 创建Prometheus指标提供者
func NewPrometheusMetricsProvider() *PrometheusMetricsProvider {
	return &PrometheusMetricsProvider{
		counters:   make(map[string]int64),
		histograms: make(map[string]*SimpleHistogram),
		gauges:     make(map[string]int64),
	}
}

// RecordWHOISQuery 记录WHOIS查询
func (p *PrometheusMetricsProvider) RecordWHOISQuery(server string, success bool, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters["whois_queries_total"]++

	if success {
		p.counters["whois_queries_success"]++
	} else {
		p.counters["whois_queries_failed"]++
	}

	// 按服务器记录
	key := fmt.Sprintf("whois_queries_{server=\"%s\"}", server)
	p.counters[key]++

	// 记录延迟直方图
	durationMs := float64(duration.Milliseconds())
	hist, ok := p.histograms["whois_query_duration_ms"]
	if !ok {
		hist = &SimpleHistogram{}
		p.histograms["whois_query_duration_ms"] = hist
	}
	hist.Count++
	hist.Sum += durationMs
	if hist.Min == 0 || durationMs < hist.Min {
		hist.Min = durationMs
	}
	if durationMs > hist.Max {
		hist.Max = durationMs
	}
	hist.Avg = hist.Sum / float64(hist.Count)
}

// RecordCacheOperation 记录缓存操作
func (p *PrometheusMetricsProvider) RecordCacheOperation(operation string, hit bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if hit {
		p.counters["cache_hits_total"]++
	} else {
		p.counters["cache_misses_total"]++
	}
}

// RecordAPIRequest 记录API请求
func (p *PrometheusMetricsProvider) RecordAPIRequest(method, path string, statusCode int, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters["api_requests_total"]++

	key := fmt.Sprintf("api_requests_{method=\"%s\",path=\"%s\",status=\"%d\"}", method, path, statusCode)
	p.counters[key]++
}

// RecordRateLimit 记录限流事件
func (p *PrometheusMetricsProvider) RecordRateLimit(server string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.counters["rate_limits_total"]++
	key := fmt.Sprintf("rate_limits_{server=\"%s\"}", server)
	p.counters[key]++
}

// RecordActiveQueries 记录活跃查询数
func (p *PrometheusMetricsProvider) RecordActiveQueries(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gauges["active_queries"] = int64(count)
}

// Name 提供者名称
func (p *PrometheusMetricsProvider) Name() string {
	return "prometheus"
}

// GetCounters 获取所有计数器
func (p *PrometheusMetricsProvider) GetCounters() map[string]int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]int64, len(p.counters))
	for k, v := range p.counters {
		result[k] = v
	}
	return result
}

// GetHistograms 获取所有直方图
func (p *PrometheusMetricsProvider) GetHistograms() map[string]SimpleHistogram {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]SimpleHistogram, len(p.histograms))
	for k, v := range p.histograms {
		result[k] = *v
	}
	return result
}

// GetGauges 获取所有仪表
func (p *PrometheusMetricsProvider) GetGauges() map[string]int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]int64, len(p.gauges))
	for k, v := range p.gauges {
		result[k] = v
	}
	return result
}

// ExportPrometheusFormat 导出Prometheus文本格式
func (p *PrometheusMetricsProvider) ExportPrometheusFormat() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result string

	// 计数器
	for name, value := range p.counters {
		result += fmt.Sprintf("# HELP %s Total count\n", name)
		result += fmt.Sprintf("# TYPE %s counter\n", name)
		result += fmt.Sprintf("%s %d\n\n", name, value)
	}

	// 直方图
	for name, hist := range p.histograms {
		result += fmt.Sprintf("# HELP %s Duration histogram\n", name)
		result += fmt.Sprintf("# TYPE %s histogram\n", name)
		result += fmt.Sprintf("%s_count %d\n", name, hist.Count)
		result += fmt.Sprintf("%s_sum %f\n", name, hist.Sum)
		result += fmt.Sprintf("%s_min %f\n", name, hist.Min)
		result += fmt.Sprintf("%s_max %f\n", name, hist.Max)
		result += fmt.Sprintf("\n")
	}

	// 仪表
	for name, value := range p.gauges {
		result += fmt.Sprintf("# HELP %s Current value\n", name)
		result += fmt.Sprintf("# TYPE %s gauge\n", name)
		result += fmt.Sprintf("%s %d\n\n", name, value)
	}

	return result
}

// OpenTelemetryMetricsProvider OpenTelemetry指标提供者
type OpenTelemetryMetricsProvider struct {
	mu sync.RWMutex

	// 指标存储（简化实现）
	counters   map[string]int64
	histograms map[string]*SimpleHistogram
	gauges     map[string]int64
	traces     []*OtelTrace
}

// OtelTrace 简化的OpenTelemetry Trace
type OtelTrace struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Start     time.Time         `json:"start"`
	End       time.Time         `json:"end"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Status    string            `json:"status"`
}

// NewOpenTelemetryMetricsProvider 创建OpenTelemetry指标提供者
func NewOpenTelemetryMetricsProvider() *OpenTelemetryMetricsProvider {
	return &OpenTelemetryMetricsProvider{
		counters:   make(map[string]int64),
		histograms: make(map[string]*SimpleHistogram),
		gauges:     make(map[string]int64),
		traces:     make([]*OtelTrace, 0),
	}
}

// RecordWHOISQuery 记录WHOIS查询
func (o *OpenTelemetryMetricsProvider) RecordWHOISQuery(server string, success bool, duration time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.counters["whois.queries.total"]++

	key := fmt.Sprintf("whois.queries{server=%s,success=%v}", server, success)
	o.counters[key]++

	durationMs := float64(duration.Milliseconds())
	hist, ok := o.histograms["whois.query.duration"]
	if !ok {
		hist = &SimpleHistogram{}
		o.histograms["whois.query.duration"] = hist
	}
	hist.Count++
	hist.Sum += durationMs
	if hist.Min == 0 || durationMs < hist.Min {
		hist.Min = durationMs
	}
	if durationMs > hist.Max {
		hist.Max = durationMs
	}
	hist.Avg = hist.Sum / float64(hist.Count)
}

// RecordCacheOperation 记录缓存操作
func (o *OpenTelemetryMetricsProvider) RecordCacheOperation(operation string, hit bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if hit {
		o.counters["whois.cache.hits"]++
	} else {
		o.counters["whois.cache.misses"]++
	}
}

// RecordAPIRequest 记录API请求
func (o *OpenTelemetryMetricsProvider) RecordAPIRequest(method, path string, statusCode int, duration time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.counters["whois.api.requests"]++

	// 创建trace
	trace := &OtelTrace{
		TraceID: fmt.Sprintf("trace-%d", time.Now().UnixNano()),
		SpanID:  fmt.Sprintf("span-%d", time.Now().UnixNano()),
		Name:    fmt.Sprintf("%s %s", method, path),
		Kind:    "SERVER",
		Start:   time.Now().Add(-duration),
		End:     time.Now(),
		Attributes: map[string]string{
			"http.method":      method,
			"http.path":        path,
			"http.status_code": fmt.Sprintf("%d", statusCode),
		},
	}
	if statusCode >= 200 && statusCode < 400 {
		trace.Status = "OK"
	} else {
		trace.Status = "ERROR"
	}
	o.traces = append(o.traces, trace)
	if len(o.traces) > 100 {
		o.traces = o.traces[1:]
	}
}

// RecordRateLimit 记录限流事件
func (o *OpenTelemetryMetricsProvider) RecordRateLimit(server string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.counters["whois.rate_limits"]++
}

// RecordActiveQueries 记录活跃查询数
func (o *OpenTelemetryMetricsProvider) RecordActiveQueries(count int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.gauges["whois.active_queries"] = int64(count)
}

// Name 提供者名称
func (o *OpenTelemetryMetricsProvider) Name() string {
	return "opentelemetry"
}

// GetTraces 获取最近的traces
func (o *OpenTelemetryMetricsProvider) GetTraces() []OtelTrace {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]OtelTrace, len(o.traces))
	for i, trace := range o.traces {
		result[i] = *trace
	}
	return result
}

// GetCounters 获取所有计数器
func (o *OpenTelemetryMetricsProvider) GetCounters() map[string]int64 {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make(map[string]int64, len(o.counters))
	for k, v := range o.counters {
		result[k] = v
	}
	return result
}

// GlobalMetrics 全局指标实例
var globalMetrics *CompositeMetrics
var globalMetricsOnce sync.Once

// GetGlobalMetrics 获取全局指标实例
func GetGlobalMetrics() *CompositeMetrics {
	globalMetricsOnce.Do(func() {
		globalMetrics = NewCompositeMetrics()
	})
	return globalMetrics
}

// InitMetricsWithProviders 使用提供者初始化全局指标
func InitMetricsWithProviders(providers ...MetricsProvider) {
	globalMetricsOnce.Do(func() {
		globalMetrics = NewCompositeMetrics(providers...)
	})
}

// RecordWHOISQuery 便捷函数：记录WHOIS查询到全局指标
func RecordWHOISQuery(server string, success bool, duration time.Duration) {
	GetGlobalMetrics().RecordWHOISQuery(server, success, duration)
}

// RecordCacheOp 便捷函数：记录缓存操作到全局指标
func RecordCacheOp(operation string, hit bool) {
	GetGlobalMetrics().RecordCacheOperation(operation, hit)
}

// RecordAPIReq 便捷函数：记录API请求到全局指标
func RecordAPIReq(method, path string, statusCode int, duration time.Duration) {
	GetGlobalMetrics().RecordAPIRequest(method, path, statusCode, duration)
}

// RecordRateLimitEvent 便捷函数：记录限流事件到全局指标
func RecordRateLimitEvent(server string) {
	GetGlobalMetrics().RecordRateLimit(server)
}

// NopMetricsProvider 空指标提供者（用于禁用指标）
type NopMetricsProvider struct{}

// NewNopMetricsProvider 创建空指标提供者
func NewNopMetricsProvider() *NopMetricsProvider {
	return &NopMetricsProvider{}
}

func (n *NopMetricsProvider) RecordWHOISQuery(_ string, _ bool, _ time.Duration)     {}
func (n *NopMetricsProvider) RecordCacheOperation(_ string, _ bool)                   {}
func (n *NopMetricsProvider) RecordAPIRequest(_, _ string, _ int, _ time.Duration)    {}
func (n *NopMetricsProvider) RecordRateLimit(_ string)                                {}
func (n *NopMetricsProvider) RecordActiveQueries(_ int)                                {}
func (n *NopMetricsProvider) Name() string                                            { return "nop" }

// Context key for metrics
type metricsCtxKey struct{}

// ContextWithMetrics 将指标注入上下文
func ContextWithMetrics(ctx context.Context, metrics *CompositeMetrics) context.Context {
	return context.WithValue(ctx, metricsCtxKey{}, metrics)
}

// MetricsFromContext 从上下文中获取指标
func MetricsFromContext(ctx context.Context) *CompositeMetrics {
	if m, ok := ctx.Value(metricsCtxKey{}).(*CompositeMetrics); ok {
		return m
	}
	return GetGlobalMetrics()
}
