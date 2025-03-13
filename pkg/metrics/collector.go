package metrics

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MetricsCollector 性能指标收集器
type MetricsCollector struct {
	mu sync.RWMutex

	// API请求指标
	APIMetrics APIMetrics

	// WHOIS查询指标
	WHOISMetrics WHOISMetrics

	// 缓存指标
	CacheMetrics CacheMetrics

	// 系统指标
	SystemMetrics SystemMetrics

	// 最后更新时间
	LastUpdated time.Time
}

// APIMetrics API相关指标
type APIMetrics struct {
	// 总请求数
	TotalRequests int64

	// 成功请求数
	SuccessfulRequests int64

	// 失败请求数
	FailedRequests int64

	// 平均响应时间（毫秒）
	AvgResponseTime int64

	// 最大响应时间（毫秒）
	MaxResponseTime int64

	// 最小响应时间（毫秒）
	MinResponseTime int64

	// 按状态码统计
	StatusCodes map[int]int64

	// 按路径统计
	PathStats map[string]*PathStats
}

// PathStats 路径统计信息
type PathStats struct {
	// 请求数
	Requests int64

	// 平均响应时间
	AvgResponseTime int64

	// 成功率
	SuccessRate float64
}

// WHOISMetrics WHOIS查询指标
type WHOISMetrics struct {
	// 总查询数
	TotalQueries int64

	// 成功查询数
	SuccessfulQueries int64

	// 失败查询数
	FailedQueries int64

	// 平均查询时间（毫秒）
	AvgQueryTime int64

	// 最大查询时间（毫秒）
	MaxQueryTime int64

	// 最小查询时间（毫秒）
	MinQueryTime int64

	// 按服务器统计
	ServerStats map[string]*ServerStats
}

// ServerStats 服务器统计信息
type ServerStats struct {
	// 查询数
	Queries int64

	// 成功数
	Successes int64

	// 失败数
	Failures int64

	// 平均响应时间
	AvgResponseTime int64
}

// CacheMetrics 缓存指标
type CacheMetrics struct {
	// 缓存命中数
	Hits int64

	// 缓存未命中数
	Misses int64

	// 缓存条目数
	Entries int64

	// 过期条目数
	Expired int64

	// 缓存命中率
	HitRate float64

	// 内存使用（字节）
	MemoryUsage int64
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	// CPU使用率
	CPUUsage float64

	// 内存使用率
	MemoryUsage float64

	// Goroutine数量
	GoroutineCount int64

	// 系统负载
	SystemLoad float64
}

var (
	defaultCollector *MetricsCollector
	collectorOnce    sync.Once
)

// GetCollector 获取指标收集器实例
func GetCollector() *MetricsCollector {
	collectorOnce.Do(func() {
		defaultCollector = &MetricsCollector{
			APIMetrics: APIMetrics{
				StatusCodes: make(map[int]int64),
				PathStats:   make(map[string]*PathStats),
			},
			WHOISMetrics: WHOISMetrics{
				ServerStats: make(map[string]*ServerStats),
			},
		}
	})
	return defaultCollector
}

// RecordAPIRequest 记录API请求指标
func (mc *MetricsCollector) RecordAPIRequest(path string, statusCode int, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 更新总体指标
	mc.APIMetrics.TotalRequests++
	if statusCode >= 200 && statusCode < 400 {
		mc.APIMetrics.SuccessfulRequests++
	} else {
		mc.APIMetrics.FailedRequests++
	}

	// 更新响应时间
	durationMs := duration.Milliseconds()
	if mc.APIMetrics.MinResponseTime == 0 || durationMs < mc.APIMetrics.MinResponseTime {
		mc.APIMetrics.MinResponseTime = durationMs
	}
	if durationMs > mc.APIMetrics.MaxResponseTime {
		mc.APIMetrics.MaxResponseTime = durationMs
	}

	// 更新平均响应时间
	mc.APIMetrics.AvgResponseTime = (mc.APIMetrics.AvgResponseTime*
		(mc.APIMetrics.TotalRequests-1) + durationMs) / mc.APIMetrics.TotalRequests

	// 更新状态码统计
	mc.APIMetrics.StatusCodes[statusCode]++

	// 更新路径统计
	pathStats, exists := mc.APIMetrics.PathStats[path]
	if !exists {
		pathStats = &PathStats{}
		mc.APIMetrics.PathStats[path] = pathStats
	}
	pathStats.Requests++
	pathStats.AvgResponseTime = (pathStats.AvgResponseTime*(pathStats.Requests-1) + durationMs) / pathStats.Requests
	pathStats.SuccessRate = float64(mc.APIMetrics.SuccessfulRequests) / float64(mc.APIMetrics.TotalRequests) * 100

	mc.LastUpdated = time.Now()
}

// RecordWHOISQuery 记录WHOIS查询指标
func (mc *MetricsCollector) RecordWHOISQuery(server string, success bool, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 更新总体指标
	mc.WHOISMetrics.TotalQueries++
	if success {
		mc.WHOISMetrics.SuccessfulQueries++
	} else {
		mc.WHOISMetrics.FailedQueries++
	}

	// 更新查询时间
	durationMs := duration.Milliseconds()
	if mc.WHOISMetrics.MinQueryTime == 0 || durationMs < mc.WHOISMetrics.MinQueryTime {
		mc.WHOISMetrics.MinQueryTime = durationMs
	}
	if durationMs > mc.WHOISMetrics.MaxQueryTime {
		mc.WHOISMetrics.MaxQueryTime = durationMs
	}

	// 更新平均查询时间
	mc.WHOISMetrics.AvgQueryTime = (mc.WHOISMetrics.AvgQueryTime*
		(mc.WHOISMetrics.TotalQueries-1) + durationMs) / mc.WHOISMetrics.TotalQueries

	// 更新服务器统计
	serverStats, exists := mc.WHOISMetrics.ServerStats[server]
	if !exists {
		serverStats = &ServerStats{}
		mc.WHOISMetrics.ServerStats[server] = serverStats
	}
	serverStats.Queries++
	if success {
		serverStats.Successes++
	} else {
		serverStats.Failures++
	}
	serverStats.AvgResponseTime = (serverStats.AvgResponseTime*(serverStats.Queries-1) + durationMs) / serverStats.Queries

	mc.LastUpdated = time.Now()
}

// UpdateCacheMetrics 更新缓存指标
func (mc *MetricsCollector) UpdateCacheMetrics(metrics CacheMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.CacheMetrics = metrics
	mc.LastUpdated = time.Now()
}

// UpdateSystemMetrics 更新系统指标
func (mc *MetricsCollector) UpdateSystemMetrics(metrics SystemMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.SystemMetrics = metrics
	mc.LastUpdated = time.Now()
}

// GetMetrics 获取所有指标
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return map[string]interface{}{
		"api": map[string]interface{}{
			"total_requests":      mc.APIMetrics.TotalRequests,
			"successful_requests": mc.APIMetrics.SuccessfulRequests,
			"failed_requests":     mc.APIMetrics.FailedRequests,
			"avg_response_time":   mc.APIMetrics.AvgResponseTime,
			"max_response_time":   mc.APIMetrics.MaxResponseTime,
			"min_response_time":   mc.APIMetrics.MinResponseTime,
			"status_codes":        mc.APIMetrics.StatusCodes,
			"path_stats":          mc.APIMetrics.PathStats,
		},
		"whois": map[string]interface{}{
			"total_queries":      mc.WHOISMetrics.TotalQueries,
			"successful_queries": mc.WHOISMetrics.SuccessfulQueries,
			"failed_queries":     mc.WHOISMetrics.FailedQueries,
			"avg_query_time":     mc.WHOISMetrics.AvgQueryTime,
			"max_query_time":     mc.WHOISMetrics.MaxQueryTime,
			"min_query_time":     mc.WHOISMetrics.MinQueryTime,
			"server_stats":       mc.WHOISMetrics.ServerStats,
		},
		"cache": map[string]interface{}{
			"hits":         mc.CacheMetrics.Hits,
			"misses":       mc.CacheMetrics.Misses,
			"entries":      mc.CacheMetrics.Entries,
			"expired":      mc.CacheMetrics.Expired,
			"hit_rate":     mc.CacheMetrics.HitRate,
			"memory_usage": mc.CacheMetrics.MemoryUsage,
		},
		"system": map[string]interface{}{
			"cpu_usage":       mc.SystemMetrics.CPUUsage,
			"memory_usage":    mc.SystemMetrics.MemoryUsage,
			"goroutine_count": mc.SystemMetrics.GoroutineCount,
			"system_load":     mc.SystemMetrics.SystemLoad,
		},
		"last_updated": mc.LastUpdated,
	}
}

// ExportMetrics 导出指标到JSON文件
func (mc *MetricsCollector) ExportMetrics(filename string) error {
	metrics := mc.GetMetrics()
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	logrus.Infof("指标已导出到文件: %s", filename)
	return nil
}

// StartMetricsCollection 启动定期指标收集
func (mc *MetricsCollector) StartMetricsCollection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			// 更新系统指标
			metrics := collectSystemMetrics()
			mc.UpdateSystemMetrics(metrics)
		}
	}()
}
