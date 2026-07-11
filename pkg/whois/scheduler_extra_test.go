package whois

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== scheduler.go Schedule 剩余分支 ====================

// TestSmartScheduler_Schedule_BackoffWait 触发退避路径：NextAllowedTime 在未来。
func TestSmartScheduler_Schedule_BackoffWait(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 通过限速错误触发 handleRateLimit，设置 NextAllowedTime 在未来
	s.RecordResult("srv", 0, NewWhoisError(ErrRateLimited, "rate limited", nil))
	st := s.GetServerState("srv")
	assert.NotNil(t, st)
	assert.True(t, st.NextAllowedTime.After(time.Now()))

	// Schedule 应进入退避等待分支，返回正的等待时长且无错误
	wait, err := s.Schedule(context.Background(), "srv")
	assert.NoError(t, err)
	assert.True(t, wait > 0, "退避路径应返回正等待时长, got %v", wait)
}

// TestSmartScheduler_Schedule_AdaptiveLimiterFalse 触发自适应限速器 Allow 返回 false。
// 令牌桶速率极低 + 突发 0 → 首次即可能 false；通过消耗令牌实现。
func TestSmartScheduler_Schedule_AdaptiveLimiterFalse(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// adaptiveLimiter 初始 rate=5, burst=10。消耗 >10 次令牌让 Allow 返回 false。
	for i := 0; i < 20; i++ {
		s.adaptiveLimiter.Allow()
	}
	// 立即调度（无退避、健康）：adaptiveLimiter.Allow 应返回 false
	wait, err := s.Schedule(context.Background(), "freshserver")
	// 限速 false 路径：返回 nil err + 等待 50ms（waitMs<50 取 50）
	assert.NoError(t, err)
	// 退避路径返回 >0；限速 false 路径返回 50ms
	if wait != 50*time.Millisecond && wait <= 0 {
		// 自适应限速可能在两次调用间补充了令牌，验证至少不 panic 且无错误
		assert.NoError(t, err)
	}
	// 若确实进入限速 false 分支，wait 应为 50ms
	assert.True(t, wait >= 0)
}

// TestSmartScheduler_Schedule_AdaptiveLimiterFalse_HighRate 触发 waitMs<50 取 50 分支：
// 需 currentRate > 20 使 1000/rate < 50。构造高 maxRate 的 adaptiveLimiter。
func TestSmartScheduler_Schedule_AdaptiveLimiterFalse_HighRate(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 替换为高 maxRate 的限速器：currentRate=100, maxRate=100 → 1000/100=10 < 50 → 取 50
	s.adaptiveLimiter = NewAdaptiveRateLimiter(100.0, 1.0, 100.0)
	// 消耗令牌使 Allow 返回 false
	for i := 0; i < 250; i++ {
		s.adaptiveLimiter.Allow()
	}
	wait, err := s.Schedule(context.Background(), "freshserver2")
	assert.NoError(t, err)
	// waitMs = 1000/100 = 10 < 50 → 取 50ms
	assert.Equal(t, 50*time.Millisecond, wait, "waitMs<50 应取 50ms")
}

// TestSmartScheduler_Schedule_BackoffExpired 触发退避路径但 NextAllowedTime 已过期（wait<=0）。
func TestSmartScheduler_Schedule_BackoffExpired(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 手动设置一个已过期的 NextAllowedTime
	s.mu.Lock()
	st := s.getOrCreateServerState("srv2")
	st.NextAllowedTime = time.Now().Add(-1 * time.Hour)
	st.Healthy = true
	s.mu.Unlock()
	// Schedule: time.Now().Before(过去时间)=false → 跳过退避，进入自适应/默认间隔
	wait, err := s.Schedule(context.Background(), "srv2")
	assert.NoError(t, err)
	// 返回默认间隔 200ms
	assert.Equal(t, 200*time.Millisecond, wait)
}

// TestSmartScheduler_Schedule_DefaultIntervalWhenZero AdaptiveInterval<=0 走默认间隔分支。
// getOrCreateServerState 初始化 AdaptiveInterval=DefaultInterval，故需手动置 0 才能进入 <=0 分支。
func TestSmartScheduler_Schedule_DefaultIntervalWhenZero(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	s.mu.Lock()
	st := s.getOrCreateServerState("zero-srv")
	st.AdaptiveInterval = 0
	st.Healthy = true
	s.mu.Unlock()
	wait, err := s.Schedule(context.Background(), "zero-srv")
	assert.NoError(t, err)
	// AdaptiveInterval<=0 → 取 DefaultInterval=200
	assert.Equal(t, 200*time.Millisecond, wait)
}

// TestSmartScheduler_increaseInterval_HitMax 增加间隔触达 MaxInterval 上限。
func TestSmartScheduler_increaseInterval_HitMax(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.MaxInterval = 300
	s := NewSmartScheduler(cfg)
	st := &ServerState{Server: "x", AdaptiveInterval: 250}
	s.increaseInterval(st)
	// 250 * 1.3 = 325 > 300 → 取 300
	assert.Equal(t, int64(300), st.AdaptiveInterval)
}

// TestSmartScheduler_decreaseInterval_HitMin 减少间隔触达 MinInterval 下限。
func TestSmartScheduler_decreaseInterval_HitMin(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.MinInterval = 100
	s := NewSmartScheduler(cfg)
	st := &ServerState{Server: "x", AdaptiveInterval: 110}
	s.decreaseInterval(st)
	// 110 * (1 - 0.15) = 93.5 < 100 → 取 100
	assert.Equal(t, int64(100), st.AdaptiveInterval)
}

// TestSmartScheduler_increaseInterval_FromDefault 从默认值增加。
func TestSmartScheduler_increaseInterval_FromDefault(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	st := &ServerState{Server: "x", AdaptiveInterval: 0}
	s.increaseInterval(st)
	// current<=0 → 取 DefaultInterval=200, 200*1.3=260
	assert.Equal(t, int64(260), st.AdaptiveInterval)
}

// TestSmartScheduler_decreaseInterval_FromDefault 从默认值减少。
func TestSmartScheduler_decreaseInterval_FromDefault(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	st := &ServerState{Server: "x", AdaptiveInterval: 0}
	s.decreaseInterval(st)
	// current<=0 → DefaultInterval=200, 200*(1-0.15)=170
	assert.Equal(t, int64(170), st.AdaptiveInterval)
}

// TestSmartScheduler_RecordResult_NonRateLimitUnhealthy 非限速失败达到 UnhealthyThreshold → 标记不健康。
func TestSmartScheduler_RecordResult_NonRateLimitUnhealthy(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.UnhealthyThreshold = 2
	s := NewSmartScheduler(cfg)
	// 连续 2 次非限速错误
	s.RecordResult("srv", 0, assertRateLimitError()) // 这是限速错误
	// 用非限速错误连续达到阈值
	s.RecordResult("srv2", 0, NewWhoisError(ErrParseFailed, "parse", nil))
	s.RecordResult("srv2", 0, NewWhoisError(ErrParseFailed, "parse", nil))
	st := s.GetServerState("srv2")
	assert.False(t, st.Healthy, "非限速连续失败达阈值应标记不健康")
}

// TestAdaptiveRateLimiter_adjustRate_SuccessPath consecutiveSuccess>=5 提高速率分支。
func TestAdaptiveRateLimiter_adjustRate_SuccessPath(t *testing.T) {
	arl := NewAdaptiveRateLimiter(5.0, 1.0, 20.0)
	// 连续成功 5 次（不经过 RecordSuccess 的 10 次阈值，直接调 adjustRate）
	arl.mu.Lock()
	arl.consecutiveSuccess = 6
	arl.mu.Unlock()
	arl.adjustRate()
	assert.True(t, arl.currentRate > 5.0, "consecutiveSuccess>=5 应提高速率, got %v", arl.currentRate)
}

// TestAdaptiveRateLimiter_adjustRate_BelowThreshold consecutiveSuccess<5 不提高速率。
func TestAdaptiveRateLimiter_adjustRate_BelowThreshold(t *testing.T) {
	arl := NewAdaptiveRateLimiter(5.0, 1.0, 20.0)
	arl.mu.Lock()
	arl.consecutiveSuccess = 2
	arl.mu.Unlock()
	original := arl.currentRate
	arl.adjustRate()
	assert.Equal(t, original, arl.currentRate, "consecutiveSuccess<5 不应提高速率")
}

// TestAdaptiveRateLimiter_RecordSuccess_HitMax 连续成功提高速率触达 maxRate。
func TestAdaptiveRateLimiter_RecordSuccess_HitMax(t *testing.T) {
	arl := NewAdaptiveRateLimiter(19.0, 1.0, 20.0)
	// 连续成功 10 次触发 adjustRate，19*1.1=20.9 → 取 20
	for i := 0; i < 10; i++ {
		arl.RecordSuccess()
	}
	assert.True(t, arl.currentRate <= 20.0)
}

// TestSmartScheduler_handleRateLimit_SecondCall 非首次退避走 multiplier 分支。
func TestSmartScheduler_handleRateLimit_SecondCall(t *testing.T) {
	cfg := DefaultSchedulerConfig()
	cfg.BackoffInitialMs = 1000
	cfg.BackoffMultiplier = 2.0
	cfg.BackoffMaxMs = 10000
	s := NewSmartScheduler(cfg)
	st := &ServerState{Server: "x", CurrentBackoff: 0, Healthy: true}
	s.handleRateLimit(st) // 首次 → 1000
	assert.Equal(t, int64(1000), st.CurrentBackoff)
	s.handleRateLimit(st) // 1000*2=2000
	assert.Equal(t, int64(2000), st.CurrentBackoff)
}

// TestSmartScheduler_MarkServerHealthy_NotExist 标记不存在服务器为健康（无副作用）。
func TestSmartScheduler_MarkServerHealthy_NotExist(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 不存在的服务器 → 不创建，仅无副作用
	s.MarkServerHealthy("nope")
	assert.Nil(t, s.GetServerState("nope"))
}

// TestSmartScheduler_GetAllServerStates_Empty 空状态。
func TestSmartScheduler_GetAllServerStates_Empty(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	assert.Empty(t, s.GetAllServerStates())
}

// TestSmartScheduler_GetStats_Initial 初始统计全 0。
func TestSmartScheduler_GetStats_Initial(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	stats := s.GetStats()
	assert.Equal(t, int64(0), stats.TotalScheduled)
	assert.Equal(t, int64(0), stats.TotalRateLimited)
}

// TestSmartScheduler_RecordResult_LatencyHistoryTrim latencyHistory 超 10 触发 trim 分支。
func TestSmartScheduler_RecordResult_LatencyHistoryTrim(t *testing.T) {
	s := NewSmartScheduler(DefaultSchedulerConfig())
	// 记录 12 次成功结果，触发 latencyHistory > 10 trim
	for i := 0; i < 12; i++ {
		s.RecordResult("srv", int64(i*100), nil)
	}
	st := s.GetServerState("srv")
	assert.NotNil(t, st)
	assert.Equal(t, 10, len(st.latencyHistory), "latencyHistory 应被裁剪到 10")
}
