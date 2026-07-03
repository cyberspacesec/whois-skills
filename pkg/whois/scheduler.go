package whois

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SmartScheduler 智能查询调度器
// 根据服务器响应时间和限速反馈自适应调整查询策略
type SmartScheduler struct {
	mu sync.RWMutex

	// 调度配置
	config SchedulerConfig

	// 服务器状态跟踪
	serverStates map[string]*ServerState

	// 速率限制器
	rateLimiter *RateLimiter

	// 全局自适应限速器
	adaptiveLimiter *AdaptiveRateLimiter

	// 统计信息
	stats SchedulerStats
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// 默认查询间隔（毫秒）
	DefaultInterval int `json:"default_interval_ms"`

	// 最小查询间隔（毫秒）
	MinInterval int `json:"min_interval_ms"`

	// 最大查询间隔（毫秒）
	MaxInterval int `json:"max_interval_ms"`

	// 自适应调整因子 (0.0-1.0)
	AdaptFactor float64 `json:"adapt_factor"`

	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`

	// 限速退避初始时间（毫秒）
	BackoffInitialMs int `json:"backoff_initial_ms"`

	// 限速退避最大时间（毫秒）
	BackoffMaxMs int `json:"backoff_max_ms"`

	// 限速退避倍数
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// 服务器健康检查间隔（秒）
	HealthCheckInterval int `json:"health_check_interval_s"`

	// 服务器不健康阈值（连续失败次数）
	UnhealthyThreshold int `json:"unhealthy_threshold"`

	// 服务器恢复检查间隔（秒）
	RecoveryInterval int `json:"recovery_interval_s"`
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		DefaultInterval:     200,
		MinInterval:         50,
		MaxInterval:         5000,
		AdaptFactor:         0.3,
		MaxConcurrency:      5,
		BackoffInitialMs:    1000,
		BackoffMaxMs:        60000,
		BackoffMultiplier:   2.0,
		HealthCheckInterval: 300,
		UnhealthyThreshold:  3,
		RecoveryInterval:    60,
	}
}

// ServerState 服务器状态跟踪
type ServerState struct {
	// 服务器地址
	Server string `json:"server"`

	// 平均响应时间（毫秒）
	AvgLatency int64 `json:"avg_latency_ms"`

	// 最后一次响应时间
	LastLatency int64 `json:"last_latency_ms"`

	// 查询次数
	QueryCount int64 `json:"query_count"`

	// 失败次数
	FailureCount int64 `json:"failure_count"`

	// 限速次数
	RateLimitedCount int64 `json:"rate_limited_count"`

	// 连续失败次数
	ConsecutiveFailures int `json:"consecutive_failures"`

	// 当前退避时间（毫秒）
	CurrentBackoff int64 `json:"current_backoff_ms"`

	// 下次允许查询时间
	NextAllowedTime time.Time `json:"next_allowed_time"`

	// 服务器是否健康
	Healthy bool `json:"healthy"`

	// 自适应间隔（毫秒）
	AdaptiveInterval int64 `json:"adaptive_interval_ms"`

	// 响应时间历史（最近10次）
	latencyHistory []int64
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	// 总调度次数
	TotalScheduled int64 `json:"total_scheduled"`

	// 限流次数
	TotalRateLimited int64 `json:"total_rate_limited"`

	// 退避次数
	TotalBackoffs int64 `json:"total_backoffs"`

	// 服务器不健康次数
	TotalUnhealthy int64 `json:"total_unhealthy"`

	// 自适应调整次数
	TotalAdaptations int64 `json:"total_adaptations"`
}

// AdaptiveRateLimiter 自适应限速器
// 根据反馈动态调整限速策略
type AdaptiveRateLimiter struct {
	mu sync.Mutex

	// 当前速率 (requests/second)
	currentRate float64

	// 最小速率
	minRate float64

	// 最大速率
	maxRate float64

	// 令牌桶
	bucket *tokenBucket

	// 连续成功次数
	consecutiveSuccess int

	// 连续限速次数
	consecutiveRateLimited int

	// 上次调整时间
	lastAdjust time.Time
}

// NewSmartScheduler 创建智能调度器
func NewSmartScheduler(config SchedulerConfig) *SmartScheduler {
	if config.DefaultInterval <= 0 {
		config.DefaultInterval = 200
	}
	if config.MinInterval <= 0 {
		config.MinInterval = 50
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 5000
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 5
	}
	if config.BackoffInitialMs <= 0 {
		config.BackoffInitialMs = 1000
	}
	if config.BackoffMaxMs <= 0 {
		config.BackoffMaxMs = 60000
	}
	if config.BackoffMultiplier <= 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.AdaptFactor <= 0 || config.AdaptFactor > 1 {
		config.AdaptFactor = 0.3
	}

	return &SmartScheduler{
		config:        config,
		serverStates:  make(map[string]*ServerState),
		adaptiveLimiter: NewAdaptiveRateLimiter(5.0, 1.0, 20.0),
	}
}

// NewAdaptiveRateLimiter 创建自适应限速器
func NewAdaptiveRateLimiter(initialRate, minRate, maxRate float64) *AdaptiveRateLimiter {
	if initialRate <= 0 {
		initialRate = 5.0
	}
	if minRate <= 0 {
		minRate = 1.0
	}
	if maxRate <= 0 {
		maxRate = 20.0
	}
	return &AdaptiveRateLimiter{
		currentRate:  initialRate,
		minRate:      minRate,
		maxRate:      maxRate,
		bucket:       newTokenBucket(initialRate, int(initialRate*2)),
		lastAdjust:   time.Now(),
	}
}

// Schedule 调度一个查询请求
// 返回等待时间，0表示可以立即执行
func (s *SmartScheduler) Schedule(ctx context.Context, server string) (time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 确保服务器状态存在
	state := s.getOrCreateServerState(server)

	// 检查服务器健康状态
	if !state.Healthy {
		return 0, fmt.Errorf("服务器 %s 当前不健康，跳过查询", server)
	}

	// 检查退避时间
	if time.Now().Before(state.NextAllowedTime) {
		waitDuration := time.Until(state.NextAllowedTime)
		return waitDuration, nil
	}

	// 检查自适应限速器
	if !s.adaptiveLimiter.Allow() {
		atomic := s.adaptiveLimiter
		atomic.mu.Lock()
		waitMs := int64(1000.0 / atomic.currentRate)
		atomic.mu.Unlock()
		if waitMs < 50 {
			waitMs = 50
		}
		s.stats.TotalRateLimited++
		return time.Duration(waitMs) * time.Millisecond, nil
	}

	// 计算自适应间隔
	interval := state.AdaptiveInterval
	if interval <= 0 {
		interval = int64(s.config.DefaultInterval)
	}

	s.stats.TotalScheduled++
	return time.Duration(interval) * time.Millisecond, nil
}

// RecordResult 记录查询结果用于自适应调整
func (s *SmartScheduler) RecordResult(server string, latency int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.getOrCreateServerState(server)
	state.QueryCount++
	state.LastLatency = latency

	// 更新响应时间历史
	state.latencyHistory = append(state.latencyHistory, latency)
	if len(state.latencyHistory) > 10 {
		state.latencyHistory = state.latencyHistory[1:]
	}

	// 更新平均延迟
	state.AvgLatency = calculateAvgLatency(state.latencyHistory)

	if err != nil {
		state.FailureCount++
		state.ConsecutiveFailures++

		// 检查是否触发退避
		if isRateLimitError(err) {
			state.RateLimitedCount++
			s.stats.TotalRateLimited++
			s.handleRateLimit(state)
			s.adaptiveLimiter.RecordRateLimited()
		} else if state.ConsecutiveFailures >= s.config.UnhealthyThreshold {
			state.Healthy = false
			s.stats.TotalUnhealthy++
			logrus.Warnf("服务器 %s 标记为不健康 (连续失败 %d 次)", server, state.ConsecutiveFailures)
		}

		// 自适应增加间隔
		s.increaseInterval(state)
	} else {
		state.ConsecutiveFailures = 0
		s.adaptiveLimiter.RecordSuccess()
		s.decreaseInterval(state)
		s.stats.TotalAdaptations++
	}
}

// handleRateLimit 处理限速
func (s *SmartScheduler) handleRateLimit(state *ServerState) {
	if state.CurrentBackoff == 0 {
		state.CurrentBackoff = int64(s.config.BackoffInitialMs)
	} else {
		state.CurrentBackoff = int64(float64(state.CurrentBackoff) * s.config.BackoffMultiplier)
		if state.CurrentBackoff > int64(s.config.BackoffMaxMs) {
			state.CurrentBackoff = int64(s.config.BackoffMaxMs)
		}
	}

	state.NextAllowedTime = time.Now().Add(time.Duration(state.CurrentBackoff) * time.Millisecond)
	s.stats.TotalBackoffs++

	logrus.Infof("服务器 %s 触发限速退避，等待 %d ms", state.Server, state.CurrentBackoff)
}

// increaseInterval 增加查询间隔（响应错误/慢速时）
func (s *SmartScheduler) increaseInterval(state *ServerState) {
	current := state.AdaptiveInterval
	if current <= 0 {
		current = int64(s.config.DefaultInterval)
	}

	// 按调整因子增加
	newInterval := int64(float64(current) * (1.0 + s.config.AdaptFactor))
	if newInterval > int64(s.config.MaxInterval) {
		newInterval = int64(s.config.MaxInterval)
	}

	state.AdaptiveInterval = newInterval
}

// decreaseInterval 减少查询间隔（响应正常/快速时）
func (s *SmartScheduler) decreaseInterval(state *ServerState) {
	current := state.AdaptiveInterval
	if current <= 0 {
		current = int64(s.config.DefaultInterval)
	}

	// 按较小的因子减少
	newInterval := int64(float64(current) * (1.0 - s.config.AdaptFactor*0.5))
	if newInterval < int64(s.config.MinInterval) {
		newInterval = int64(s.config.MinInterval)
	}

	state.AdaptiveInterval = newInterval
}

// getOrCreateServerState 获取或创建服务器状态
func (s *SmartScheduler) getOrCreateServerState(server string) *ServerState {
	if state, ok := s.serverStates[server]; ok {
		return state
	}

	state := &ServerState{
		Server:           server,
		Healthy:          true,
		AdaptiveInterval: int64(s.config.DefaultInterval),
		latencyHistory:   make([]int64, 0),
	}
	s.serverStates[server] = state
	return state
}

// GetServerState 获取服务器状态
func (s *SmartScheduler) GetServerState(server string) *ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if state, ok := s.serverStates[server]; ok {
		result := *state
		return &result
	}
	return nil
}

// GetAllServerStates 获取所有服务器状态
func (s *SmartScheduler) GetAllServerStates() map[string]ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]ServerState)
	for k, v := range s.serverStates {
		result[k] = *v
	}
	return result
}

// GetStats 获取统计信息
func (s *SmartScheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// MarkServerHealthy 手动标记服务器为健康
func (s *SmartScheduler) MarkServerHealthy(server string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state, ok := s.serverStates[server]; ok {
		state.Healthy = true
		state.ConsecutiveFailures = 0
		state.CurrentBackoff = 0
	}
}

// MarkServerUnhealthy 手动标记服务器为不健康
func (s *SmartScheduler) MarkServerUnhealthy(server string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.getOrCreateServerState(server)
	state.Healthy = false
	s.stats.TotalUnhealthy++
}

// AdaptiveRateLimiter methods

// Allow 检查自适应限速器是否允许请求
func (arl *AdaptiveRateLimiter) Allow() bool {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	// 检查是否需要调整速率
	if time.Since(arl.lastAdjust) > 30*time.Second {
		arl.adjustRate()
	}

	return arl.bucket.allow()
}

// RecordSuccess 记录成功请求
func (arl *AdaptiveRateLimiter) RecordSuccess() {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	arl.consecutiveSuccess++
	arl.consecutiveRateLimited = 0

	// 连续成功10次以上，逐步提高速率
	if arl.consecutiveSuccess >= 10 {
		arl.adjustRate()
		arl.consecutiveSuccess = 0
	}
}

// RecordRateLimited 记录被限速
func (arl *AdaptiveRateLimiter) RecordRateLimited() {
	arl.mu.Lock()
	defer arl.mu.Unlock()

	arl.consecutiveRateLimited++
	arl.consecutiveSuccess = 0

	// 立即降低速率
	arl.currentRate = arl.currentRate * 0.7
	if arl.currentRate < arl.minRate {
		arl.currentRate = arl.minRate
	}
	arl.bucket = newTokenBucket(arl.currentRate, int(arl.currentRate*2))
	arl.lastAdjust = time.Now()
}

// adjustRate 根据最近状态调整速率
func (arl *AdaptiveRateLimiter) adjustRate() {
	if arl.consecutiveSuccess >= 5 {
		// 提高速率
		arl.currentRate *= 1.1
		if arl.currentRate > arl.maxRate {
			arl.currentRate = arl.maxRate
		}
	}

	arl.bucket = newTokenBucket(arl.currentRate, int(arl.currentRate*2))
	arl.lastAdjust = time.Now()
}

// GetCurrentRate 获取当前速率
func (arl *AdaptiveRateLimiter) GetCurrentRate() float64 {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	return arl.currentRate
}

// 辅助函数

// isRateLimitError 判断错误是否是限速错误
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	whoisErr := CheckError(err)
	return whoisErr.Type == ErrRateLimited || whoisErr.Type == ErrServerConnectFailed
}

// calculateAvgLatency 计算平均延迟
func calculateAvgLatency(history []int64) int64 {
	if len(history) == 0 {
		return 0
	}
	var sum int64
	for _, v := range history {
		sum += v
	}
	return sum / int64(len(history))
}
