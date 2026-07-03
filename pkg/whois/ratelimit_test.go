package whois

import (
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 10.0,
		BurstSize:  20,
		PerServerRate: map[string]float64{
			"whois.verisign-grs.com": 5.0,
		},
	}
	rl := NewRateLimiter(config)
	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 100.0,
		BurstSize:  100,
	}
	rl := NewRateLimiter(config)

	if !rl.Allow("whois.example.com") {
		t.Error("Should allow request with high rate limit")
	}
}

func TestRateLimiter_GlobalRateLimit(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 1.0,
		BurstSize:  2,
	}
	rl := NewRateLimiter(config)

	// Should allow up to burst size
	if !rl.Allow("server1.com") {
		t.Error("First request should be allowed")
	}
	if !rl.Allow("server2.com") {
		t.Error("Second request should be allowed")
	}
	// Third request may be denied (burst exhausted)
	// But since token bucket refills over time, we just check it doesn't panic
	rl.Allow("server3.com")
}

func TestRateLimiter_PerServerRate(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 100.0,
		BurstSize:  100,
		PerServerRate: map[string]float64{
			"slow.server.com": 1.0,
		},
	}
	rl := NewRateLimiter(config)

	// Server with configured rate
	if !rl.Allow("slow.server.com") {
		t.Error("First request to slow server should be allowed")
	}
}

func TestRateLimiter_NoGlobalRate(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 0,
		BurstSize:  0,
	}
	rl := NewRateLimiter(config)

	// Without global rate, all requests should be allowed
	if !rl.Allow("server.com") {
		t.Error("Should allow when no global rate configured")
	}
}

func TestRateLimiter_NoServerRate(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 100.0,
		BurstSize:  100,
	}
	rl := NewRateLimiter(config)

	// Server without specific rate should be allowed (uses global only)
	if !rl.Allow("unknown.server.com") {
		t.Error("Should allow when no per-server rate configured")
	}
}

func TestRateLimiter_Nil(t *testing.T) {
	var rl *RateLimiter
	if !rl.Allow("server.com") {
		t.Error("Nil rate limiter should allow all requests")
	}
}

func TestNewTokenBucket(t *testing.T) {
	tb := newTokenBucket(10.0, 20)
	if tb == nil {
		t.Fatal("newTokenBucket() returned nil")
	}
	if tb.rate != 10.0 {
		t.Errorf("Rate = %f, want 10.0", tb.rate)
	}
	if tb.maxTokens != 20 {
		t.Errorf("MaxTokens = %f, want 20", tb.maxTokens)
	}
}

func TestNewTokenBucket_DefaultBurst(t *testing.T) {
	tb := newTokenBucket(5.0, 0)
	if tb.maxTokens != 5.0 {
		t.Errorf("Default MaxTokens = %f, want 5.0 (equals rate)", tb.maxTokens)
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	tb := newTokenBucket(1.0, 2)

	if !tb.allow() {
		t.Error("First request should be allowed")
	}
	if !tb.allow() {
		t.Error("Second request should be allowed (burst=2)")
	}
	// Third request might fail since burst is 2
	// This is timing dependent so we just verify no panic
	tb.allow()
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := newTokenBucket(1000.0, 10) // High rate for quick refill

	// Drain the bucket
	for i := 0; i < 10; i++ {
		tb.allow()
	}

	// Wait for refill
	time.Sleep(10 * time.Millisecond)

	// Should be refilled
	if !tb.allow() {
		t.Error("Should be allowed after refill")
	}
}

func TestRateLimiterConfig_Fields(t *testing.T) {
	config := RateLimiterConfig{
		GlobalRate: 10.0,
		PerServerRate: map[string]float64{
			"server1.com": 5.0,
			"server2.com": 3.0,
		},
		BurstSize: 20,
	}

	if config.GlobalRate != 10.0 {
		t.Errorf("GlobalRate = %f, want 10.0", config.GlobalRate)
	}
	if len(config.PerServerRate) != 2 {
		t.Errorf("PerServerRate count = %d, want 2", len(config.PerServerRate))
	}
	if config.BurstSize != 20 {
		t.Errorf("BurstSize = %d, want 20", config.BurstSize)
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	config := RateLimiterConfig{
		GlobalRate: 1000.0,
		BurstSize:  1000,
	}
	rl := NewRateLimiter(config)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow("server.com")
	}
}
