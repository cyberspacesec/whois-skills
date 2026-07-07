package metrics

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// AlertLevel.String / getSlackColor 测试
// ============================================================

func TestAlertLevel_String(t *testing.T) {
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "CRITICAL", CriticalLevel.String())
	// 越界值走 default 分支
	assert.Equal(t, "UNKNOWN", AlertLevel(999).String())
}

func TestGetSlackColor(t *testing.T) {
	assert.Equal(t, "#36a64f", getSlackColor(InfoLevel))
	assert.Equal(t, "#ffcc00", getSlackColor(WarnLevel))
	assert.Equal(t, "#ff9900", getSlackColor(ErrorLevel))
	assert.Equal(t, "#ff0000", getSlackColor(CriticalLevel))
	assert.Equal(t, "#cccccc", getSlackColor(AlertLevel(999)))
}

// ============================================================
// getCurrentValue 测试
// ============================================================

func newCollectorWithValues(cpu, mem float64, apiTotal, apiFailed, whoisTotal, whoisFailed int64) *MetricsCollector {
	return &MetricsCollector{
		APIMetrics: APIMetrics{
			TotalRequests:  apiTotal,
			FailedRequests: apiFailed,
			StatusCodes:    make(map[int]int64),
			PathStats:      make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			TotalQueries:  whoisTotal,
			FailedQueries: whoisFailed,
			ServerStats:   make(map[string]*ServerStats),
		},
		SystemMetrics: SystemMetrics{
			CPUUsage:    cpu,
			MemoryUsage: mem,
		},
	}
}

func TestGetCurrentValue_CPU(t *testing.T) {
	mc := newCollectorWithValues(85.5, 0, 0, 0, 0, 0)
	rule := &AlertRule{Name: "high_cpu_usage"}
	assert.Equal(t, 85.5, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_Memory(t *testing.T) {
	mc := newCollectorWithValues(0, 92.3, 0, 0, 0, 0)
	rule := &AlertRule{Name: "high_memory_usage"}
	assert.Equal(t, 92.3, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_APIErrorRate_WithRequests(t *testing.T) {
	mc := newCollectorWithValues(0, 0, 100, 20, 0, 0)
	rule := &AlertRule{Name: "high_api_error_rate"}
	// 20/100*100 = 20
	assert.Equal(t, 20.0, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_APIErrorRate_NoRequests(t *testing.T) {
	mc := newCollectorWithValues(0, 0, 0, 5, 0, 0)
	rule := &AlertRule{Name: "high_api_error_rate"}
	assert.Equal(t, 0.0, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_WHOISFailureRate_WithQueries(t *testing.T) {
	mc := newCollectorWithValues(0, 0, 0, 0, 50, 15, )
	rule := &AlertRule{Name: "high_whois_failure_rate"}
	// 15/50*100 = 30
	assert.Equal(t, 30.0, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_WHOISFailureRate_NoQueries(t *testing.T) {
	mc := newCollectorWithValues(0, 0, 0, 0, 0, 5)
	rule := &AlertRule{Name: "high_whois_failure_rate"}
	assert.Equal(t, 0.0, getCurrentValue(rule, mc))
}

func TestGetCurrentValue_UnknownRule(t *testing.T) {
	mc := newCollectorWithValues(1, 2, 3, 4, 5, 6)
	rule := &AlertRule{Name: "unknown_rule"}
	assert.Equal(t, 0.0, getCurrentValue(rule, mc))
}

// ============================================================
// triggerAlert 测试（含通知通道、历史记录、消息模板）
// ============================================================

type recordingNotifier struct {
	mu       sync.Mutex
	received []*AlertEvent
	err      error
}

func (n *recordingNotifier) Notify(event *AlertEvent) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.received = append(n.received, event)
	return n.err
}

func (n *recordingNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.received)
}

func TestTriggerAlert_HappyPath(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}
	mock := &recordingNotifier{}
	am.RegisterNotifier("slack", mock)
	am.RegisterNotifier("email", mock)

	rule := &AlertRule{
		Name:            "high_cpu_usage",
		Level:           WarnLevel,
		Threshold:       80.0,
		MessageTemplate: "CPU使用率超过80%%，当前值: %.2f%%",
		NotifyChannels:  []string{"slack", "email", "missing-channel"},
		status: AlertStatus{
			IsAlerting: true,
			StartTime:  time.Now().Add(-6 * time.Minute),
		},
	}
	mc := newCollectorWithValues(85.5, 0, 0, 0, 0, 0)

	am.triggerAlert(rule, mc)

	// 历史记录应包含 1 条
	hist := am.GetHistory()
	assert.Len(t, hist, 1)
	assert.Equal(t, "high_cpu_usage", hist[0].RuleName)
	assert.Equal(t, WarnLevel, hist[0].Level)
	assert.Equal(t, 85.5, hist[0].CurrentValue)
	assert.Equal(t, 80.0, hist[0].Threshold)
	assert.Contains(t, hist[0].Message, "85.50")

	// 两个已注册通道都应收到通知；missing-channel 无 notifier 应被跳过（不 panic）
	assert.Equal(t, 2, mock.count())

	// lastAlertTime 应被更新
	assert.False(t, rule.lastAlertTime.IsZero())
}

func TestTriggerAlert_NotifierError(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}
	mock := &recordingNotifier{err: assertError("boom")}
	am.RegisterNotifier("slack", mock)

	rule := &AlertRule{
		Name:            "high_cpu_usage",
		Level:           ErrorLevel,
		Threshold:       80.0,
		MessageTemplate: "CPU %.2f",
		NotifyChannels:  []string{"slack"},
		status: AlertStatus{
			IsAlerting: true,
			StartTime:  time.Now().Add(-6 * time.Minute),
		},
	}
	mc := newCollectorWithValues(90.0, 0, 0, 0, 0, 0)

	// 不应 panic，仍应写入历史
	assert.NotPanics(t, func() {
		am.triggerAlert(rule, mc)
	})
	assert.Len(t, am.GetHistory(), 1)
	assert.Equal(t, 1, mock.count())
}

// assertError 是一个简单的 error 类型用于测试
type simpleErr string

func (e simpleErr) Error() string { return string(e) }

func assertError(msg string) error { return simpleErr(msg) }

// ============================================================
// addToHistory 截断测试
// ============================================================

func TestAddToHistory_Truncate(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 3,
		notifiers:      make(map[string]AlertNotifier),
	}

	for i := 0; i < 5; i++ {
		am.addToHistory(&AlertEvent{RuleName: "rule", Message: "m"})
	}

	hist := am.GetHistory()
	assert.Len(t, hist, 3, "history should be capped at maxHistorySize")
}

// ============================================================
// CheckRules 分支测试
// ============================================================

func TestAlertManager_CheckRules_IntervalNotReached(t *testing.T) {
	mc := newCollectorWithValues(90, 0, 0, 0, 0, 0)
	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:        "high_cpu_usage",
				Enabled:     true,
				Interval:    time.Hour,
				Threshold:   80.0,
				Condition:   func(mc *MetricsCollector) bool { return true },
				lastAlertTime: time.Now(), // 刚触发过，间隔未到
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	// 间隔未到，应跳过该规则，不进入告警状态
	assert.Len(t, am.GetHistory(), 0)
	assert.False(t, am.rules[0].status.IsAlerting)
}

func TestAlertManager_CheckRules_FirstAlert(t *testing.T) {
	mc := newCollectorWithValues(90, 0, 0, 0, 0, 0)
	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:            "high_cpu_usage",
				Enabled:         true,
				Interval:        0, // 立即触发
				Threshold:       80.0,
				Duration:        time.Hour,
				Condition:       func(mc *MetricsCollector) bool { return true },
				MessageTemplate: "CPU %.2f",
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	// 首次：进入告警状态但持续时间未到，不应触发 triggerAlert
	assert.True(t, am.rules[0].status.IsAlerting)
	assert.Len(t, am.GetHistory(), 0)
	assert.Equal(t, 90.0, am.rules[0].status.CurrentValue)
	assert.Equal(t, 80.0, am.rules[0].status.Threshold)
}

func TestAlertManager_CheckRules_DurationExceededDeadlocks(t *testing.T) {
	// 注意：CheckRules 持有 RLock，而 triggerAlert -> addToHistory 会尝试获取
	// 同一把锁的 Lock，二者来自同一 goroutine 会永久死锁（源码固有问题）。
	// 由于约束为"不改生产代码"，该"持续时间已到并触发告警"分支无法经由
	// CheckRules 安全覆盖。triggerAlert 的逻辑由 TestTriggerAlert_* 直接覆盖。
	// 这里仅验证：在不调用 CheckRules 的情况下，手动构造的持续超时状态
	// 可被直接调用 triggerAlert 处理（不死锁，因为不经过 CheckRules 的 RLock）。
	mc := newCollectorWithValues(90, 0, 0, 0, 0, 0)
	mock := &recordingNotifier{}
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      map[string]AlertNotifier{"slack": mock},
	}
	rule := &AlertRule{
		Name:            "high_cpu_usage",
		Enabled:         true,
		Interval:        0,
		Threshold:       80.0,
		Duration:        time.Millisecond,
		Condition:       func(mc *MetricsCollector) bool { return true },
		MessageTemplate: "CPU %.2f",
		NotifyChannels:  []string{"slack"},
		status: AlertStatus{
			IsAlerting: true,
			StartTime:  time.Now().Add(-time.Second),
		},
	}
	am.triggerAlert(rule, mc)
	assert.Len(t, am.GetHistory(), 1)
	assert.Equal(t, 1, mock.count())
}

func TestAlertManager_CheckRules_ConditionReset(t *testing.T) {
	mc := newCollectorWithValues(10, 0, 0, 0, 0, 0) // CPU 未超阈值
	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:      "high_cpu_usage",
				Enabled:   true,
				Interval:  0,
				Condition: func(mc *MetricsCollector) bool { return mc.SystemMetrics.CPUUsage > 80 },
				status: AlertStatus{
					IsAlerting: true,
					StartTime:  time.Now().Add(-time.Hour),
				},
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	// 条件不满足，应重置告警状态
	assert.False(t, am.rules[0].status.IsAlerting)
	assert.Len(t, am.GetHistory(), 0)
}

func TestAlertManager_CheckRules_AlertingButDurationNotReached(t *testing.T) {
	mc := newCollectorWithValues(90, 0, 0, 0, 0, 0)
	am := &AlertManager{
		rules: []*AlertRule{
			{
				Name:      "high_cpu_usage",
				Enabled:   true,
				Interval:  0,
				Duration:  time.Hour, // 很长，未到
				Condition: func(mc *MetricsCollector) bool { return true },
				status: AlertStatus{
					IsAlerting: true,
					StartTime:  time.Now(), // 刚开始
				},
			},
		},
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}

	am.CheckRules(mc)
	// 在告警状态但持续时间未到，不应触发
	assert.Len(t, am.GetHistory(), 0)
	assert.True(t, am.rules[0].status.IsAlerting)
}

// ============================================================
// registerDefaultRules 测试（通过 GetAlertManager 单例触发）
// ============================================================

func TestRegisterDefaultRules(t *testing.T) {
	// GetAlertManager 内部 once 会调用 registerDefaultRules
	am := GetAlertManager()
	assert.NotNil(t, am)

	names := make(map[string]bool)
	for _, r := range am.rules {
		names[r.Name] = true
	}
	assert.True(t, names["high_cpu_usage"])
	assert.True(t, names["high_memory_usage"])
	assert.True(t, names["high_api_error_rate"])
	assert.True(t, names["high_whois_failure_rate"])

	// 验证默认规则的 Condition 各分支
	cpuRule := findRule(am, "high_cpu_usage")
	assert.NotNil(t, cpuRule.Condition)
	assert.False(t, cpuRule.Condition(newCollectorWithValues(50, 0, 0, 0, 0, 0)))
	assert.True(t, cpuRule.Condition(newCollectorWithValues(81, 0, 0, 0, 0, 0)))

	memRule := findRule(am, "high_memory_usage")
	assert.False(t, memRule.Condition(newCollectorWithValues(0, 80, 0, 0, 0, 0)))
	assert.True(t, memRule.Condition(newCollectorWithValues(0, 91, 0, 0, 0, 0)))

	apiRule := findRule(am, "high_api_error_rate")
	// TotalRequests == 0 -> false
	assert.False(t, apiRule.Condition(newCollectorWithValues(0, 0, 0, 0, 0, 0)))
	// errorRate > 10
	assert.True(t, apiRule.Condition(newCollectorWithValues(0, 0, 100, 11, 0, 0)))
	// errorRate <= 10
	assert.False(t, apiRule.Condition(newCollectorWithValues(0, 0, 100, 5, 0, 0)))

	whoisRule := findRule(am, "high_whois_failure_rate")
	assert.False(t, whoisRule.Condition(newCollectorWithValues(0, 0, 0, 0, 0, 0)))
	assert.True(t, whoisRule.Condition(newCollectorWithValues(0, 0, 0, 0, 100, 21)))
	assert.False(t, whoisRule.Condition(newCollectorWithValues(0, 0, 0, 0, 100, 10)))
}

func findRule(am *AlertManager, name string) *AlertRule {
	for _, r := range am.rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// ============================================================
// StartAlertManager 测试（短间隔 + 不 panic）
// ============================================================

func TestStartAlertManager(t *testing.T) {
	// 注意：CheckRules 在 RLock 下写 rule.status，是源码固有的并发不安全点。
	// 多次 StartAlertManager 会启动多个后台 goroutine 同时写同一批 rules，
	// 在 -race 下必然报警。因此这里只启动一次，并用较长间隔确保 ticker
	// 在测试期间尽量不触发 CheckRules（即便触发也只有一个 goroutine，无竞争）。
	assert.NotPanics(t, func() {
		StartAlertManager(time.Hour)
	})
	// 给后台 goroutine 启动时间
	time.Sleep(50 * time.Millisecond)
}

// ============================================================
// ExportMetrics 写失败测试
// ============================================================

func TestMetricsCollector_ExportMetrics_WriteFail(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}
	// 写入不存在的目录
	err := mc.ExportMetrics("/nonexistent/dir/metrics.json")
	assert.Error(t, err)
}

// ============================================================
// StartMetricsCollection 测试（短间隔 + sleep）
// ============================================================

func TestMetricsCollector_StartMetricsCollection(t *testing.T) {
	mc := &MetricsCollector{
		APIMetrics: APIMetrics{
			StatusCodes: make(map[int]int64),
			PathStats:   make(map[string]*PathStats),
		},
		WHOISMetrics: WHOISMetrics{
			ServerStats: make(map[string]*ServerStats),
		},
	}
	// collectSystemMetrics 内部 cpu.Percent 会阻塞约 1 秒，需等待足够时间
	mc.StartMetricsCollection(50 * time.Millisecond)
	time.Sleep(1500 * time.Millisecond)
	// 通过加锁的 GetMetrics 读取，避免与后台 goroutine 的写竞争（-race 安全）
	metrics := mc.GetMetrics()
	sys, ok := metrics["system"].(map[string]interface{})
	if ok {
		// GoroutineCount 来自 runtime 必非零（若已被更新）
		if g, ok := sys["goroutine_count"].(int64); ok {
			assert.NotZero(t, g)
		}
	}
}

// ============================================================
// system.go 测试
// ============================================================

func TestCollectSystemMetrics(t *testing.T) {
	metrics := collectSystemMetrics()
	// GoroutineCount 来自 runtime.NumGoroutine()，必定 > 0
	assert.NotZero(t, metrics.GoroutineCount)
	// CPU/Memory/SystemLoad 依赖 gopsutil，离线环境可能为 0，只断言非负
	assert.GreaterOrEqual(t, metrics.CPUUsage, 0.0)
	assert.GreaterOrEqual(t, metrics.MemoryUsage, 0.0)
	assert.GreaterOrEqual(t, metrics.SystemLoad, 0.0)
}

func TestGetSystemInfo(t *testing.T) {
	info := GetSystemInfo()
	assert.NotNil(t, info)
	// runtime 键必定存在
	_, ok := info["runtime"]
	assert.True(t, ok, "runtime info should always be present")
	// 验证 runtime 子字段
	rt, ok := info["runtime"].(map[string]interface{})
	assert.True(t, ok)
	_, hasGoroutines := rt["goroutines"]
	assert.True(t, hasGoroutines)
	// goroutines 应大于 0
	g := rt["goroutines"].(int)
	assert.Greater(t, g, 0)
}

func TestStartSystemMetricsCollection(t *testing.T) {
	collector := GetCollector()
	assert.NotPanics(t, func() {
		StartSystemMetricsCollection(50 * time.Millisecond)
	})
	// collectSystemMetrics 内部 cpu.Percent 阻塞约 1 秒
	time.Sleep(1500 * time.Millisecond)
	// 通过加锁的 GetMetrics 读取，避免与后台 goroutine 的写竞争（-race 安全）
	metrics := collector.GetMetrics()
	if sys, ok := metrics["system"].(map[string]interface{}); ok {
		if g, ok := sys["goroutine_count"].(int64); ok {
			assert.NotZero(t, g)
		}
	}
}

// ============================================================
// notifier.go 测试
// ============================================================

func TestRegisterDefaultNotifiers(t *testing.T) {
	am := &AlertManager{
		rules:          make([]*AlertRule, 0),
		history:        make([]*AlertEvent, 0),
		maxHistorySize: 100,
		notifiers:      make(map[string]AlertNotifier),
	}
	am.RegisterDefaultNotifiers()
	assert.NotNil(t, am.notifiers["email"])
	assert.NotNil(t, am.notifiers["slack"])
	assert.NotNil(t, am.notifiers["webhook"])
}

func newAlertEvent() *AlertEvent {
	return &AlertEvent{
		RuleName:     "high_cpu_usage",
		Level:        WarnLevel,
		Message:      "CPU过高",
		Timestamp:    time.Now(),
		CurrentValue: 85.5,
		Threshold:    80.0,
		Duration:     5 * time.Minute,
	}
}

func TestSlackNotifier_Notify_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &SlackNotifier{WebhookURL: srv.URL, Channel: "#alerts"}
	err := n.Notify(newAlertEvent())
	assert.NoError(t, err)
}

func TestSlackNotifier_Notify_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := &SlackNotifier{WebhookURL: srv.URL}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Slack响应错误")
}

func TestSlackNotifier_Notify_BadURL(t *testing.T) {
	n := &SlackNotifier{WebhookURL: "http://127.0.0.1:0/bad-url"}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "发送Slack消息失败")
}

func TestWebhookNotifier_Notify_Happy(t *testing.T) {
	var gotMethod, gotCT string
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		body = make([]byte, r.ContentLength)
		r.Body.Read(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &WebhookNotifier{
		URL:    srv.URL,
		Method: "PUT",
		Headers: map[string]string{
			"X-Custom": "custom-val",
		},
	}
	err := n.Notify(newAlertEvent())
	assert.NoError(t, err)
	assert.Equal(t, "PUT", gotMethod)
	assert.Equal(t, "application/json", gotCT)

	var decoded map[string]interface{}
	assert.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "high_cpu_usage", decoded["rule_name"])
}

func TestWebhookNotifier_Notify_DefaultMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Method 为空，应默认 POST
	n := &WebhookNotifier{URL: srv.URL}
	err := n.Notify(newAlertEvent())
	assert.NoError(t, err)
	assert.Equal(t, http.MethodPost, gotMethod)
}

func TestWebhookNotifier_Notify_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	n := &WebhookNotifier{URL: srv.URL}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Webhook响应错误")
}

func TestWebhookNotifier_Notify_FormatMessageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &WebhookNotifier{
		URL: srv.URL,
		FormatMessage: func(event *AlertEvent) (interface{}, error) {
			return nil, assertError("format failed")
		},
	}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "格式化消息失败")
}

func TestWebhookNotifier_Notify_FormatMessagePayload(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = make([]byte, r.ContentLength)
		r.Body.Read(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &WebhookNotifier{
		URL: srv.URL,
		FormatMessage: func(event *AlertEvent) (interface{}, error) {
			return map[string]string{"custom": "payload", "rule": event.RuleName}, nil
		},
	}
	err := n.Notify(newAlertEvent())
	assert.NoError(t, err)

	var decoded map[string]string
	assert.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "payload", decoded["custom"])
	assert.Equal(t, "high_cpu_usage", decoded["rule"])
}

func TestWebhookNotifier_Notify_BadURL(t *testing.T) {
	n := &WebhookNotifier{URL: "http://127.0.0.1:0/bad"}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "发送Webhook请求失败")
}

func TestEmailNotifier_Notify_Error(t *testing.T) {
	// 指向一个不存在的 SMTP 地址，SendMail 必失败
	n := &EmailNotifier{
		Host:     "127.0.0.1",
		Port:     1, // 不可达端口
		Username: "u",
		Password: "p",
		From:     "alert@example.com",
		To:       []string{"admin@example.com"},
		CC:       []string{"ops@example.com"},
	}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "发送邮件失败")
}

func TestEmailNotifier_Notify_NoCC(t *testing.T) {
	// 无 CC 分支，仍应因 SMTP 不可达返回错误（覆盖 CC 为空路径）
	n := &EmailNotifier{
		Host: "127.0.0.1",
		Port: 1,
		From: "alert@example.com",
		To:   []string{"admin@example.com"},
	}
	err := n.Notify(newAlertEvent())
	assert.Error(t, err)
}

// ============================================================
// 一个用 mock SMTP server 覆盖 Email happy 路径的尝试
// （如果环境支持则通过，否则会被 SendMail 视为错误，仅作为辅助覆盖）
// ============================================================

func TestEmailNotifier_Notify_MockSMTP(t *testing.T) {
	// 起一个极简 TCP listener，模拟 SMTP 行为：接受连接后立即关闭
	// smtp.SendMail 会因无法完成握手而返回错误 —— 这覆盖了构建邮件内容/头的全部路径
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法启动 listener: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// 写入一个 SMTP greeting，让 SendMail 进入读取流程
			conn.Write([]byte("220 mock smtp\r\n"))
			// 简单回显后关闭，SendMail 会失败
			buf := make([]byte, 1024)
			conn.Read(buf)
			conn.Write([]byte("500 syntax error\r\n"))
			conn.Close()
		}
	}()

	n := &EmailNotifier{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "alert@example.com",
		Password: "pwd",
		From:     "alert@example.com",
		To:       []string{"admin@example.com"},
		CC:       []string{"ops@example.com"},
	}
	// 这里期望失败（mock SMTP 不完整），但已覆盖邮件构建逻辑
	_ = n.Notify(newAlertEvent())
}
