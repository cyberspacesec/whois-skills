package monitor

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	// 互斥锁保护并发访问
	mu sync.RWMutex

	// 启动时间
	startTime time.Time

	// 总查询次数
	totalQueries int64

	// 成功查询次数
	successfulQueries int64

	// 失败查询次数
	failedQueries int64

	// 代理查询次数
	proxyQueries int64

	// 非代理查询次数
	directQueries int64

	// 查询延迟（毫秒）总和，用于计算平均延迟
	totalLatencyMs int64

	// 最近查询延迟记录，用于计算percentile
	recentLatencies []int64

	// 按域名统计信息
	domainStats map[string]*DomainStat

	// 最近错误记录
	recentErrors []ErrorRecord

	// 最大存储的延迟记录数
	maxLatencyRecords int

	// 最大存储的错误记录数
	maxErrorRecords int
}

// DomainStat 域名统计信息
type DomainStat struct {
	// 查询次数
	Queries int64 `json:"queries"`

	// 成功查询次数
	Successful int64 `json:"successful"`

	// 失败查询次数
	Failed int64 `json:"failed"`

	// 最后查询时间
	LastQuery time.Time `json:"last_query"`

	// 平均查询延迟（毫秒）
	AvgLatencyMs int64 `json:"avg_latency_ms"`
}

// ErrorRecord 错误记录
type ErrorRecord struct {
	// 错误时间
	Time time.Time `json:"time"`

	// 域名
	Domain string `json:"domain"`

	// 错误消息
	Message string `json:"message"`

	// 是否使用代理
	UsedProxy bool `json:"used_proxy"`
}

// PerformanceStats 性能统计信息
type PerformanceStats struct {
	// 运行时间（秒）
	UptimeSeconds int64 `json:"uptime_seconds"`

	// 总查询次数
	TotalQueries int64 `json:"total_queries"`

	// 成功查询次数
	SuccessfulQueries int64 `json:"successful_queries"`

	// 失败查询次数
	FailedQueries int64 `json:"failed_queries"`

	// 代理查询次数
	ProxyQueries int64 `json:"proxy_queries"`

	// 非代理查询次数
	DirectQueries int64 `json:"direct_queries"`

	// 成功率
	SuccessRate float64 `json:"success_rate"`

	// 平均查询延迟（毫秒）
	AvgLatencyMs float64 `json:"avg_latency_ms"`

	// 90%查询延迟（毫秒）
	P90LatencyMs int64 `json:"p90_latency_ms"`

	// 95%查询延迟（毫秒）
	P95LatencyMs int64 `json:"p95_latency_ms"`

	// 99%查询延迟（毫秒）
	P99LatencyMs int64 `json:"p99_latency_ms"`

	// 最近查询的域名（最多10个）
	RecentDomains []string `json:"recent_domains"`

	// 最近错误记录（最多10条）
	RecentErrors []ErrorRecord `json:"recent_errors"`

	// 每秒查询数
	QueriesPerSecond float64 `json:"queries_per_second"`
}

var (
	// 默认性能监控器实例
	defaultMonitor *PerformanceMonitor
	once           sync.Once
)

// GetMonitor 获取性能监控器实例
func GetMonitor() *PerformanceMonitor {
	once.Do(func() {
		defaultMonitor = &PerformanceMonitor{
			startTime:         time.Now(),
			domainStats:       make(map[string]*DomainStat),
			maxLatencyRecords: 1000, // 存储最近1000条延迟记录
			maxErrorRecords:   100,  // 存储最近100条错误记录
		}
	})
	return defaultMonitor
}

// RecordQuery 记录一次查询
func (m *PerformanceMonitor) RecordQuery(domain string, latencyMs int64, successful bool, usedProxy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新总查询统计
	m.totalQueries++
	if successful {
		m.successfulQueries++
	} else {
		m.failedQueries++
	}

	if usedProxy {
		m.proxyQueries++
	} else {
		m.directQueries++
	}

	// 更新延迟统计
	m.totalLatencyMs += latencyMs

	// 保存最近延迟记录（最多保存maxLatencyRecords条）
	if len(m.recentLatencies) >= m.maxLatencyRecords {
		// 移除最旧的记录
		m.recentLatencies = m.recentLatencies[1:]
	}
	m.recentLatencies = append(m.recentLatencies, latencyMs)

	// 更新域名统计
	stat, exists := m.domainStats[domain]
	if !exists {
		stat = &DomainStat{}
		m.domainStats[domain] = stat
	}

	stat.Queries++
	if successful {
		stat.Successful++
	} else {
		stat.Failed++
	}
	stat.LastQuery = time.Now()

	// 更新域名的平均延迟
	if stat.Queries == 1 {
		stat.AvgLatencyMs = latencyMs
	} else {
		// 增量计算平均值
		stat.AvgLatencyMs = (stat.AvgLatencyMs*(stat.Queries-1) + latencyMs) / stat.Queries
	}
}

// RecordError 记录一次错误
func (m *PerformanceMonitor) RecordError(domain string, errMsg string, usedProxy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建错误记录
	record := ErrorRecord{
		Time:      time.Now(),
		Domain:    domain,
		Message:   errMsg,
		UsedProxy: usedProxy,
	}

	// 保存最近错误记录（最多保存maxErrorRecords条）
	if len(m.recentErrors) >= m.maxErrorRecords {
		// 移除最旧的记录
		m.recentErrors = m.recentErrors[1:]
	}
	m.recentErrors = append(m.recentErrors, record)
}

// GetStats 获取当前性能统计信息
func (m *PerformanceMonitor) GetStats() PerformanceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.startTime).Seconds()

	// 计算统计信息
	stats := PerformanceStats{
		UptimeSeconds:     int64(uptime),
		TotalQueries:      m.totalQueries,
		SuccessfulQueries: m.successfulQueries,
		FailedQueries:     m.failedQueries,
		ProxyQueries:      m.proxyQueries,
		DirectQueries:     m.directQueries,
		QueriesPerSecond:  float64(m.totalQueries) / uptime,
	}

	// 计算成功率
	if m.totalQueries > 0 {
		stats.SuccessRate = float64(m.successfulQueries) / float64(m.totalQueries) * 100
	}

	// 计算平均延迟
	if m.totalQueries > 0 {
		stats.AvgLatencyMs = float64(m.totalLatencyMs) / float64(m.totalQueries)
	}

	// 计算延迟百分位数
	if len(m.recentLatencies) > 0 {
		// 复制一份延迟数据进行排序
		latencies := make([]int64, len(m.recentLatencies))
		copy(latencies, m.recentLatencies)
		stats.P90LatencyMs = percentile(latencies, 90)
		stats.P95LatencyMs = percentile(latencies, 95)
		stats.P99LatencyMs = percentile(latencies, 99)
	}

	// 收集最近查询的域名
	recentDomains := make([]string, 0, 10)
	domainLastQuery := make(map[string]time.Time)

	// 构建域名和最后查询时间的映射
	for domain, stat := range m.domainStats {
		domainLastQuery[domain] = stat.LastQuery
	}

	// 按最后查询时间排序
	for domain, _ := range m.domainStats {
		if len(recentDomains) < 10 {
			recentDomains = append(recentDomains, domain)
		} else {
			// 找出最早的记录
			earliest := recentDomains[0]
			earliestTime := domainLastQuery[earliest]

			for i := 1; i < len(recentDomains); i++ {
				if domainLastQuery[recentDomains[i]].Before(earliestTime) {
					earliest = recentDomains[i]
					earliestTime = domainLastQuery[earliest]
				}
			}

			// 如果当前域名的查询时间比最早的记录更晚，则替换
			if domainLastQuery[domain].After(earliestTime) {
				for i := 0; i < len(recentDomains); i++ {
					if recentDomains[i] == earliest {
						recentDomains[i] = domain
						break
					}
				}
			}
		}
	}
	stats.RecentDomains = recentDomains

	// 收集最近错误记录
	if len(m.recentErrors) > 0 {
		count := min(len(m.recentErrors), 10)
		stats.RecentErrors = make([]ErrorRecord, count)
		// 复制最近的错误记录
		copy(stats.RecentErrors, m.recentErrors[len(m.recentErrors)-count:])
	}

	return stats
}

// LogStats 定期记录性能统计信息到日志
func (m *PerformanceMonitor) LogStats(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		stats := m.GetStats()

		statsJSON, err := json.Marshal(stats)
		if err != nil {
			logrus.Errorf("无法序列化性能统计信息: %v", err)
			continue
		}

		logrus.Infof("性能统计: %s", string(statsJSON))
	}
}

// 启动定期日志记录
func StartPerformanceLogging(interval time.Duration) {
	monitor := GetMonitor()
	go monitor.LogStats(interval)
}

// percentile 计算百分位数
func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}

	// 复制切片以避免修改输入
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	index := int(float64(len(sorted)-1) * float64(p) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	if index < 0 {
		index = 0
	}

	return sorted[index]
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// WithPerformanceMonitoring 包装查询函数，添加性能监控
func WithPerformanceMonitoring(domain string, usedProxy bool, fn func() (interface{}, error)) (interface{}, error) {
	startTime := time.Now()
	result, err := fn()
	latencyMs := time.Since(startTime).Milliseconds()

	monitor := GetMonitor()
	monitor.RecordQuery(domain, latencyMs, err == nil, usedProxy)

	if err != nil {
		monitor.RecordError(domain, err.Error(), usedProxy)
	}

	return result, err
}
