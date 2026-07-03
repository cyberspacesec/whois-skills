package whois

import (
	"sync"
	"time"
)

// RateLimiterConfig 速率限制配置
type RateLimiterConfig struct {
	// GlobalRate 全局速率（每秒请求数）
	GlobalRate float64 `json:"global_rate"`

	// PerServerRate 每服务器速率（每秒请求数）
	PerServerRate map[string]float64 `json:"per_server_rate,omitempty"`

	// BurstSize 突发大小
	BurstSize int `json:"burst_size,omitempty"`
}

// RateLimiter WHOIS查询速率限制器
type RateLimiter struct {
	mu sync.Mutex

	config RateLimiterConfig

	// 每服务器令牌桶
	serverBuckets map[string]*tokenBucket

	// 全局令牌桶
	globalBucket *tokenBucket
}

// tokenBucket 令牌桶
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	rate       float64
	lastRefill time.Time
}

// newTokenBucket 创建新的令牌桶
func newTokenBucket(rate float64, burst int) *tokenBucket {
	maxTokens := float64(burst)
	if maxTokens <= 0 {
		maxTokens = rate // 默认突发 = 1秒的令牌数
	}
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// allow 检查令牌桶是否允许请求
func (tb *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		config:        config,
		serverBuckets: make(map[string]*tokenBucket),
	}
	if config.GlobalRate > 0 {
		rl.globalBucket = newTokenBucket(config.GlobalRate, config.BurstSize)
	}
	for server, rate := range config.PerServerRate {
		rl.serverBuckets[server] = newTokenBucket(rate, config.BurstSize)
	}
	return rl
}

// Allow 检查是否允许对指定服务器的请求
func (rl *RateLimiter) Allow(server string) bool {
	if rl == nil {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 检查全局速率
	if rl.globalBucket != nil && !rl.globalBucket.allow() {
		return false
	}

	// 检查每服务器速率
	bucket, ok := rl.serverBuckets[server]
	if ok {
		return bucket.allow()
	}

	// 如果没有配置该服务器的速率，默认允许
	return true
}

// Wait 等待直到请求被允许
func (rl *RateLimiter) Wait(server string) {
	for !rl.Allow(server) {
		time.Sleep(100 * time.Millisecond)
	}
}
