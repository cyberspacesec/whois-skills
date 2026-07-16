package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
)

// runMetricsCmd 构造 metrics 命令树执行给定参数，返回 stdout 与 error。
func runMetricsCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newMetricsCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// recordSomeMetrics 向全局指标记录若干事件，使 stats 含数据并覆盖平均耗时分支。
func recordSomeMetrics(t *testing.T) {
	t.Helper()
	m := whois.GetGlobalMetrics()
	m.RecordWHOISQuery("whois.verisign-grs.com", true, 100*time.Millisecond)
	m.RecordWHOISQuery("whois.example.com", false, 50*time.Millisecond)
	m.RecordCacheOperation("get", true)
	m.RecordCacheOperation("get", false)
	m.RecordAPIRequest("GET", "/v1/whois/example.com", 200, 30*time.Millisecond)
	m.RecordRateLimit("whois.iana.org")
}

// --- newMetricsCmd ---

// TestMetricsCmd_NoSub 无子命令输出 Usage。
func TestMetricsCmd_NoSub(t *testing.T) {
	out, err := runMetricsCmd(t)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(out, "stats") || strings.Contains(out, "Usage"))
}

// --- newMetricsStatsCmd ---

// TestMetricsStats_Empty 无数据的文本输出（TotalQueries=0 不打印平均耗时）。
func TestMetricsStats_Empty(t *testing.T) {
	out, err := runMetricsCmd(t, "stats")
	assert.NoError(t, err)
	assert.Contains(t, out, "WHOIS 全局指标")
	assert.Contains(t, out, "总查询数")
	assert.NotContains(t, out, "平均查询耗时")
}

// TestMetricsStats_WithData 有数据的文本输出（覆盖平均耗时分支）。
func TestMetricsStats_WithData(t *testing.T) {
	recordSomeMetrics(t)
	out, err := runMetricsCmd(t, "stats")
	assert.NoError(t, err)
	assert.Contains(t, out, "平均查询耗时")
}

// TestMetricsStats_JSON --json 输出结构化指标。
func TestMetricsStats_JSON(t *testing.T) {
	recordSomeMetrics(t)
	out, err := runMetricsCmd(t, "stats", "--json")
	assert.NoError(t, err)
	assert.Contains(t, out, `"total_queries"`)
}

// --- newMetricsExportCmd ---

// TestMetricsExport_Empty 无数据导出 Prometheus 文本。
func TestMetricsExport_Empty(t *testing.T) {
	out, err := runMetricsCmd(t, "export")
	assert.NoError(t, err)
	assert.Contains(t, out, "# HELP whois_queries_total")
	assert.Contains(t, out, "# TYPE whois_queries_total counter")
	assert.Contains(t, out, "whois_queries_total")
}

// TestMetricsExport_WithData 有数据导出（值非零）。
func TestMetricsExport_WithData(t *testing.T) {
	recordSomeMetrics(t)
	out, err := runMetricsCmd(t, "export")
	assert.NoError(t, err)
	assert.Contains(t, out, "# HELP whois_queries_successful_total")
	assert.Contains(t, out, "whois_queries_successful_total")
	assert.Contains(t, out, "whois_rate_limit_events_total")
}
