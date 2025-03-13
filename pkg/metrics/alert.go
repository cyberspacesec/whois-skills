package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AlertLevel 告警级别
type AlertLevel int

const (
	// InfoLevel 信息级别
	InfoLevel AlertLevel = iota
	// WarnLevel 警告级别
	WarnLevel
	// ErrorLevel 错误级别
	ErrorLevel
	// CriticalLevel 严重级别
	CriticalLevel
)

// AlertRule 告警规则
type AlertRule struct {
	// 规则名称
	Name string

	// 规则描述
	Description string

	// 告警级别
	Level AlertLevel

	// 检查间隔
	Interval time.Duration

	// 阈值
	Threshold float64

	// 持续时间（超过该时间才触发告警）
	Duration time.Duration

	// 告警条件函数
	Condition func(mc *MetricsCollector) bool

	// 告警消息模板
	MessageTemplate string

	// 通知方式
	NotifyChannels []string

	// 是否启用
	Enabled bool

	// 上次告警时间
	lastAlertTime time.Time

	// 告警状态
	status AlertStatus
}

// AlertStatus 告警状态
type AlertStatus struct {
	// 是否处于告警状态
	IsAlerting bool

	// 开始时间
	StartTime time.Time

	// 持续时间
	Duration time.Duration

	// 当前值
	CurrentValue float64

	// 阈值
	Threshold float64
}

// AlertManager 告警管理器
type AlertManager struct {
	mu sync.RWMutex

	// 告警规则列表
	rules []*AlertRule

	// 告警历史
	history []*AlertEvent

	// 最大历史记录数
	maxHistorySize int

	// 通知处理器
	notifiers map[string]AlertNotifier
}

// AlertEvent 告警事件
type AlertEvent struct {
	// 规则名称
	RuleName string `json:"rule_name"`

	// 告警级别
	Level AlertLevel `json:"level"`

	// 告警消息
	Message string `json:"message"`

	// 触发时间
	Timestamp time.Time `json:"timestamp"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 持续时间
	Duration time.Duration `json:"duration"`
}

// AlertNotifier 告警通知接口
type AlertNotifier interface {
	// Notify 发送告警通知
	Notify(event *AlertEvent) error
}

var (
	defaultManager *AlertManager
	managerOnce    sync.Once
)

// GetAlertManager 获取告警管理器实例
func GetAlertManager() *AlertManager {
	managerOnce.Do(func() {
		defaultManager = &AlertManager{
			rules:          make([]*AlertRule, 0),
			history:        make([]*AlertEvent, 0),
			maxHistorySize: 1000,
			notifiers:      make(map[string]AlertNotifier),
		}
		defaultManager.registerDefaultRules()
	})
	return defaultManager
}

// registerDefaultRules 注册默认告警规则
func (am *AlertManager) registerDefaultRules() {
	// CPU使用率告警
	am.AddRule(&AlertRule{
		Name:        "high_cpu_usage",
		Description: "CPU使用率过高",
		Level:       WarnLevel,
		Interval:    time.Minute,
		Threshold:   80.0,
		Duration:    5 * time.Minute,
		Condition: func(mc *MetricsCollector) bool {
			return mc.SystemMetrics.CPUUsage > 80.0
		},
		MessageTemplate: "CPU使用率超过80%，当前值: %.2f%%",
		NotifyChannels:  []string{"email", "slack"},
		Enabled:         true,
	})

	// 内存使用率告警
	am.AddRule(&AlertRule{
		Name:        "high_memory_usage",
		Description: "内存使用率过高",
		Level:       WarnLevel,
		Interval:    time.Minute,
		Threshold:   90.0,
		Duration:    5 * time.Minute,
		Condition: func(mc *MetricsCollector) bool {
			return mc.SystemMetrics.MemoryUsage > 90.0
		},
		MessageTemplate: "内存使用率超过90%，当前值: %.2f%%",
		NotifyChannels:  []string{"email", "slack"},
		Enabled:         true,
	})

	// API错误率告警
	am.AddRule(&AlertRule{
		Name:        "high_api_error_rate",
		Description: "API错误率过高",
		Level:       ErrorLevel,
		Interval:    time.Minute,
		Threshold:   10.0,
		Duration:    5 * time.Minute,
		Condition: func(mc *MetricsCollector) bool {
			if mc.APIMetrics.TotalRequests == 0 {
				return false
			}
			errorRate := float64(mc.APIMetrics.FailedRequests) / float64(mc.APIMetrics.TotalRequests) * 100
			return errorRate > 10.0
		},
		MessageTemplate: "API错误率超过10%，当前值: %.2f%%",
		NotifyChannels:  []string{"email", "slack"},
		Enabled:         true,
	})

	// WHOIS查询失败率告警
	am.AddRule(&AlertRule{
		Name:        "high_whois_failure_rate",
		Description: "WHOIS查询失败率过高",
		Level:       ErrorLevel,
		Interval:    time.Minute,
		Threshold:   20.0,
		Duration:    5 * time.Minute,
		Condition: func(mc *MetricsCollector) bool {
			if mc.WHOISMetrics.TotalQueries == 0 {
				return false
			}
			failureRate := float64(mc.WHOISMetrics.FailedQueries) / float64(mc.WHOISMetrics.TotalQueries) * 100
			return failureRate > 20.0
		},
		MessageTemplate: "WHOIS查询失败率超过20%，当前值: %.2f%%",
		NotifyChannels:  []string{"email", "slack"},
		Enabled:         true,
	})
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule *AlertRule) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.rules = append(am.rules, rule)
}

// RemoveRule 移除告警规则
func (am *AlertManager) RemoveRule(name string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, rule := range am.rules {
		if rule.Name == name {
			am.rules = append(am.rules[:i], am.rules[i+1:]...)
			return
		}
	}
}

// RegisterNotifier 注册告警通知处理器
func (am *AlertManager) RegisterNotifier(name string, notifier AlertNotifier) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.notifiers[name] = notifier
}

// CheckRules 检查所有告警规则
func (am *AlertManager) CheckRules(mc *MetricsCollector) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for _, rule := range am.rules {
		if !rule.Enabled {
			continue
		}

		// 检查是否到达检查间隔
		if time.Since(rule.lastAlertTime) < rule.Interval {
			continue
		}

		// 检查告警条件
		if rule.Condition(mc) {
			// 如果已经在告警状态
			if rule.status.IsAlerting {
				// 检查持续时间
				if time.Since(rule.status.StartTime) >= rule.Duration {
					am.triggerAlert(rule, mc)
				}
			} else {
				// 开始告警状态
				rule.status = AlertStatus{
					IsAlerting:   true,
					StartTime:    time.Now(),
					CurrentValue: getCurrentValue(rule, mc),
					Threshold:    rule.Threshold,
				}
			}
		} else {
			// 重置告警状态
			rule.status.IsAlerting = false
		}
	}
}

// triggerAlert 触发告警
func (am *AlertManager) triggerAlert(rule *AlertRule, mc *MetricsCollector) {
	currentValue := getCurrentValue(rule, mc)
	message := fmt.Sprintf(rule.MessageTemplate, currentValue)

	event := &AlertEvent{
		RuleName:     rule.Name,
		Level:        rule.Level,
		Message:      message,
		Timestamp:    time.Now(),
		CurrentValue: currentValue,
		Threshold:    rule.Threshold,
		Duration:     time.Since(rule.status.StartTime),
	}

	// 添加到历史记录
	am.addToHistory(event)

	// 发送通知
	for _, channel := range rule.NotifyChannels {
		if notifier, ok := am.notifiers[channel]; ok {
			if err := notifier.Notify(event); err != nil {
				logrus.Errorf("发送告警通知失败 [%s]: %v", channel, err)
			}
		}
	}

	rule.lastAlertTime = time.Now()
}

// getCurrentValue 获取当前值
func getCurrentValue(rule *AlertRule, mc *MetricsCollector) float64 {
	switch rule.Name {
	case "high_cpu_usage":
		return mc.SystemMetrics.CPUUsage
	case "high_memory_usage":
		return mc.SystemMetrics.MemoryUsage
	case "high_api_error_rate":
		if mc.APIMetrics.TotalRequests == 0 {
			return 0
		}
		return float64(mc.APIMetrics.FailedRequests) / float64(mc.APIMetrics.TotalRequests) * 100
	case "high_whois_failure_rate":
		if mc.WHOISMetrics.TotalQueries == 0 {
			return 0
		}
		return float64(mc.WHOISMetrics.FailedQueries) / float64(mc.WHOISMetrics.TotalQueries) * 100
	default:
		return 0
	}
}

// addToHistory 添加告警事件到历史记录
func (am *AlertManager) addToHistory(event *AlertEvent) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 如果超过最大历史记录数，移除最旧的记录
	if len(am.history) >= am.maxHistorySize {
		am.history = am.history[1:]
	}

	am.history = append(am.history, event)
}

// GetHistory 获取告警历史记录
func (am *AlertManager) GetHistory() []*AlertEvent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// 返回副本以避免并发问题
	history := make([]*AlertEvent, len(am.history))
	copy(history, am.history)

	return history
}

// StartAlertManager 启动告警管理器
func StartAlertManager(checkInterval time.Duration) {
	manager := GetAlertManager()
	collector := GetCollector()

	ticker := time.NewTicker(checkInterval)
	go func() {
		for range ticker.C {
			manager.CheckRules(collector)
		}
	}()

	logrus.Infof("告警管理器已启动，检查间隔: %v", checkInterval)
}
