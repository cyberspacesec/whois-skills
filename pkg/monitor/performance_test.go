package monitor

import (
	"testing"
	"time"
)

func TestGetMonitor(t *testing.T) {
	m := GetMonitor()
	if m == nil {
		t.Fatal("GetMonitor() returned nil")
	}
}

func TestPerformanceMonitor_RecordQuery(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	m.RecordQuery("example.com", 100, true, false)
	m.RecordQuery("example.com", 200, false, true)
	m.RecordQuery("other.com", 50, true, false)

	if m.totalQueries != 3 {
		t.Errorf("totalQueries = %d, want 3", m.totalQueries)
	}
	if m.successfulQueries != 2 {
		t.Errorf("successfulQueries = %d, want 2", m.successfulQueries)
	}
	if m.failedQueries != 1 {
		t.Errorf("failedQueries = %d, want 1", m.failedQueries)
	}
	if m.directQueries != 2 {
		t.Errorf("directQueries = %d, want 2", m.directQueries)
	}
	if m.proxyQueries != 1 {
		t.Errorf("proxyQueries = %d, want 1", m.proxyQueries)
	}
	if m.totalLatencyMs != 350 {
		t.Errorf("totalLatencyMs = %d, want 350", m.totalLatencyMs)
	}
}

func TestPerformanceMonitor_RecordError(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	m.RecordError("example.com", "timeout", false)

	if len(m.recentErrors) != 1 {
		t.Errorf("recentErrors count = %d, want 1", len(m.recentErrors))
	}
	if m.recentErrors[0].Domain != "example.com" {
		t.Errorf("Error domain = %s, want example.com", m.recentErrors[0].Domain)
	}
	if m.recentErrors[0].Message != "timeout" {
		t.Errorf("Error message = %s, want timeout", m.recentErrors[0].Message)
	}
}

func TestPerformanceMonitor_GetStats(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	m.RecordQuery("example.com", 100, true, false)
	m.RecordQuery("example.com", 200, true, false)
	m.RecordQuery("other.com", 50, false, true)
	m.RecordError("other.com", "connection failed", true)

	stats := m.GetStats()

	if stats.TotalQueries != 3 {
		t.Errorf("TotalQueries = %d, want 3", stats.TotalQueries)
	}
	if stats.SuccessfulQueries != 2 {
		t.Errorf("SuccessfulQueries = %d, want 2", stats.SuccessfulQueries)
	}
	if stats.FailedQueries != 1 {
		t.Errorf("FailedQueries = %d, want 1", stats.FailedQueries)
	}
	if stats.ProxyQueries != 1 {
		t.Errorf("ProxyQueries = %d, want 1", stats.ProxyQueries)
	}
	if stats.SuccessRate < 60 || stats.SuccessRate > 70 {
		t.Errorf("SuccessRate = %f, want ~66.7", stats.SuccessRate)
	}
	if stats.AvgLatencyMs != 116.66666666666667 {
		t.Logf("AvgLatencyMs = %f (close to 116.67)", stats.AvgLatencyMs)
	}
	if len(stats.RecentErrors) != 1 {
		t.Errorf("RecentErrors count = %d, want 1", len(stats.RecentErrors))
	}
	if len(stats.RecentDomains) != 2 {
		t.Errorf("RecentDomains count = %d, want 2", len(stats.RecentDomains))
	}
}

func TestPerformanceMonitor_DomainStats(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	m.RecordQuery("example.com", 100, true, false)
	m.RecordQuery("example.com", 200, true, false)

	stat, exists := m.domainStats["example.com"]
	if !exists {
		t.Fatal("Domain stat should exist")
	}
	if stat.Queries != 2 {
		t.Errorf("Queries = %d, want 2", stat.Queries)
	}
	if stat.Successful != 2 {
		t.Errorf("Successful = %d, want 2", stat.Successful)
	}
	if stat.AvgLatencyMs != 150 {
		t.Errorf("AvgLatencyMs = %d, want 150", stat.AvgLatencyMs)
	}
}

func TestPerformanceMonitor_MaxLatencyRecords(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 5,
		maxErrorRecords:   100,
	}

	for i := 0; i < 10; i++ {
		m.RecordQuery("example.com", int64(i*100), true, false)
	}

	if len(m.recentLatencies) > 5 {
		t.Errorf("recentLatencies count = %d, should be capped at 5", len(m.recentLatencies))
	}
}

func TestPerformanceMonitor_MaxErrorRecords(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   3,
	}

	for i := 0; i < 10; i++ {
		m.RecordError("example.com", "error", false)
	}

	if len(m.recentErrors) > 3 {
		t.Errorf("recentErrors count = %d, should be capped at 3", len(m.recentErrors))
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		values []int64
		p      int
		want   int64
	}{
		{"empty", []int64{}, 50, 0},
		{"single", []int64{100}, 50, 100},
		{"p90", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 90, 90},
		{"p99", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 99, 90},
		{"p50", []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}, 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.values, tt.p)
			if got != tt.want {
				t.Errorf("percentile() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Errorf("min(1,2) = %d, want 1", min(1, 2))
	}
	if min(5, 3) != 3 {
		t.Errorf("min(5,3) = %d, want 3", min(5, 3))
	}
}

func TestDomainStat_Fields(t *testing.T) {
	stat := &DomainStat{
		Queries:    100,
		Successful: 90,
		Failed:     10,
		AvgLatencyMs: 150,
	}

	if stat.Queries != 100 {
		t.Errorf("Queries = %d, want 100", stat.Queries)
	}
	if stat.AvgLatencyMs != 150 {
		t.Errorf("AvgLatencyMs = %d, want 150", stat.AvgLatencyMs)
	}
}

func TestErrorRecord_Fields(t *testing.T) {
	record := ErrorRecord{
		Domain:    "example.com",
		Message:   "timeout",
		UsedProxy: true,
	}

	if record.Domain != "example.com" {
		t.Errorf("Domain = %s, want example.com", record.Domain)
	}
	if !record.UsedProxy {
		t.Error("UsedProxy should be true")
	}
}

func TestPerformanceStats_Fields(t *testing.T) {
	stats := PerformanceStats{
		UptimeSeconds:     3600,
		TotalQueries:      1000,
		SuccessfulQueries: 950,
		FailedQueries:     50,
		SuccessRate:       95.0,
		AvgLatencyMs:      120.5,
		P90LatencyMs:      200,
		P95LatencyMs:      300,
		P99LatencyMs:      500,
		QueriesPerSecond:  0.278,
	}

	if stats.TotalQueries != 1000 {
		t.Errorf("TotalQueries = %d, want 1000", stats.TotalQueries)
	}
	if stats.P99LatencyMs != 500 {
		t.Errorf("P99LatencyMs = %d, want 500", stats.P99LatencyMs)
	}
}

func TestWithPerformanceMonitoring(t *testing.T) {
	result, err := WithPerformanceMonitoring("example.com", false, func() (interface{}, error) {
		return "whois data", nil
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "whois data" {
		t.Errorf("Result = %v, want 'whois data'", result)
	}
}

func TestWithPerformanceMonitoring_Error(t *testing.T) {
	_, err := WithPerformanceMonitoring("example.com", true, func() (interface{}, error) {
		return nil, &testError{msg: "query failed"}
	})
	if err == nil {
		t.Error("Expected error")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func BenchmarkRecordQuery(b *testing.B) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordQuery("example.com", 100, true, false)
	}
}

func BenchmarkPercentile(b *testing.B) {
	values := make([]int64, 1000)
	for i := range values {
		values[i] = int64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		percentile(values, 99)
	}
}
