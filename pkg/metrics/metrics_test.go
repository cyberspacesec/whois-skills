package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// MetricsCollector 测试
// ============================================================

func TestGetCollector(t *testing.T) {
	collector := GetCollector()
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.APIMetrics.StatusCodes)
	assert.NotNil(t, collector.APIMetrics.PathStats)
	assert.NotNil(t, collector.WHOISMetrics.ServerStats)
}

func TestMetricsCollector_RecordAPIRequest(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	// 记录成功的API请求
	mc.RecordAPIRequest("/api/whois", 200, 100*time.Millisecond)

	assert.Equal(t, int64(1), mc.APIMetrics.TotalRequests)
	assert.Equal(t, int64(1), mc.APIMetrics.SuccessfulRequests)
	assert.Equal(t, int64(0), mc.APIMetrics.FailedRequests)
	assert.Equal(t, int64(100), mc.APIMetrics.AvgResponseTime)
	assert.Equal(t, int64(1), mc.APIMetrics.StatusCodes[200])

	// 验证路径统计
	pathStats := mc.APIMetrics.PathStats["/api/whois"]
	assert.NotNil(t, pathStats)
	assert.Equal(t, int64(1), pathStats.Requests)
}

func TestMetricsCollector_RecordAPIRequest_Failure(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	mc.RecordAPIRequest("/api/whois", 500, 50*time.Millisecond)

	assert.Equal(t, int64(1), mc.APIMetrics.TotalRequests)
	assert.Equal(t, int64(0), mc.APIMetrics.SuccessfulRequests)
	assert.Equal(t, int64(1), mc.APIMetrics.FailedRequests)
	assert.Equal(t, int64(1), mc.APIMetrics.StatusCodes[500])
}

func TestMetricsCollector_RecordWHOISQuery(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	mc.RecordWHOISQuery("whois.verisign-grs.com", true, 200*time.Millisecond)

	assert.Equal(t, int64(1), mc.WHOISMetrics.TotalQueries)
	assert.Equal(t, int64(1), mc.WHOISMetrics.SuccessfulQueries)
	assert.Equal(t, int64(0), mc.WHOISMetrics.FailedQueries)
	assert.Equal(t, int64(200), mc.WHOISMetrics.AvgQueryTime)

	serverStats := mc.WHOISMetrics.ServerStats["whois.verisign-grs.com"]
	assert.NotNil(t, serverStats)
	assert.Equal(t, int64(1), serverStats.Queries)
	assert.Equal(t, int64(1), serverStats.Successes)
}

func TestMetricsCollector_RecordWHOISQuery_Failure(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	mc.RecordWHOISQuery("whois.cnnic.cn", false, 300*time.Millisecond)

	assert.Equal(t, int64(1), mc.WHOISMetrics.TotalQueries)
	assert.Equal(t, int64(0), mc.WHOISMetrics.SuccessfulQueries)
	assert.Equal(t, int64(1), mc.WHOISMetrics.FailedQueries)

	serverStats := mc.WHOISMetrics.ServerStats["whois.cnnic.cn"]
	assert.NotNil(t, serverStats)
	assert.Equal(t, int64(1), serverStats.Failures)
}

func TestMetricsCollector_UpdateCacheMetrics(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	cacheMetrics := CacheMetrics{
		Hits:     100,
		Misses:   20,
		Entries:  80,
		HitRate:  83.3,
	}

	mc.UpdateCacheMetrics(cacheMetrics)
	assert.Equal(t, int64(100), mc.CacheMetrics.Hits)
	assert.Equal(t, int64(20), mc.CacheMetrics.Misses)
	assert.Equal(t, 83.3, mc.CacheMetrics.HitRate)
}

func TestMetricsCollector_UpdateSystemMetrics(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	sysMetrics := SystemMetrics{
		CPUUsage:       45.5,
		MemoryUsage:    60.2,
		GoroutineCount: 10,
	}

	mc.UpdateSystemMetrics(sysMetrics)
	assert.Equal(t, 45.5, mc.SystemMetrics.CPUUsage)
	assert.Equal(t, 60.2, mc.SystemMetrics.MemoryUsage)
	assert.Equal(t, int64(10), mc.SystemMetrics.GoroutineCount)
}

func TestMetricsCollector_GetMetrics(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			TotalRequests: 5,
			StatusCodes:   make(map[int]int64),
			PathStats:     make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			TotalQueries:  3,
			ServerStats:   make(map[string]*ServerStats),
		},
	}

	metrics := mc.GetMetrics()
	assert.NotNil(t, metrics)

	apiMetrics, ok := metrics["api"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, int64(5), apiMetrics["total_requests"])

	whoisMetrics, ok := metrics["whois"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, int64(3), whoisMetrics["total_queries"])
}

func TestMetricsCollector_ExportMetrics(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			TotalRequests: 1,
			StatusCodes:   make(map[int]int64),
			PathStats:     make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			TotalQueries: 1,
			ServerStats:  make(map[string]*ServerStats),
		},
	}

	// 创建临时目录
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "metrics.json")

	err := mc.ExportMetrics(filename)
	assert.NoError(t, err)

	// 验证文件存在且内容有效
	data, err := os.ReadFile(filename)
	assert.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	assert.NoError(t, err)
	assert.NotNil(t, result["api"])
}

func TestMetricsCollector_MultipleRequests(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	// 记录多个请求
	mc.RecordAPIRequest("/api/whois", 200, 100*time.Millisecond)
	mc.RecordAPIRequest("/api/whois", 200, 150*time.Millisecond)
	mc.RecordAPIRequest("/api/ip", 200, 50*time.Millisecond)
	mc.RecordAPIRequest("/api/whois", 500, 200*time.Millisecond)

	assert.Equal(t, int64(4), mc.APIMetrics.TotalRequests)
	assert.Equal(t, int64(3), mc.APIMetrics.SuccessfulRequests)
	assert.Equal(t, int64(1), mc.APIMetrics.FailedRequests)

	// 验证平均响应时间
	assert.True(t, mc.APIMetrics.AvgResponseTime > 0)
}

// ============================================================
// AlertManager 测试
// ============================================================

func TestGetAlertManager(t *testing.T) {
	manager := GetAlertManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.rules)
	assert.NotNil(t, manager.notifiers)
}

func TestAlertManager_AddRule(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	rule := &AlertRule{
		Name:        "test_rule",
		Description: "测试规则",
		Level:       WarnLevel,
		Threshold:   50.0,
		Enabled:     true,
	}

	am.AddRule(rule)
	assert.Len(t, am.rules, 1)
	assert.Equal(t, "test_rule", am.rules[0].Name)
}

func TestAlertManager_RemoveRule(t *testing.T) {
	am := &AlertManager{
		rules: []*AlertRule{
			{Name: "rule1", Enabled: true},
			{Name: "rule2", Enabled: true},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.RemoveRule("rule1")
	assert.Len(t, am.rules, 1)
	assert.Equal(t, "rule2", am.rules[0].Name)
}

func TestAlertManager_RemoveRule_NotFound(t *testing.T) {
	am := &AlertManager{
		rules: []*AlertRule{
			{Name: "rule1", Enabled: true},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.RemoveRule("nonexistent")
	assert.Len(t, am.rules, 1) // 不应改变
}

func TestAlertManager_RegisterNotifier(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	notifier := &mockNotifier{}
	am.RegisterNotifier("mock", notifier)
	assert.NotNil(t, am.notifiers["mock"])
}

func TestAlertManager_GetHistory(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	// 添加历史事件
	event1 := &AlertEvent{RuleName: "rule1", Level: WarnLevel, Message: "test1"}
	event2 := &AlertEvent{RuleName: "rule2", Level: ErrorLevel, Message: "test2"}
	am.addToHistory(event1)
	am.addToHistory(event2)

	history := am.GetHistory()
	assert.Len(t, history, 2)
	assert.Equal(t, "rule1", history[0].RuleName)
	assert.Equal(t, "rule2", history[1].RuleName)
}

func TestAlertManager_CheckRules_DisabledRule(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:      "disabled_rule",
				Enabled:   false,
				Condition: func(mc *MetricsCollector) bool { return true },
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	// 禁用规则不应产生任何历史记录
	assert.Len(t, am.GetHistory(), 0)
}

func TestAlertManager_CheckRules_ConditionNotMet(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}

	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:      "test_rule",
				Enabled:   true,
				Condition: func(mc *MetricsCollector) bool { return false },
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	assert.Len(t, am.GetHistory(), 0)
}

// mockNotifier 用于测试的模拟通知器
type mockNotifier struct{}

func (m *mockNotifier) Notify(event *AlertEvent) error {
	return nil
}
