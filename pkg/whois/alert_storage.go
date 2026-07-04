package whois

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ============================================================================
// 监控/告警持久化
//
// AlertStorageProvider 接口抽象告警历史存储，内置 LocalAlertStorage 复用
// StorageProvider。MonitorStateProvider 接口抽象监控状态持久化，支持进程重启后
// 恢复 watchlist。
//
// 使用方式：
//   - 告警触发后调用 SaveAlert 持久化
//   - 调用 QueryAlerts 查询历史告警
//   - 监控启动时 LoadMonitorState 恢复 watchlist
// ============================================================================

// AlertStorageProvider 告警存储提供者接口。
type AlertStorageProvider interface {
	// SaveAlert 保存告警。
	SaveAlert(ctx context.Context, alert *DomainAlert) error
	// QueryAlerts 查询告警（按域名/类型/级别/时间范围过滤）。
	QueryAlerts(ctx context.Context, filter AlertFilter) ([]DomainAlert, error)
	// GetAlert 获取指定告警。
	GetAlert(ctx context.Context, id string) (*DomainAlert, error)
	// DeleteAlert 删除告警。
	DeleteAlert(ctx context.Context, id string) error
	// Close 关闭数据源。
	Close() error
}

// AlertFilter 告警查询过滤器。
type AlertFilter struct {
	Domain    string     `json:"domain,omitempty"`
	Type      AlertType  `json:"type,omitempty"`
	Level     AlertLevel `json:"level,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Limit     int        `json:"limit,omitempty"`
}

// MonitorStateProvider 监控状态持久化接口。
type MonitorStateProvider interface {
	// SaveWatchState 保存单个域名的监控状态。
	SaveWatchState(ctx context.Context, state *DomainWatchState) error
	// LoadWatchStates 加载所有域名的监控状态。
	LoadWatchStates(ctx context.Context) (map[string]*DomainWatchState, error)
	// DeleteWatchState 删除域名的监控状态。
	DeleteWatchState(ctx context.Context, domain string) error
	// Close 关闭数据源。
	Close() error
}

// AlertStorageConfig 告警存储配置。
type AlertStorageConfig struct {
	// 是否启用告警持久化
	Enabled bool `json:"enabled"`

	// 存储类型 (local/redis)
	Type string `json:"type"`

	// 本地存储目录（type=local）
	Directory string `json:"directory,omitempty"`
}

// MonitorStateConfig 监控状态持久化配置。
type MonitorStateConfig struct {
	// 是否启用状态持久化
	Enabled bool `json:"enabled"`

	// 存储类型 (local/redis)
	Type string `json:"type"`

	// 本地存储目录（type=local）
	Directory string `json:"directory,omitempty"`
}

// ---- 全局 Provider ----

var globalAlertStorageProvider AlertStorageProvider
var globalMonitorStateProvider MonitorStateProvider

// GetAlertStorageProvider 返回全局告警存储提供者。
func GetAlertStorageProvider() AlertStorageProvider {
	return globalAlertStorageProvider
}

// SetAlertStorageProvider 注入自定义告警存储提供者。
func SetAlertStorageProvider(p AlertStorageProvider) {
	globalAlertStorageProvider = p
}

// GetMonitorStateProvider 返回全局监控状态提供者。
func GetMonitorStateProvider() MonitorStateProvider {
	return globalMonitorStateProvider
}

// SetMonitorStateProvider 注入自定义监控状态提供者。
func SetMonitorStateProvider(p MonitorStateProvider) {
	globalMonitorStateProvider = p
}

// InitAlertStorageFromConfig 从配置初始化告警存储。
func InitAlertStorageFromConfig(cfg *AlertStorageConfig) error {
	if !cfg.Enabled {
		globalAlertStorageProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		sp := GetStorageProvider()
		if sp == nil {
			if cfg.Directory == "" {
				cfg.Directory = "data/alerts"
			}
			var err error
			sp, err = NewLocalFileStorage(cfg.Directory)
			if err != nil {
				return err
			}
			SetStorageProvider(sp)
		}
		globalAlertStorageProvider = NewLocalAlertStorage(sp)
		return nil
	default:
		return fmt.Errorf("未知告警存储类型: %s", cfg.Type)
	}
}

// InitMonitorStateFromConfig 从配置初始化监控状态存储。
func InitMonitorStateFromConfig(cfg *MonitorStateConfig) error {
	if !cfg.Enabled {
		globalMonitorStateProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		sp := GetStorageProvider()
		if sp == nil {
			if cfg.Directory == "" {
				cfg.Directory = "data/monitor"
			}
			var err error
			sp, err = NewLocalFileStorage(cfg.Directory)
			if err != nil {
				return err
			}
			SetStorageProvider(sp)
		}
		globalMonitorStateProvider = NewLocalMonitorStateStorage(sp)
		return nil
	default:
		return fmt.Errorf("未知监控状态存储类型: %s", cfg.Type)
	}
}

// ---- LocalAlertStorage 本地告警存储 ----

// LocalAlertStorage 使用 StorageProvider 存储告警。
type LocalAlertStorage struct {
	storage StorageProvider
	mu      sync.RWMutex
}

// NewLocalAlertStorage 创建本地告警存储。
func NewLocalAlertStorage(storage StorageProvider) *LocalAlertStorage {
	return &LocalAlertStorage{storage: storage}
}

// alertKey 生成告警存储 key：alert:<timestamp>:<id>
func alertKey(alert *DomainAlert) string {
	return fmt.Sprintf("alert:%d:%s", alert.Timestamp.Unix(), alert.ID)
}

// alertPrefix 生成告警前缀。
func alertPrefix() string {
	return "alert:"
}

// SaveAlert 保存告警。
func (s *LocalAlertStorage) SaveAlert(ctx context.Context, alert *DomainAlert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if alert.ID == "" {
		alert.ID = generateAlertID()
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}
	return s.storage.Save(ctx, alertKey(alert), alert)
}

// QueryAlerts 查询告警。
func (s *LocalAlertStorage) QueryAlerts(ctx context.Context, filter AlertFilter) ([]DomainAlert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.storage.List(ctx, alertPrefix())
	if err != nil {
		return nil, err
	}
	var alerts []DomainAlert
	for _, key := range keys {
		var alert DomainAlert
		if err := s.storage.Load(ctx, key, &alert); err != nil {
			logrus.Debugf("加载告警 %s 失败: %v", key, err)
			continue
		}
		// 应用过滤条件
		if filter.Domain != "" && alert.Domain != filter.Domain {
			continue
		}
		if filter.Type != "" && alert.Type != filter.Type {
			continue
		}
		if filter.Level != "" && alert.Level != filter.Level {
			continue
		}
		if filter.StartTime != nil && alert.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && alert.Timestamp.After(*filter.EndTime) {
			continue
		}
		alerts = append(alerts, alert)
	}
	// 按时间降序排序
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.After(alerts[j].Timestamp)
	})
	// 应用限制
	if filter.Limit > 0 && len(alerts) > filter.Limit {
		alerts = alerts[:filter.Limit]
	}
	return alerts, nil
}

// GetAlert 获取指定告警。
func (s *LocalAlertStorage) GetAlert(ctx context.Context, id string) (*DomainAlert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 需要扫描找到对应 ID
	keys, err := s.storage.List(ctx, alertPrefix())
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		var alert DomainAlert
		if err := s.storage.Load(ctx, key, &alert); err != nil {
			continue
		}
		if alert.ID == id {
			return &alert, nil
		}
	}
	return nil, fmt.Errorf("告警不存在: %s", id)
}

// DeleteAlert 删除告警。
func (s *LocalAlertStorage) DeleteAlert(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys, err := s.storage.List(ctx, alertPrefix())
	if err != nil {
		return err
	}
	for _, key := range keys {
		var alert DomainAlert
		if err := s.storage.Load(ctx, key, &alert); err != nil {
			continue
		}
		if alert.ID == id {
			return s.storage.Delete(ctx, key)
		}
	}
	return fmt.Errorf("告警不存在: %s", id)
}

// Close 无操作。
func (s *LocalAlertStorage) Close() error { return nil }

// generateAlertID 生成告警 ID。
func generateAlertID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ---- LocalMonitorStateStorage 本地监控状态存储 ----

// LocalMonitorStateStorage 使用 StorageProvider 存储监控状态。
type LocalMonitorStateStorage struct {
	storage StorageProvider
	mu      sync.RWMutex
}

// NewLocalMonitorStateStorage 创建本地监控状态存储。
func NewLocalMonitorStateStorage(storage StorageProvider) *LocalMonitorStateStorage {
	return &LocalMonitorStateStorage{storage: storage}
}

// watchStateKey 生成监控状态存储 key：watch:<domain>
func watchStateKey(domain string) string {
	return fmt.Sprintf("watch:%s", domain)
}

// SaveWatchState 保存监控状态。
func (s *LocalMonitorStateStorage) SaveWatchState(ctx context.Context, state *DomainWatchState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if state.Domain == "" {
		return fmt.Errorf("域名不能为空")
	}
	return s.storage.Save(ctx, watchStateKey(state.Domain), state)
}

// LoadWatchStates 加载所有监控状态。
func (s *LocalMonitorStateStorage) LoadWatchStates(ctx context.Context) (map[string]*DomainWatchState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.storage.List(ctx, "watch:")
	if err != nil {
		return nil, err
	}
	states := make(map[string]*DomainWatchState)
	for _, key := range keys {
		var state DomainWatchState
		if err := s.storage.Load(ctx, key, &state); err != nil {
			logrus.Debugf("加载监控状态 %s 失败: %v", key, err)
			continue
		}
		states[state.Domain] = &state
	}
	return states, nil
}

// DeleteWatchState 删除监控状态。
func (s *LocalMonitorStateStorage) DeleteWatchState(ctx context.Context, domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storage.Delete(ctx, watchStateKey(domain))
}

// Close 无操作。
func (s *LocalMonitorStateStorage) Close() error { return nil }

// ---- 便捷函数 ----

// SaveAlert 保存告警到全局存储（若已注入）。
func SaveAlert(ctx context.Context, alert *DomainAlert) error {
	provider := GetAlertStorageProvider()
	if provider == nil {
		logrus.Debugf("告警存储未启用，跳过保存: %s", alert.ID)
		return nil
	}
	return provider.SaveAlert(ctx, alert)
}

// QueryAlerts 查询告警。
func QueryAlerts(ctx context.Context, filter AlertFilter) ([]DomainAlert, error) {
	provider := GetAlertStorageProvider()
	if provider == nil {
		return nil, fmt.Errorf("告警存储未启用")
	}
	return provider.QueryAlerts(ctx, filter)
}

// SaveWatchState 保存监控状态到全局存储。
func SaveWatchState(ctx context.Context, state *DomainWatchState) error {
	provider := GetMonitorStateProvider()
	if provider == nil {
		logrus.Debugf("监控状态存储未启用，跳过保存: %s", state.Domain)
		return nil
	}
	return provider.SaveWatchState(ctx, state)
}

// LoadWatchStates 加载监控状态。
func LoadWatchStates(ctx context.Context) (map[string]*DomainWatchState, error) {
	provider := GetMonitorStateProvider()
	if provider == nil {
		return nil, fmt.Errorf("监控状态存储未启用")
	}
	return provider.LoadWatchStates(ctx)
}