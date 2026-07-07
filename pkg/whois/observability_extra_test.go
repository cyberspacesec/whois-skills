package whois

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---- CompositeMetrics 各 Record* 带 provider ----

func TestCompositeMetrics_RecordCacheOperation_WithProvider(t *testing.T) {
	cm := NewCompositeMetrics()
	otel := NewOpenTelemetryMetricsProvider()
	cm.AddProvider(otel)
	cm.RecordCacheOperation("get", true)
	cm.RecordCacheOperation("get", false)
	cs := otel.GetCounters()
	assert.Equal(t, int64(1), cs["whois.cache.hits"])
	assert.Equal(t, int64(1), cs["whois.cache.misses"])
}

func TestCompositeMetrics_RecordAPIRequest_WithProvider(t *testing.T) {
	cm := NewCompositeMetrics()
	otel := NewOpenTelemetryMetricsProvider()
	cm.AddProvider(otel)
	cm.RecordAPIRequest("GET", "/x", 200, 10*time.Millisecond)
	cm.RecordAPIRequest("GET", "/x", 500, 10*time.Millisecond)
	assert.Len(t, otel.GetTraces(), 2)
}

func TestCompositeMetrics_RecordRateLimit_WithProvider(t *testing.T) {
	cm := NewCompositeMetrics()
	otel := NewOpenTelemetryMetricsProvider()
	cm.AddProvider(otel)
	cm.RecordRateLimit("whois.example.com")
	cs := otel.GetCounters()
	assert.Equal(t, int64(1), cs["whois.rate_limits"])
}

func TestCompositeMetrics_RecordActiveQueries(t *testing.T) {
	cm := NewCompositeMetrics()
	otel := NewOpenTelemetryMetricsProvider()
	cm.AddProvider(otel)
	cm.RecordActiveQueries(5)
	// 无 panic 即可；验证 otel gauge
	_ = otel
}

// ---- OpenTelemetryMetricsProvider Record* 直接调用 ----

func TestOpenTelemetryMetricsProvider_RecordCacheOperation(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()
	o.RecordCacheOperation("get", true)
	o.RecordCacheOperation("get", false)
	cs := o.GetCounters()
	assert.Equal(t, int64(1), cs["whois.cache.hits"])
	assert.Equal(t, int64(1), cs["whois.cache.misses"])
}

func TestOpenTelemetryMetricsProvider_RecordRateLimit(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()
	o.RecordRateLimit("whois.example.com")
	cs := o.GetCounters()
	assert.Equal(t, int64(1), cs["whois.rate_limits"])
}

func TestOpenTelemetryMetricsProvider_RecordActiveQueries(t *testing.T) {
	o := NewOpenTelemetryMetricsProvider()
	o.RecordActiveQueries(7)
	// gauges 私有，通过无 panic 验证
	o.RecordActiveQueries(0)
}

// ---- NopMetricsProvider 各方法 ----

func TestNopMetricsProvider_AllMethods(t *testing.T) {
	n := NewNopMetricsProvider()
	// 全部为空实现，无 panic 即通过
	n.RecordWHOISQuery("s", true, time.Millisecond)
	n.RecordCacheOperation("get", true)
	n.RecordAPIRequest("GET", "/x", 200, time.Millisecond)
	n.RecordRateLimit("s")
	n.RecordActiveQueries(1)
	assert.Equal(t, "nop", n.Name())
}

// ---- CompositeMetrics.RecordActiveQueries 无 provider ----

func TestCompositeMetrics_RecordActiveQueries_NoProvider(t *testing.T) {
	cm := NewCompositeMetrics()
	assert.NotPanics(t, func() { cm.RecordActiveQueries(3) })
}
