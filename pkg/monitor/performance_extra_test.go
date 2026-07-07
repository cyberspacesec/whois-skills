package monitor

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================================
// percentile 边界测试
// ============================================================

func TestPercentile_P100(t *testing.T) {
	values := []int64{10, 20, 30, 40, 50}
	// p=100 -> index = (len-1)*100/100 = 4 -> 50
	got := percentile(values, 100)
	if got != 50 {
		t.Errorf("percentile(100) = %d, want 50", got)
	}
}

func TestPercentile_NegativeP(t *testing.T) {
	values := []int64{10, 20, 30, 40, 50}
	// p=-1：index = int(4 * -0.01) = int(-0.04) = 0；0 不 <0，走 sorted[0]=10
	got := percentile(values, -1)
	if got != 10 {
		t.Errorf("percentile(-1) = %d, want 10 (index 0)", got)
	}
}

func TestPercentile_NegativePClamped(t *testing.T) {
	// p=-100，len=5 -> index = int(4 * -1.0) = -4 < 0 -> clamp to 0 -> sorted[0]=10
	values := []int64{10, 20, 30, 40, 50}
	got := percentile(values, -100)
	if got != 10 {
		t.Errorf("percentile(-100) = %d, want 10 (clamped from negative index)", got)
	}
}

func TestPercentile_IndexClampedToLast(t *testing.T) {
	// 构造 index >= len 的情况：p 很大使得 index 超出
	// index = int((len-1) * p / 100)；当 len=1, p=100 -> index=0
	// 用 p=200：index = int(0 * 200/100) = 0，无法超出
	// 改用更大切片 + p>100
	values := []int64{5, 10, 15}
	got := percentile(values, 200)
	// index = int(2 * 2.0) = 4 >= len(3) -> clamp to 2 -> sorted[2]=15
	if got != 15 {
		t.Errorf("percentile(200) = %d, want 15 (clamped to last)", got)
	}
}

// ============================================================
// GetStats 分支测试
// ============================================================

func TestGetStats_Empty(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}
	stats := m.GetStats()
	// 空监控器：无延迟、无错误、无域名
	if stats.TotalQueries != 0 {
		t.Errorf("TotalQueries = %d, want 0", stats.TotalQueries)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", stats.SuccessRate)
	}
	if stats.AvgLatencyMs != 0 {
		t.Errorf("AvgLatencyMs = %f, want 0", stats.AvgLatencyMs)
	}
	if stats.P90LatencyMs != 0 || stats.P95LatencyMs != 0 || stats.P99LatencyMs != 0 {
		t.Errorf("Percentiles should be 0 for empty, got p90=%d p95=%d p99=%d",
			stats.P90LatencyMs, stats.P95LatencyMs, stats.P99LatencyMs)
	}
	if len(stats.RecentDomains) != 0 {
		t.Errorf("RecentDomains = %v, want empty", stats.RecentDomains)
	}
	if len(stats.RecentErrors) != 0 {
		t.Errorf("RecentErrors = %v, want empty", stats.RecentErrors)
	}
}

func TestGetStats_MoreThan10Domains(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	// 记录 15 个域名，每个查询时间递增，确保超过 10 个时进入替换分支
	for i := 0; i < 15; i++ {
		domain := "domain" + string(rune('a'+i)) + ".com"
		// 让后面记录的域名 LastQuery 更晚
		m.RecordQuery(domain, int64(100+i*10), true, false)
		time.Sleep(2 * time.Millisecond)
	}

	stats := m.GetStats()
	// RecentDomains 最多 10 个
	if len(stats.RecentDomains) > 10 {
		t.Errorf("RecentDomains count = %d, should be capped at 10", len(stats.RecentDomains))
	}
	if len(stats.RecentDomains) != 10 {
		t.Errorf("RecentDomains count = %d, want 10", len(stats.RecentDomains))
	}
	// 应包含最后查询的 10 个域名（a..o 中后 10 个，即 f..o）
	// 由于 map 迭代顺序不定，用集合判断
	domainSet := make(map[string]bool)
	for _, d := range stats.RecentDomains {
		domainSet[d] = true
	}
	// 最早查询的 domaina..domaine 不应在结果中
	for _, early := range []string{"domaina.com", "domainb.com", "domainc.com", "domaind.com", "domaine.com"} {
		if domainSet[early] {
			t.Errorf("early domain %s should not be in recent 10", early)
		}
	}
}

func TestGetStats_WithLatenciesAndErrors(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}

	// 记录多个延迟以触发 percentile 计算
	for i := 0; i < 10; i++ {
		m.RecordQuery("example.com", int64((i+1)*10), true, false)
	}
	// 记录错误
	m.RecordError("example.com", "boom1", false)
	m.RecordError("example.com", "boom2", true)

	stats := m.GetStats()

	// 百分位数应非零
	if stats.P90LatencyMs == 0 {
		t.Errorf("P90LatencyMs = 0, want non-zero")
	}
	if stats.P95LatencyMs == 0 {
		t.Errorf("P95LatencyMs = 0, want non-zero")
	}
	if stats.P99LatencyMs == 0 {
		t.Errorf("P99LatencyMs = 0, want non-zero")
	}
	// 平均延迟 = (10+20+...+100)/10 = 55
	if stats.AvgLatencyMs != 55.0 {
		t.Errorf("AvgLatencyMs = %f, want 55.0", stats.AvgLatencyMs)
	}
	// 最近错误最多 10 条，这里 2 条
	if len(stats.RecentErrors) != 2 {
		t.Errorf("RecentErrors count = %d, want 2", len(stats.RecentErrors))
	}
	// 成功率 100%
	if stats.SuccessRate != 100.0 {
		t.Errorf("SuccessRate = %f, want 100", stats.SuccessRate)
	}
}

func TestGetStats_RecentErrorsCappedAt10(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}
	// 记录 15 条错误，GetStats 最多返回 10 条
	for i := 0; i < 15; i++ {
		m.RecordError("example.com", "err", false)
	}
	stats := m.GetStats()
	if len(stats.RecentErrors) != 10 {
		t.Errorf("RecentErrors count = %d, want 10", len(stats.RecentErrors))
	}
}

func TestGetStats_DomainReplaceEarliestLogic(t *testing.T) {
	// 直接测试 >10 域名时"替换最早记录"的逻辑分支
	// 先填满 10 个，再逐个加入新域名触发替换
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}
	// 先记录 10 个"旧"域名
	for i := 0; i < 10; i++ {
		d := "old" + string(rune('a'+i)) + ".com"
		m.RecordQuery(d, 100, true, false)
		time.Sleep(1 * time.Millisecond)
	}
	// 再记录 1 个"新"域名，其 LastQuery 比 10 个旧域名中第一个更晚
	m.RecordQuery("new.com", 100, true, false)

	stats := m.GetStats()
	if len(stats.RecentDomains) != 10 {
		t.Errorf("RecentDomains count = %d, want 10", len(stats.RecentDomains))
	}
	// new.com 应在结果中
	found := false
	for _, d := range stats.RecentDomains {
		if d == "new.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("new.com should be in recent domains, got %v", stats.RecentDomains)
	}
}

// ============================================================
// LogStats / StartPerformanceLogging 测试（goroutine + sleep）
// ============================================================

func TestPerformanceMonitor_LogStats(t *testing.T) {
	m := &PerformanceMonitor{
		startTime:         time.Now(),
		domainStats:       make(map[string]*DomainStat),
		maxLatencyRecords: 1000,
		maxErrorRecords:   100,
	}
	m.RecordQuery("example.com", 100, true, false)

	// LogStats 是阻塞循环，在 goroutine 中运行一小段时间
	done := make(chan struct{})
	go func() {
		m.LogStats(20 * time.Millisecond)
	}()
	// 等 2 个 tick 后停止 goroutine（通过让函数继续运行直到测试结束）
	time.Sleep(60 * time.Millisecond)
	// 验证 stats 可被序列化（间接验证 LogStats 内部 json.Marshal 成功路径）
	stats := m.GetStats()
	_, err := json.Marshal(stats)
	if err != nil {
		t.Errorf("Marshal stats failed: %v", err)
	}
	close(done)
	// LogStats 无返回，goroutine 会泄漏但测试可结束
	_ = done
}

func TestStartPerformanceLogging(t *testing.T) {
	// 不应 panic
	StartPerformanceLogging(20 * time.Millisecond)
	// 让 goroutine 跑几个 tick
	time.Sleep(60 * time.Millisecond)
	// 先记录一些数据，让默认 monitor 有内容
	m := GetMonitor()
	m.RecordQuery("log-test.com", 50, true, false)
	time.Sleep(40 * time.Millisecond)
}

// ============================================================
// GetMonitor 单例（补充验证）
// ============================================================

func TestGetMonitor_Singleton(t *testing.T) {
	m1 := GetMonitor()
	m2 := GetMonitor()
	if m1 != m2 {
		t.Error("GetMonitor should return the same instance")
	}
}

// ============================================================
// WithPerformanceMonitoring 补充：返回值与错误透传
// ============================================================

func TestWithPerformanceMonitoring_NilResult(t *testing.T) {
	result, err := WithPerformanceMonitoring("empty.com", true, func() (interface{}, error) {
		return nil, nil
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("Result = %v, want nil", result)
	}
}

// ============================================================
// PerformanceStats JSON 序列化（覆盖字段标签）
// ============================================================

func TestPerformanceStats_JSONRoundTrip(t *testing.T) {
	stats := PerformanceStats{
		UptimeSeconds:     100,
		TotalQueries:      10,
		SuccessfulQueries: 9,
		FailedQueries:     1,
		ProxyQueries:      3,
		DirectQueries:     7,
		SuccessRate:       90.0,
		AvgLatencyMs:      55.5,
		P90LatencyMs:      80,
		P95LatencyMs:      90,
		P99LatencyMs:      100,
		RecentDomains:     []string{"a.com"},
		QueriesPerSecond:  0.1,
	}
	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var decoded PerformanceStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded.TotalQueries != 10 {
		t.Errorf("TotalQueries = %d, want 10", decoded.TotalQueries)
	}
	if decoded.P99LatencyMs != 100 {
		t.Errorf("P99LatencyMs = %d, want 100", decoded.P99LatencyMs)
	}
}
