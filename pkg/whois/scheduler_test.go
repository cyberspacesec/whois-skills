package whois

import (
	"context"
	"fmt"
	"testing"
)

func TestNewSmartScheduler(t *testing.T) {
	config := DefaultSchedulerConfig()
	scheduler := NewSmartScheduler(config)

	if scheduler == nil {
		t.Fatal("NewSmartScheduler() returned nil")
	}
	if scheduler.config.DefaultInterval != 200 {
		t.Errorf("DefaultInterval = %d, want 200", scheduler.config.DefaultInterval)
	}
}

func TestNewSmartScheduler_Defaults(t *testing.T) {
	config := SchedulerConfig{}
	scheduler := NewSmartScheduler(config)

	if scheduler.config.DefaultInterval != 200 {
		t.Errorf("Default DefaultInterval = %d, want 200", scheduler.config.DefaultInterval)
	}
	if scheduler.config.MinInterval != 50 {
		t.Errorf("Default MinInterval = %d, want 50", scheduler.config.MinInterval)
	}
	if scheduler.config.MaxInterval != 5000 {
		t.Errorf("Default MaxInterval = %d, want 5000", scheduler.config.MaxInterval)
	}
	if scheduler.config.MaxConcurrency != 5 {
		t.Errorf("Default MaxConcurrency = %d, want 5", scheduler.config.MaxConcurrency)
	}
}

func TestSmartScheduler_Schedule(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	wait, err := scheduler.Schedule(context.Background(), "whois.verisign-grs.com")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	// First schedule should return the default interval
	if wait < 0 {
		t.Errorf("Wait duration should not be negative, got %v", wait)
	}
}

func TestSmartScheduler_Schedule_UnhealthyServer(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.MarkServerUnhealthy("bad.server.com")

	_, err := scheduler.Schedule(context.Background(), "bad.server.com")
	if err == nil {
		t.Error("Expected error for unhealthy server")
	}
}

func TestSmartScheduler_RecordResult_Success(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.RecordResult("whois.example.com", 100, nil)

	state := scheduler.GetServerState("whois.example.com")
	if state == nil {
		t.Fatal("Server state should exist")
	}
	if state.QueryCount != 1 {
		t.Errorf("QueryCount = %d, want 1", state.QueryCount)
	}
	if state.AvgLatency != 100 {
		t.Errorf("AvgLatency = %d, want 100", state.AvgLatency)
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", state.ConsecutiveFailures)
	}
}

func TestSmartScheduler_RecordResult_Failure(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.RecordResult("whois.example.com", 5000, fmt.Errorf("connection timeout"))

	state := scheduler.GetServerState("whois.example.com")
	if state == nil {
		t.Fatal("Server state should exist")
	}
	if state.FailureCount != 1 {
		t.Errorf("FailureCount = %d, want 1", state.FailureCount)
	}
	if state.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %d, want 1", state.ConsecutiveFailures)
	}
}

func TestSmartScheduler_RecordResult_RateLimited(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	// Simulate rate limit error
	scheduler.RecordResult("whois.example.com", 100, NewWhoisError(ErrRateLimited, "rate limited", nil))

	state := scheduler.GetServerState("whois.example.com")
	if state == nil {
		t.Fatal("Server state should exist")
	}
	if state.RateLimitedCount != 1 {
		t.Errorf("RateLimitedCount = %d, want 1", state.RateLimitedCount)
	}
	if state.CurrentBackoff == 0 {
		t.Error("Expected non-zero backoff after rate limit")
	}
}

func TestSmartScheduler_AdaptiveInterval(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	// Record successes - interval should decrease
	for i := 0; i < 5; i++ {
		scheduler.RecordResult("whois.example.com", 50, nil)
	}

	state := scheduler.GetServerState("whois.example.com")
	if state == nil {
		t.Fatal("Server state should exist")
	}

	// After successes, interval should be less than default
	if state.AdaptiveInterval >= int64(scheduler.config.DefaultInterval) {
		t.Errorf("After successes, interval should decrease, got %d (default: %d)",
			state.AdaptiveInterval, scheduler.config.DefaultInterval)
	}

	// Now record failures - interval should increase
	for i := 0; i < 5; i++ {
		scheduler.RecordResult("whois.example.com", 5000, fmt.Errorf("error"))
	}

	state = scheduler.GetServerState("whois.example.com")
	if state.AdaptiveInterval <= int64(scheduler.config.MinInterval) {
		t.Errorf("After failures, interval should increase, got %d", state.AdaptiveInterval)
	}
}

func TestSmartScheduler_UnhealthyThreshold(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.UnhealthyThreshold = 3
	scheduler := NewSmartScheduler(config)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		scheduler.RecordResult("whois.example.com", 0, fmt.Errorf("error %d", i))
	}

	state := scheduler.GetServerState("whois.example.com")
	if state.Healthy {
		t.Error("Server should be marked unhealthy after threshold failures")
	}
}

func TestSmartScheduler_MarkServerHealthy(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.MarkServerUnhealthy("whois.example.com")

	state := scheduler.GetServerState("whois.example.com")
	if state == nil {
		t.Fatal("Server state should exist")
	}
	if state.Healthy {
		t.Error("Server should be unhealthy")
	}

	scheduler.MarkServerHealthy("whois.example.com")

	state = scheduler.GetServerState("whois.example.com")
	if !state.Healthy {
		t.Error("Server should be healthy after MarkServerHealthy")
	}
	if state.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", state.ConsecutiveFailures)
	}
}

func TestSmartScheduler_GetAllServerStates(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.RecordResult("server1.com", 100, nil)
	scheduler.RecordResult("server2.com", 200, nil)

	states := scheduler.GetAllServerStates()
	if len(states) != 2 {
		t.Errorf("States count = %d, want 2", len(states))
	}
}

func TestSmartScheduler_GetStats(t *testing.T) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())

	scheduler.RecordResult("server.com", 100, nil)

	stats := scheduler.GetStats()
	if stats.TotalScheduled == 0 && stats.TotalAdaptations == 0 {
		t.Error("Expected some stats after recording results")
	}
}

func TestAdaptiveRateLimiter(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(5.0, 1.0, 20.0)

	if limiter == nil {
		t.Fatal("NewAdaptiveRateLimiter() returned nil")
	}
	if limiter.currentRate != 5.0 {
		t.Errorf("Initial rate = %f, want 5.0", limiter.currentRate)
	}
}

func TestAdaptiveRateLimiter_Allow(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(10.0, 1.0, 20.0)

	// Should allow initial requests
	if !limiter.Allow() {
		t.Error("Expected first request to be allowed")
	}
}

func TestAdaptiveRateLimiter_RecordSuccess(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(5.0, 1.0, 20.0)

	for i := 0; i < 10; i++ {
		limiter.RecordSuccess()
	}

	// After many successes, rate should have increased
	rate := limiter.GetCurrentRate()
	if rate <= 5.0 {
		t.Errorf("Rate should increase after successes, got %f", rate)
	}
}

func TestAdaptiveRateLimiter_RecordRateLimited(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(10.0, 1.0, 20.0)

	limiter.RecordRateLimited()

	rate := limiter.GetCurrentRate()
	if rate >= 10.0 {
		t.Errorf("Rate should decrease after rate limiting, got %f", rate)
	}
}

func TestAdaptiveRateLimiter_RecordRateLimited_MinRate(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(2.0, 1.0, 20.0)

	// Multiple rate limits should not go below min rate
	for i := 0; i < 10; i++ {
		limiter.RecordRateLimited()
	}

	rate := limiter.GetCurrentRate()
	if rate < 1.0 {
		t.Errorf("Rate should not go below min rate (1.0), got %f", rate)
	}
}

func TestAdaptiveRateLimiter_MaxRate(t *testing.T) {
	limiter := NewAdaptiveRateLimiter(19.0, 1.0, 20.0)

	// Many successes should not exceed max rate
	for i := 0; i < 100; i++ {
		limiter.RecordSuccess()
	}

	rate := limiter.GetCurrentRate()
	if rate > 20.0 {
		t.Errorf("Rate should not exceed max rate (20.0), got %f", rate)
	}
}

func TestDefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()

	if config.DefaultInterval != 200 {
		t.Errorf("DefaultInterval = %d, want 200", config.DefaultInterval)
	}
	if config.MinInterval != 50 {
		t.Errorf("MinInterval = %d, want 50", config.MinInterval)
	}
	if config.MaxInterval != 5000 {
		t.Errorf("MaxInterval = %d, want 5000", config.MaxInterval)
	}
	if config.BackoffInitialMs != 1000 {
		t.Errorf("BackoffInitialMs = %d, want 1000", config.BackoffInitialMs)
	}
	if config.BackoffMaxMs != 60000 {
		t.Errorf("BackoffMaxMs = %d, want 60000", config.BackoffMaxMs)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", config.BackoffMultiplier)
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"rate limited", NewWhoisError(ErrRateLimited, "rate limited", nil), true},
		{"server connect failed", NewWhoisError(ErrServerConnectFailed, "connect failed", nil), true},
		{"parse error", NewWhoisError(ErrParseFailed, "parse failed", nil), false},
		{"generic error", fmt.Errorf("generic"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRateLimitError(tt.err)
			if got != tt.want {
				t.Errorf("isRateLimitError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateAvgLatency(t *testing.T) {
	tests := []struct {
		name    string
		history []int64
		want    int64
	}{
		{"empty", []int64{}, 0},
		{"single", []int64{100}, 100},
		{"multiple", []int64{100, 200, 300}, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateAvgLatency(tt.history)
			if got != tt.want {
				t.Errorf("calculateAvgLatency() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSmartScheduler_BackoffExponential(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.BackoffInitialMs = 1000
	config.BackoffMultiplier = 2.0
	scheduler := NewSmartScheduler(config)

	// First rate limit - backoff should be 1000ms
	scheduler.RecordResult("whois.example.com", 0, NewWhoisError(ErrRateLimited, "rate limited", nil))
	state := scheduler.GetServerState("whois.example.com")
	if state.CurrentBackoff != 1000 {
		t.Errorf("First backoff = %d, want 1000", state.CurrentBackoff)
	}

	// Second rate limit - backoff should be 2000ms
	scheduler.RecordResult("whois.example.com", 0, NewWhoisError(ErrRateLimited, "rate limited", nil))
	state = scheduler.GetServerState("whois.example.com")
	if state.CurrentBackoff != 2000 {
		t.Errorf("Second backoff = %d, want 2000", state.CurrentBackoff)
	}

	// Third rate limit - backoff should be 4000ms
	scheduler.RecordResult("whois.example.com", 0, NewWhoisError(ErrRateLimited, "rate limited", nil))
	state = scheduler.GetServerState("whois.example.com")
	if state.CurrentBackoff != 4000 {
		t.Errorf("Third backoff = %d, want 4000", state.CurrentBackoff)
	}
}

func TestSmartScheduler_BackoffMax(t *testing.T) {
	config := DefaultSchedulerConfig()
	config.BackoffInitialMs = 30000
	config.BackoffMaxMs = 60000
	config.BackoffMultiplier = 2.0
	scheduler := NewSmartScheduler(config)

	// Rate limit multiple times to exceed max
	for i := 0; i < 5; i++ {
		scheduler.RecordResult("whois.example.com", 0, NewWhoisError(ErrRateLimited, "rate limited", nil))
	}

	state := scheduler.GetServerState("whois.example.com")
	if state.CurrentBackoff > int64(config.BackoffMaxMs) {
		t.Errorf("Backoff = %d, should not exceed max %d", state.CurrentBackoff, config.BackoffMaxMs)
	}
}

func TestServerState_Fields(t *testing.T) {
	state := &ServerState{
		Server:               "whois.example.com",
		AvgLatency:           150,
		QueryCount:           100,
		FailureCount:         2,
		RateLimitedCount:     1,
		ConsecutiveFailures:  0,
		CurrentBackoff:       0,
		Healthy:              true,
		AdaptiveInterval:     200,
	}

	if state.Server != "whois.example.com" {
		t.Errorf("Server = %s, want whois.example.com", state.Server)
	}
	if !state.Healthy {
		t.Error("Server should be healthy")
	}
}

func TestSchedulerStats_Fields(t *testing.T) {
	stats := SchedulerStats{
		TotalScheduled:   100,
		TotalRateLimited: 5,
		TotalBackoffs:    3,
		TotalUnhealthy:   1,
		TotalAdaptations: 50,
	}

	if stats.TotalScheduled != 100 {
		t.Errorf("TotalScheduled = %d, want 100", stats.TotalScheduled)
	}
}

func BenchmarkCalculateAvgLatency(b *testing.B) {
	history := make([]int64, 10)
	for i := range history {
		history[i] = int64(i * 100)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateAvgLatency(history)
	}
}

func BenchmarkSmartScheduler_RecordResult(b *testing.B) {
	scheduler := NewSmartScheduler(DefaultSchedulerConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.RecordResult("whois.example.com", 100, nil)
	}
}
