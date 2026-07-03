package whois

import (
	"context"
	"fmt"
	"sync"
	"time"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// DomainMonitor 域名监控器
// 用于监控域名到期、状态变更、注册人变更等
type DomainMonitor struct {
	mu sync.RWMutex

	// 监控配置
	config MonitorConfig

	// 域名监控状态
	watchlist map[string]*DomainWatchState

	// 告警通道
	alertChan chan *DomainAlert

	// 取消函数
	cancel context.CancelFunc

	// 告警回调
	alertCallback func(alert *DomainAlert)
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	// 检查间隔（分钟）
	CheckInterval int `json:"check_interval_minutes"`

	// 到期预警天数
	ExpiryWarningDays int `json:"expiry_warning_days"`

	// 到期紧急天数
	ExpiryCriticalDays int `json:"expiry_critical_days"`

	// 是否监控状态变更
	WatchStatusChange bool `json:"watch_status_change"`

	// 是否监控注册人变更
	WatchRegistrantChange bool `json:"watch_registrant_change"`

	// 是否监控NS变更
	WatchNSChange bool `json:"watch_ns_change"`

	// 是否监控DNS服务器变更
	WatchDNSChange bool `json:"watch_dns_change"`

	// 查询超时（秒）
	QueryTimeout int `json:"query_timeout_seconds"`

	// 最大并发检查数
	MaxConcurrentChecks int `json:"max_concurrent_checks"`
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		CheckInterval:         60,    // 每小时检查一次
		ExpiryWarningDays:     30,    // 30天预警
		ExpiryCriticalDays:    7,     // 7天紧急
		WatchStatusChange:     true,  // 监控状态变更
		WatchRegistrantChange: true,  // 监控注册人变更
		WatchNSChange:         true,  // 监控NS变更
		WatchDNSChange:        true,  // 监控DNS变更
		QueryTimeout:          10,    // 10秒超时
		MaxConcurrentChecks:   5,     // 最大5个并发
	}
}

// DomainWatchState 域名监控状态
type DomainWatchState struct {
	// 域名
	Domain string `json:"domain"`

	// 上次检查时间
	LastCheck time.Time `json:"last_check"`

	// 上次WHOIS信息
	LastInfo *whoisparser.WhoisInfo `json:"last_info,omitempty"`

	// 上次原始响应
	LastRaw string `json:"-"`

	// 当前状态
	Status WatchStatus `json:"status"`

	// 到期时间
	ExpirationDate string `json:"expiration_date,omitempty"`

	// 剩余天数
	DaysRemaining int `json:"days_remaining,omitempty"`

	// 已触发的告警数量
	AlertCount int `json:"alert_count"`

	// 添加时间
	AddedAt time.Time `json:"added_at"`
}

// WatchStatus 监控状态
type WatchStatus string

const (
	WatchStatusActive  WatchStatus = "active"  // 正常
	WatchStatusWarning WatchStatus = "warning"  // 即将到期
	WatchStatusCritical WatchStatus = "critical" // 紧急到期
	WatchStatusExpired WatchStatus = "expired"  // 已过期
	WatchStatusError   WatchStatus = "error"    // 查询错误
	WatchStatusChanged WatchStatus = "changed"  // 发生变更
)

// DomainAlert 域名告警
type DomainAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 域名
	Domain string `json:"domain"`

	// 告警类型
	Type AlertType `json:"type"`

	// 告警级别
	Level AlertLevel `json:"level"`

	// 告警消息
	Message string `json:"message"`

	// 变更前值
	OldValue string `json:"old_value,omitempty"`

	// 变更后值
	NewValue string `json:"new_value,omitempty"`

	// 触发时间
	Timestamp time.Time `json:"timestamp"`

	// 建议操作
	Action string `json:"action,omitempty"`
}

// AlertType 告警类型
type AlertType string

const (
	// AlertExpiryWarning 到期预警
	AlertExpiryWarning AlertType = "expiry_warning"

	// AlertExpiryCritical 到期紧急
	AlertExpiryCritical AlertType = "expiry_critical"

	// AlertExpiryPassed 已过期
	AlertExpiryPassed AlertType = "expiry_passed"

	// AlertStatusChange 域名状态变更
	AlertStatusChange AlertType = "status_change"

	// AlertRegistrantChange 注册人变更
	AlertRegistrantChange AlertType = "registrant_change"

	// AlertNSChange NS变更
	AlertNSChange AlertType = "ns_change"

	// AlertDNSChange DNS服务器变更
	AlertDNSChange AlertType = "dns_change"

	// AlertQueryError 查询错误
	AlertQueryError AlertType = "query_error"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// NewDomainMonitor 创建新的域名监控器
func NewDomainMonitor(config MonitorConfig) *DomainMonitor {
	if config.CheckInterval <= 0 {
		config.CheckInterval = 60
	}
	if config.ExpiryWarningDays <= 0 {
		config.ExpiryWarningDays = 30
	}
	if config.ExpiryCriticalDays <= 0 {
		config.ExpiryCriticalDays = 7
	}
	if config.MaxConcurrentChecks <= 0 {
		config.MaxConcurrentChecks = 5
	}

	return &DomainMonitor{
		config:    config,
		watchlist: make(map[string]*DomainWatchState),
		alertChan: make(chan *DomainAlert, 100),
	}
}

// AddWatch 添加域名到监控列表
func (m *DomainMonitor) AddWatch(domain string, initialInfo *whoisparser.WhoisInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := &DomainWatchState{
		Domain:   domain,
		Status:   WatchStatusActive,
		LastInfo: initialInfo,
		AddedAt:  time.Now(),
	}

	// 设置到期信息
	if initialInfo != nil && initialInfo.Domain != nil {
		state.ExpirationDate = initialInfo.Domain.ExpirationDate
		state.DaysRemaining = calculateDaysRemaining(initialInfo.Domain.ExpirationDate)
		state.Status = m.determineWatchStatus(state.DaysRemaining)
	}

	m.watchlist[domain] = state
}

// RemoveWatch 移除域名监控
func (m *DomainMonitor) RemoveWatch(domain string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watchlist, domain)
}

// GetWatchList 获取当前监控列表
func (m *DomainMonitor) GetWatchList() []DomainWatchState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []DomainWatchState
	for _, state := range m.watchlist {
		list = append(list, *state)
	}
	return list
}

// GetWatchState 获取指定域名的监控状态
func (m *DomainMonitor) GetWatchState(domain string) *DomainWatchState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state, ok := m.watchlist[domain]; ok {
		result := *state
		return &result
	}
	return nil
}

// Alerts 获取告警通道
func (m *DomainMonitor) Alerts() <-chan *DomainAlert {
	return m.alertChan
}

// OnAlert 设置告警回调
func (m *DomainMonitor) OnAlert(callback func(alert *DomainAlert)) {
	m.alertCallback = callback
}

// Start 启动监控
func (m *DomainMonitor) Start(ctx context.Context) {
	ctx, m.cancel = context.WithCancel(ctx)

	ticker := time.NewTicker(time.Duration(m.config.CheckInterval) * time.Minute)
	defer ticker.Stop()

	// 首次立即检查
	m.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

// Stop 停止监控
func (m *DomainMonitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// checkAll 检查所有监控域名
func (m *DomainMonitor) checkAll(ctx context.Context) {
	m.mu.RLock()
	domains := make([]string, 0, len(m.watchlist))
	for d := range m.watchlist {
		domains = append(domains, d)
	}
	m.mu.RUnlock()

	if len(domains) == 0 {
		return
	}

	sem := make(chan struct{}, m.config.MaxConcurrentChecks)
	var wg sync.WaitGroup

	for _, domain := range domains {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m.checkDomain(ctx, d)
		}(domain)
	}

	wg.Wait()
}

// checkDomain 检查单个域名
func (m *DomainMonitor) checkDomain(ctx context.Context, domain string) {
	opts := &QueryOptions{
		Domain:  domain,
		Timeout: m.config.QueryTimeout,
	}

	result, err := ExecuteQueryWithResultContext(ctx, opts)

	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.watchlist[domain]
	if !exists {
		return
	}

	state.LastCheck = time.Now()

	if err != nil {
		state.Status = WatchStatusError
		m.emitAlert(&DomainAlert{
			ID:        fmt.Sprintf("%s-error-%d", domain, time.Now().Unix()),
			Domain:    domain,
			Type:      AlertQueryError,
			Level:     AlertLevelWarning,
			Message:   fmt.Sprintf("WHOIS查询失败: %v", err),
			Timestamp: time.Now(),
			Action:    "请检查网络连接或稍后重试",
		})
		return
	}

	if result.Info == nil {
		return
	}

	// 比较变更
	var alerts []*DomainAlert

	// 到期检查
	if result.Info.Domain != nil && result.Info.Domain.ExpirationDate != "" {
		state.ExpirationDate = result.Info.Domain.ExpirationDate
		days := calculateDaysRemaining(result.Info.Domain.ExpirationDate)
		state.DaysRemaining = days

		newStatus := m.determineWatchStatus(days)
		if newStatus != state.Status {
			state.Status = newStatus
			alerts = append(alerts, m.createExpiryAlert(domain, newStatus, days))
		}
	}

	// 状态变更
	if m.config.WatchStatusChange && state.LastInfo != nil && state.LastInfo.Domain != nil && result.Info.Domain != nil {
		oldStatus := state.LastInfo.Domain.Status
		newStatus := result.Info.Domain.Status
		if !stringSlicesEqual(oldStatus, newStatus) {
			alerts = append(alerts, &DomainAlert{
				ID:        fmt.Sprintf("%s-status-%d", domain, time.Now().Unix()),
				Domain:    domain,
				Type:      AlertStatusChange,
				Level:     AlertLevelWarning,
				Message:   fmt.Sprintf("域名状态发生变更"),
				OldValue:  formatStringSlice(oldStatus),
				NewValue:  formatStringSlice(newStatus),
				Timestamp: time.Now(),
				Action:    "请确认状态变更是否正常",
			})
		}
	}

	// 注册人变更
	if m.config.WatchRegistrantChange && state.LastInfo != nil && state.LastInfo.Registrant != nil && result.Info.Registrant != nil {
		if state.LastInfo.Registrant.Name != result.Info.Registrant.Name ||
			state.LastInfo.Registrant.Email != result.Info.Registrant.Email ||
			state.LastInfo.Registrant.Organization != result.Info.Registrant.Organization {
			alerts = append(alerts, &DomainAlert{
				ID:        fmt.Sprintf("%s-registrant-%d", domain, time.Now().Unix()),
				Domain:    domain,
				Type:      AlertRegistrantChange,
				Level:     AlertLevelCritical,
				Message:   "注册人信息发生变更",
				OldValue:  formatContact(state.LastInfo.Registrant),
				NewValue:  formatContact(result.Info.Registrant),
				Timestamp: time.Now(),
				Action:    "请确认注册人变更是否合法",
			})
		}
	}

	// NS变更
	if m.config.WatchNSChange && state.LastInfo != nil && state.LastInfo.Domain != nil && result.Info.Domain != nil {
		oldNS := state.LastInfo.Domain.NameServers
		newNS := result.Info.Domain.NameServers
		if !stringSlicesEqual(oldNS, newNS) {
			alerts = append(alerts, &DomainAlert{
				ID:        fmt.Sprintf("%s-ns-%d", domain, time.Now().Unix()),
				Domain:    domain,
				Type:      AlertNSChange,
				Level:     AlertLevelWarning,
				Message:   "域名服务器(NS)发生变更",
				OldValue:  formatStringSlice(oldNS),
				NewValue:  formatStringSlice(newNS),
				Timestamp: time.Now(),
				Action:    "请确认NS变更是否正常",
			})
		}
	}

	// 更新状态
	state.LastInfo = result.Info
	state.LastRaw = result.RawResponse
	if len(alerts) > 0 {
		state.AlertCount += len(alerts)
		state.Status = WatchStatusChanged
	}

	// 发送告警
	for _, alert := range alerts {
		m.emitAlert(alert)
	}
}

// determineWatchStatus 根据剩余天数确定监控状态
func (m *DomainMonitor) determineWatchStatus(days int) WatchStatus {
	if days <= 0 {
		return WatchStatusExpired
	}
	if days <= m.config.ExpiryCriticalDays {
		return WatchStatusCritical
	}
	if days <= m.config.ExpiryWarningDays {
		return WatchStatusWarning
	}
	return WatchStatusActive
}

// createExpiryAlert 创建到期告警
func (m *DomainMonitor) createExpiryAlert(domain string, status WatchStatus, days int) *DomainAlert {
	alert := &DomainAlert{
		ID:        fmt.Sprintf("%s-expiry-%d", domain, time.Now().Unix()),
		Domain:    domain,
		Type:      AlertExpiryWarning,
		Timestamp: time.Now(),
	}

	switch status {
	case WatchStatusExpired:
		alert.Level = AlertLevelCritical
		alert.Type = AlertExpiryPassed
		alert.Message = fmt.Sprintf("域名已过期 %d 天", -days)
		alert.Action = "域名已过期，请立即续费或采取其他措施"
	case WatchStatusCritical:
		alert.Level = AlertLevelCritical
		alert.Type = AlertExpiryCritical
		alert.Message = fmt.Sprintf("域名将在 %d 天后到期", days)
		alert.Action = "请立即续费以避免域名过期"
	case WatchStatusWarning:
		alert.Level = AlertLevelWarning
		alert.Message = fmt.Sprintf("域名将在 %d 天后到期", days)
		alert.Action = "请关注域名到期时间并安排续费"
	default:
		alert.Level = AlertLevelInfo
		alert.Message = fmt.Sprintf("域名到期状态更新: 剩余 %d 天", days)
	}

	return alert
}

// emitAlert 发送告警
func (m *DomainMonitor) emitAlert(alert *DomainAlert) {
	if m.alertCallback != nil {
		m.alertCallback(alert)
	}

	select {
	case m.alertChan <- alert:
	default:
		logrus.Warnf("告警通道已满，丢弃告警: %s - %s", alert.Domain, alert.Message)
	}
}

// CheckNow 立即检查指定域名
func (m *DomainMonitor) CheckNow(ctx context.Context, domain string) error {
	m.mu.RLock()
	_, exists := m.watchlist[domain]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("域名 %s 不在监控列表中", domain)
	}

	m.checkDomain(ctx, domain)
	return nil
}

// CollectAlerts 收集所有告警（阻塞直到通道关闭或上下文取消）
func CollectAlerts(alertChan <-chan *DomainAlert) []*DomainAlert {
	var alerts []*DomainAlert
	for alert := range alertChan {
		alerts = append(alerts, alert)
	}
	return alerts
}

// 辅助函数

// calculateDaysRemaining 计算距离到期还有多少天
func calculateDaysRemaining(expirationDate string) int {
	if expirationDate == "" {
		return -1
	}

	// 尝试多种日期格式
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"Jan 02 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, expirationDate); err == nil {
			return int(time.Until(t).Hours() / 24)
		}
	}

	return -1
}

// stringSlicesEqual 比较两个字符串切片是否相等
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// formatStringSlice 格式化字符串切片为逗号分隔
func formatStringSlice(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ", "
		}
		result += v
	}
	return result
}

// formatContact 格式化联系人信息
func formatContact(c *whoisparser.Contact) string {
	if c == nil {
		return ""
	}
	parts := []string{}
	if c.Name != "" {
		parts = append(parts, c.Name)
	}
	if c.Organization != "" {
		parts = append(parts, c.Organization)
	}
	if c.Email != "" {
		parts = append(parts, c.Email)
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "; "
		}
		result += p
	}
	return result
}
