package whois

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewCompositeMetrics(t *testing.T) {
	cm := NewCompositeMetrics()
	if cm == nil {
		t.Fatal("NewCompositeMetrics() returned nil")
	}
}

func TestCompositeMetrics_WithProviders(t *testing.T) {
	prom := NewPrometheusMetricsProvider()
	otel := NewOpenTelemetryMetricsProvider()
	cm := NewCompositeMetrics(prom, otel)

	if len(cm.providers) != 2 {
		t.Errorf("Providers count = %d, want 2", len(cm.providers))
	}
}

func TestCompositeMetrics_AddProvider(t *testing.T) {
	cm := NewCompositeMetrics()
	prom := NewPrometheusMetricsProvider()
	cm.AddProvider(prom)

	if len(cm.providers) != 1 {
		t.Errorf("Providers count = %d, want 1", len(cm.providers))
	}
}

func TestCompositeMetrics_RecordWHOISQuery(t *testing.T) {
	cm := NewCompositeMetrics()

	cm.RecordWHOISQuery("whois.verisign-grs.com", true, 100*time.Millisecond)
	cm.RecordWHOISQuery("whois.verisign-grs.com", false, 500*time.Millisecond)

	stats := cm.GetBuiltInStats()
	if stats.TotalQueries != 2 {
		t.Errorf("TotalQueries = %d, want 2", stats.TotalQueries)
	}
	if stats.SuccessfulQueries != 1 {
		t.Errorf("SuccessfulQueries = %d, want 1", stats.SuccessfulQueries)
	}
	if stats.FailedQueries != 1 {
		t.Errorf("FailedQueries = %d, want 1", stats.FailedQueries)
	}
}

func TestCompositeMetrics_RecordCacheOperation(t *testing.T) {
	cm := NewCompositeMetrics()

	cm.RecordCacheOperation("get", true)
	cm.RecordCacheOperation("get", true)
	cm.RecordCacheOperation("get", false)

	stats := cm.GetBuiltInStats()
	if stats.CacheHits != 2 {
		t.Errorf("CacheHits = %d, want 2", stats.CacheHits)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("CacheMisses = %d, want 1", stats.CacheMisses)
	}
}

func TestCompositeMetrics_RecordAPIRequest(t *testing.T) {
	cm := NewCompositeMetrics()

	cm.RecordAPIRequest("GET", "/api/whois", 200, 50*time.Millisecond)
	cm.RecordAPIRequest("POST", "/api/query", 500, 200*time.Millisecond)

	stats := cm.GetBuiltInStats()
	if stats.APIRequests != 2 {
		t.Errorf("APIRequests = %d, want 2", stats.APIRequests)
	}
}

func TestCompositeMetrics_RecordRateLimit(t *testing.T) {
	cm := NewCompositeMetrics()

	cm.RecordRateLimit("whois.verisign-grs.com")

	stats := cm.GetBuiltInStats()
	if stats.RateLimitEvents != 1 {
		t.Errorf("RateLimitEvents = %d, want 1", stats.RateLimitEvents)
	}
}

func TestCompositeMetrics_Concurrent(t *testing.T) {
	cm := NewCompositeMetrics()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			cm.RecordWHOISQuery("server", true, 100*time.Millisecond)
			cm.RecordCacheOperation("get", true)
			cm.RecordAPIRequest("GET", "/test", 200, 10*time.Millisecond)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cm.GetBuiltInStats()
	if stats.TotalQueries != 10 {
		t.Errorf("TotalQueries = %d, want 10", stats.TotalQueries)
	}
	if stats.CacheHits != 10 {
		t.Errorf("CacheHits = %d, want 10", stats.CacheHits)
	}
	if stats.APIRequests != 10 {
		t.Errorf("APIRequests = %d, want 10", stats.APIRequests)
	}
}

// Prometheus provider tests

func TestPrometheusMetricsProvider(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	if p.Name() != "prometheus" {
		t.Errorf("Name = %s, want prometheus", p.Name())
	}
}

func TestPrometheusMetricsProvider_RecordWHOISQuery(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordWHOISQuery("whois.example.com", true, 150*time.Millisecond)
	p.RecordWHOISQuery("whois.example.com", false, 300*time.Millisecond)

	counters := p.GetCounters()
	if counters["whois_queries_total"] != 2 {
		t.Errorf("whois_queries_total = %d, want 2", counters["whois_queries_total"])
	}
	if counters["whois_queries_success"] != 1 {
		t.Errorf("whois_queries_success = %d, want 1", counters["whois_queries_success"])
	}
	if counters["whois_queries_failed"] != 1 {
		t.Errorf("whois_queries_failed = %d, want 1", counters["whois_queries_failed"])
	}

	histograms := p.GetHistograms()
	hist, ok := histograms["whois_query_duration_ms"]
	if !ok {
		t.Fatal("whois_query_duration_ms histogram should exist")
	}
	if hist.Count != 2 {
		t.Errorf("histogram count = %d, want 2", hist.Count)
	}
}

func TestPrometheusMetricsProvider_RecordCacheOperation(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordCacheOperation("get", true)
	p.RecordCacheOperation("get", false)

	counters := p.GetCounters()
	if counters["cache_hits_total"] != 1 {
		t.Errorf("cache_hits_total = %d, want 1", counters["cache_hits_total"])
	}
	if counters["cache_misses_total"] != 1 {
		t.Errorf("cache_misses_total = %d, want 1", counters["cache_misses_total"])
	}
}

func TestPrometheusMetricsProvider_RecordAPIRequest(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordAPIRequest("GET", "/api/whois", 200, 50*time.Millisecond)

	counters := p.GetCounters()
	if counters["api_requests_total"] != 1 {
		t.Errorf("api_requests_total = %d, want 1", counters["api_requests_total"])
	}
}

func TestPrometheusMetricsProvider_RecordRateLimit(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordRateLimit("whois.example.com")

	counters := p.GetCounters()
	if counters["rate_limits_total"] != 1 {
		t.Errorf("rate_limits_total = %d, want 1", counters["rate_limits_total"])
	}
}

func TestPrometheusMetricsProvider_RecordActiveQueries(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordActiveQueries(5)

	gauges := p.GetGauges()
	if gauges["active_queries"] != 5 {
		t.Errorf("active_queries = %d, want 5", gauges["active_queries"])
	}
}

func TestPrometheusMetricsProvider_ExportFormat(t *testing.T) {
	p := NewPrometheusMetricsProvider()

	p.RecordWHOISQuery("whois.example.com", true, 100*time.Millisecond)
	p.RecordCacheOperation("get", true)
	p.RecordActiveQueries(3)

	output := p.ExportPrometheusFormat()

	if !strings.Contains(output, "whois_queries_total") {
		t.Error("Prometheus output should contain whois_queries_total")
	}
	if !strings.Contains(output, "counter") {
		t.Error("Prometheus output should contain counter type")
	}
	if !strings.Contains(output, "gauge") {
		t.Error("Prometheus output should contain gauge type")
	}
	if !strings.Contains(output, "histogram") {
		t.Error("Prometheus output should contain histogram type")
	}
}

// OpenTelemetry provider tests

func TestOpenTelemetryMetricsProvider(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()

	if o.Name() != "opentelemetry" {
		t.Errorf("Name = %s, want opentelemetry", o.Name())
	}
}

func TestOpenTelemetryMetricsProvider_RecordWHOISQuery(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()

	o.RecordWHOISQuery("whois.example.com", true, 100*time.Millisecond)

	counters := o.GetCounters()
	if counters["whois.queries.total"] != 1 {
		t.Errorf("whois.queries.total = %d, want 1", counters["whois.queries.total"])
	}
}

func TestOpenTelemetryMetricsProvider_RecordAPIRequest(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()

	o.RecordAPIRequest("GET", "/api/whois", 200, 50*time.Millisecond)

	traces := o.GetTraces()
	if len(traces) != 1 {
		t.Errorf("Traces count = %d, want 1", len(traces))
	}
	if traces[0].Name != "GET /api/whois" {
		t.Errorf("Trace name = %s, want 'GET /api/whois'", traces[0].Name)
	}
	if traces[0].Status != "OK" {
		t.Errorf("Trace status = %s, want OK", traces[0].Status)
	}
}

func TestOpenTelemetryMetricsProvider_RecordAPIRequest_Error(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()

	o.RecordAPIRequest("GET", "/api/whois", 500, 50*time.Millisecond)

	traces := o.GetTraces()
	if len(traces) != 1 {
		t.Errorf("Traces count = %d, want 1", len(traces))
	}
	if traces[0].Status != "ERROR" {
		t.Errorf("Trace status = %s, want ERROR", traces[0].Status)
	}
}

func TestOpenTelemetryMetricsProvider_TraceLimit(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()

	// Add more than 100 traces
	for i := 0; i < 110; i++ {
		o.RecordAPIRequest("GET", "/test", 200, 10*time.Millisecond)
	}

	traces := o.GetTraces()
	if len(traces) > 100 {
		t.Errorf("Traces should be capped at 100, got %d", len(traces))
	}
}

// Nop provider tests

func TestNopMetricsProvider(t *testing.T) {
	nop := NewNopMetricsProvider()

	if nop.Name() != "nop" {
		t.Errorf("Name = %s, want nop", nop.Name())
	}

	// These should not panic
	nop.RecordWHOISQuery("server", true, time.Second)
	nop.RecordCacheOperation("get", true)
	nop.RecordAPIRequest("GET", "/test", 200, time.Second)
	nop.RecordRateLimit("server")
	nop.RecordActiveQueries(1)
}

// Global metrics tests

func TestGetGlobalMetrics(t *testing.T) {
	metrics := GetGlobalMetrics()
	if metrics == nil {
		t.Fatal("GetGlobalMetrics() returned nil")
	}
}

func TestInitMetricsWithProviders(t *testing.T) {
	// Reset the once
	globalMetricsOnce = sync.Once{}

	prom := NewPrometheusMetricsProvider()
	InitMetricsWithProviders(prom)

	metrics := GetGlobalMetrics()
	if len(metrics.providers) != 1 {
		t.Errorf("Providers count = %d, want 1", len(metrics.providers))
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Reset
	globalMetricsOnce = sync.Once{}

	RecordWHOISQuery("server", true, 100*time.Millisecond)
	RecordCacheOp("get", true)
	RecordAPIReq("GET", "/test", 200, 50*time.Millisecond)
	RecordRateLimitEvent("server")

	metrics := GetGlobalMetrics()
	stats := metrics.GetBuiltInStats()
	if stats.TotalQueries != 1 {
		t.Errorf("TotalQueries = %d, want 1", stats.TotalQueries)
	}
	if stats.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", stats.CacheHits)
	}
}

// Context tests

func TestContextWithMetrics(t *testing.T) {
	cm := NewCompositeMetrics()
	ctx := ContextWithMetrics(context.Background(), cm)

	retrieved := MetricsFromContext(ctx)
	if retrieved != cm {
		t.Error("Retrieved metrics should match injected metrics")
	}
}

func TestMetricsFromContext_Default(t *testing.T) {
	// Context without metrics should return global
	ctx := context.Background()
	metrics := MetricsFromContext(ctx)
	if metrics == nil {
		t.Error("Should return global metrics as default")
	}
}

func TestBuiltInStats_Fields(t *testing.T) {
	stats := BuiltInStats{
		TotalQueries:      100,
		SuccessfulQueries: 90,
		FailedQueries:     10,
		CacheHits:         50,
		CacheMisses:       30,
		APIRequests:       200,
		RateLimitEvents:   5,
		TotalQueryTimeMs:  15000,
	}

	if stats.TotalQueries != 100 {
		t.Errorf("TotalQueries = %d, want 100", stats.TotalQueries)
	}
}

func TestSimpleHistogram(t *testing.T) {
	hist := &SimpleHistogram{}

	// Simulate recording
	hist.Count = 3
	hist.Sum = 300
	hist.Min = 50
	hist.Max = 200
	hist.Avg = 100

	if hist.Count != 3 {
		t.Errorf("Count = %d, want 3", hist.Count)
	}
	if hist.Avg != 100 {
		t.Errorf("Avg = %f, want 100", hist.Avg)
	}
}

func TestOtelTrace_Fields(t *testing.T) {
	trace := &OtelTrace{
		TraceID: "test-trace-id",
		SpanID:  "test-span-id",
		Name:    "GET /api/whois",
		Kind:    "SERVER",
		Status:  "OK",
		Attributes: map[string]string{
			"http.method":      "GET",
			"http.status_code": "200",
		},
	}

	if trace.TraceID != "test-trace-id" {
		t.Errorf("TraceID = %s, want test-trace-id", trace.TraceID)
	}
	if trace.Kind != "SERVER" {
		t.Errorf("Kind = %s, want SERVER", trace.Kind)
	}
}

func TestMetricsProvider_Interface(t *testing.T) {
	// Verify both providers implement the interface
	var _ MetricsProvider = NewPrometheusMetricsProvider()
	var _ MetricsProvider = NewOpenTelemetryMetricsProvider()
	var _ MetricsProvider = NewNopMetricsProvider()
}

func TestCompositeMetrics_MultipleProviders(t *testing.T) {
	prom := NewPrometheusMetricsProvider()
	otel := NewOpenTelemetryMetricsProvider()
	cm := NewCompositeMetrics(prom, otel)

	cm.RecordWHOISQuery("server", true, 100*time.Millisecond)

	// Both providers should have the data
	promCounters := prom.GetCounters()
	if promCounters["whois_queries_total"] != 1 {
		t.Errorf("Prometheus whois_queries_total = %d, want 1", promCounters["whois_queries_total"])
	}

	otelCounters := otel.GetCounters()
	if otelCounters["whois.queries.total"] != 1 {
		t.Errorf("OpenTelemetry whois.queries.total = %d, want 1", otelCounters["whois.queries.total"])
	}
}

func BenchmarkCompositeMetrics_RecordWHOISQuery(b *testing.B) {
	cm := NewCompositeMetrics()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.RecordWHOISQuery("server", true, 100*time.Millisecond)
	}
}

func BenchmarkPrometheusMetricsProvider_RecordWHOISQuery(b *testing.B) {
	p := NewPrometheusMetricsProvider()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.RecordWHOISQuery("server", true, 100*time.Millisecond)
	}
}
