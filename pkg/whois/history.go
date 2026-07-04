package whois

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 历史 WHOIS 快照
//
// 上层可查询域名在不同时间点的 WHOIS 记录，用于检测注册人/NS/过期日期等变更。
// HistoryProvider 接口抽象历史数据存储，内置 LocalHistoryStorage 使用
// StorageProvider（本地/Redis）按时间戳索引存储快照。
//
// 使用方式：
//   - 查询后调用 SaveHistorySnapshot 将当前结果落盘
//   - 调用 QueryHistorySnapshots 取历史快照列表
//   - 调用 CompareSnapshots 对比两次快照差异
// ============================================================================

// WhoisSnapshot WHOIS 历史快照。
type WhoisSnapshot struct {
	// 域名
	Domain string `json:"domain"`

	// 快照时间（Unix 秒）
	Timestamp int64 `json:"timestamp"`

	// WHOIS 信息（结构化）
	Info whoisparser.WhoisInfo `json:"info"`

	// 原始 WHOIS 文本（可选）
	RawResponse string `json:"raw_response,omitempty"`

	// 查询来源
	Source string `json:"source,omitempty"`

	// 备注（如"手动采集"/"定时监控"）
	Note string `json:"note,omitempty"`
}

// WhoisSnapshotDiff 两次快照差异。
type WhoisSnapshotDiff struct {
	Domain       string           `json:"domain"`
	FromTime     int64            `json:"from_time"`
	ToTime       int64            `json:"to_time"`
	ChangedFields []FieldChange   `json:"changed_fields,omitempty"`
	AddedFields  []string         `json:"added_fields,omitempty"`
	RemovedFields []string        `json:"removed_fields,omitempty"`
}

// FieldChange 字段变更详情。
type FieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value,omitempty"`
	NewValue string `json:"new_value,omitempty"`
}

// HistoryProvider 历史 WHOIS 快照提供者接口。
type HistoryProvider interface {
	// SaveSnapshot 保存快照。
	SaveSnapshot(ctx context.Context, snapshot *WhoisSnapshot) error
	// QuerySnapshots 查询域名的所有快照（按时间升序）。
	QuerySnapshots(ctx context.Context, domain string) ([]WhoisSnapshot, error)
	// QuerySnapshotsInRange 查询域名的快照时间范围。
	QuerySnapshotsInRange(ctx context.Context, domain string, start, end int64) ([]WhoisSnapshot, error)
	// GetSnapshot 获取指定时间点的快照。
	GetSnapshot(ctx context.Context, domain string, timestamp int64) (*WhoisSnapshot, error)
	// DeleteSnapshot 删除指定快照。
	DeleteSnapshot(ctx context.Context, domain string, timestamp int64) error
	// Close 关闭数据源。
	Close() error
}

// HistoryConfig 历史 WHOIS 配置。
type HistoryConfig struct {
	// 是否启用历史快照
	Enabled bool `json:"enabled"`

	// 存储类型 (local/redis/custom)
	Type string `json:"type"`

	// 本地存储目录（type=local，若空则复用 StorageProvider）
	Directory string `json:"directory,omitempty"`

	// 最大保留天数（自动清理过期）
	MaxRetentionDays int `json:"max_retention_days,omitempty"`
}

// ---- 全局 HistoryProvider ----

var globalHistoryProvider HistoryProvider

// GetHistoryProvider 返回全局历史快照提供者。
func GetHistoryProvider() HistoryProvider {
	return globalHistoryProvider
}

// SetHistoryProvider 注入自定义历史快照提供者。
func SetHistoryProvider(p HistoryProvider) {
	globalHistoryProvider = p
}

// InitHistoryFromConfig 从配置初始化全局 HistoryProvider。
func InitHistoryFromConfig(cfg *HistoryConfig) error {
	if !cfg.Enabled {
		globalHistoryProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		// 复用 StorageProvider（若已初始化）或新建本地存储
		sp := GetStorageProvider()
		if sp == nil {
			if cfg.Directory == "" {
				cfg.Directory = "data/history"
			}
			sp, err := NewLocalFileStorage(cfg.Directory)
			if err != nil {
				return err
			}
			SetStorageProvider(sp)
		}
		globalHistoryProvider = NewLocalHistoryStorage(sp)
		return nil
	default:
		return fmt.Errorf("未知历史存储类型: %s", cfg.Type)
	}
}

// ---- LocalHistoryStorage 使用 StorageProvider 存储快照 ----

// LocalHistoryStorage 本地历史快照存储（复用 StorageProvider）。
type LocalHistoryStorage struct {
	storage StorageProvider
	mu      sync.RWMutex
}

// NewLocalHistoryStorage 创建本地历史存储。
func NewLocalHistoryStorage(storage StorageProvider) *LocalHistoryStorage {
	return &LocalHistoryStorage{storage: storage}
}

// historyKey 生成历史快照的存储 key：history:<domain>:<timestamp>
func historyKey(domain string, timestamp int64) string {
	return fmt.Sprintf("history:%s:%d", domain, timestamp)
}

// historyDomainPrefix 生成域名历史快照的前缀。
func historyDomainPrefix(domain string) string {
	return fmt.Sprintf("history:%s:", domain)
}

// SaveSnapshot 保存快照。
func (s *LocalHistoryStorage) SaveSnapshot(ctx context.Context, snapshot *WhoisSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if snapshot.Domain == "" {
		return fmt.Errorf("域名不能为空")
	}
	if snapshot.Timestamp == 0 {
		snapshot.Timestamp = time.Now().Unix()
	}
	return s.storage.Save(ctx, historyKey(snapshot.Domain, snapshot.Timestamp), snapshot)
}

// QuerySnapshots 查询域名的所有快照（按时间升序）。
func (s *LocalHistoryStorage) QuerySnapshots(ctx context.Context, domain string) ([]WhoisSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys, err := s.storage.List(ctx, historyDomainPrefix(domain))
	if err != nil {
		return nil, err
	}
	var snapshots []WhoisSnapshot
	for _, key := range keys {
		var snap WhoisSnapshot
		if err := s.storage.Load(ctx, key, &snap); err != nil {
			logrus.Debugf("加载快照 %s 失败: %v", key, err)
			continue
		}
		snapshots = append(snapshots, snap)
	}
	// 按时间升序排序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp < snapshots[j].Timestamp
	})
	return snapshots, nil
}

// QuerySnapshotsInRange 查询域名的快照时间范围。
func (s *LocalHistoryStorage) QuerySnapshotsInRange(ctx context.Context, domain string, start, end int64) ([]WhoisSnapshot, error) {
	all, err := s.QuerySnapshots(ctx, domain)
	if err != nil {
		return nil, err
	}
	var result []WhoisSnapshot
	for _, snap := range all {
		if snap.Timestamp >= start && snap.Timestamp <= end {
			result = append(result, snap)
		}
	}
	return result, nil
}

// GetSnapshot 获取指定时间点的快照。
func (s *LocalHistoryStorage) GetSnapshot(ctx context.Context, domain string, timestamp int64) (*WhoisSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snap WhoisSnapshot
	err := s.storage.Load(ctx, historyKey(domain, timestamp), &snap)
	if err != nil {
		return nil, fmt.Errorf("快照不存在: %s/%d", domain, timestamp)
	}
	return &snap, nil
}

// DeleteSnapshot 删除指定快照。
func (s *LocalHistoryStorage) DeleteSnapshot(ctx context.Context, domain string, timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storage.Delete(ctx, historyKey(domain, timestamp))
}

// Close 无操作（StorageProvider 由上层管理）。
func (s *LocalHistoryStorage) Close() error { return nil }

// ---- 便捷函数 ----

// SaveHistorySnapshot 保存当前 WHOIS 查询结果为历史快照。
// 若全局 HistoryProvider 未注入，静默跳过（不报错）。
func SaveHistorySnapshot(ctx context.Context, domain string, info *whoisparser.WhoisInfo, raw string, note string) error {
	provider := GetHistoryProvider()
	if provider == nil {
		logrus.Debugf("历史快照未启用，跳过保存: %s", domain)
		return nil
	}
	if info == nil {
		return fmt.Errorf("WHOIS 信息为空")
	}
	snapshot := &WhoisSnapshot{
		Domain:      domain,
		Timestamp:   time.Now().Unix(),
		Info:        *info,
		RawResponse: raw,
		Source:      "sdk",
		Note:        note,
	}
	return provider.SaveSnapshot(ctx, snapshot)
}

// QueryHistorySnapshots 查询域名的所有历史快照。
func QueryHistorySnapshots(ctx context.Context, domain string) ([]WhoisSnapshot, error) {
	provider := GetHistoryProvider()
	if provider == nil {
		return nil, fmt.Errorf("历史快照未启用")
	}
	return provider.QuerySnapshots(ctx, domain)
}

// CompareSnapshots 对比两次快照差异。
func CompareSnapshots(from, to *WhoisSnapshot) *WhoisSnapshotDiff {
	if from == nil || to == nil {
		return nil
	}
	diff := &WhoisSnapshotDiff{
		Domain:     to.Domain,
		FromTime:   from.Timestamp,
		ToTime:     to.Timestamp,
	}
	// 对比 Domain 字段
	compareWhoisFields(&from.Info, &to.Info, diff)
	return diff
}

// compareWhoisFields 对比 WhoisInfo 字段变更。
func compareWhoisFields(from, to *whoisparser.WhoisInfo, diff *WhoisSnapshotDiff) {
	// Domain
	if from.Domain != nil && to.Domain != nil {
		compareStructFields(from.Domain, to.Domain, "domain", diff)
		// NameServers 在 Domain 内，单独对比
		compareStringLists(from.Domain.NameServers, to.Domain.NameServers, "nameservers", diff)
	} else if from.Domain == nil && to.Domain != nil {
		diff.AddedFields = append(diff.AddedFields, "domain")
	} else if from.Domain != nil && to.Domain == nil {
		diff.RemovedFields = append(diff.RemovedFields, "domain")
	}

	// Registrar
	if from.Registrar != nil && to.Registrar != nil {
		compareStructFields(from.Registrar, to.Registrar, "registrar", diff)
	} else if from.Registrar == nil && to.Registrar != nil {
		diff.AddedFields = append(diff.AddedFields, "registrar")
	} else if from.Registrar != nil && to.Registrar == nil {
		diff.RemovedFields = append(diff.RemovedFields, "registrar")
	}

	// Registrant
	if from.Registrant != nil && to.Registrant != nil {
		compareStructFields(from.Registrant, to.Registrant, "registrant", diff)
	} else if from.Registrant == nil && to.Registrant != nil {
		diff.AddedFields = append(diff.AddedFields, "registrant")
	} else if from.Registrant != nil && to.Registrant == nil {
		diff.RemovedFields = append(diff.RemovedFields, "registrant")
	}

	// Administrative
	if from.Administrative != nil && to.Administrative != nil {
		compareStructFields(from.Administrative, to.Administrative, "administrative", diff)
	} else if from.Administrative == nil && to.Administrative != nil {
		diff.AddedFields = append(diff.AddedFields, "administrative")
	} else if from.Administrative != nil && to.Administrative == nil {
		diff.RemovedFields = append(diff.RemovedFields, "administrative")
	}

	// Technical
	if from.Technical != nil && to.Technical != nil {
		compareStructFields(from.Technical, to.Technical, "technical", diff)
	} else if from.Technical == nil && to.Technical != nil {
		diff.AddedFields = append(diff.AddedFields, "technical")
	} else if from.Technical != nil && to.Technical == nil {
		diff.RemovedFields = append(diff.RemovedFields, "technical")
	}

	// Billing
	if from.Billing != nil && to.Billing != nil {
		compareStructFields(from.Billing, to.Billing, "billing", diff)
	} else if from.Billing == nil && to.Billing != nil {
		diff.AddedFields = append(diff.AddedFields, "billing")
	} else if from.Billing != nil && to.Billing == nil {
		diff.RemovedFields = append(diff.RemovedFields, "billing")
	}
}

// compareStructFields 通用结构体字段对比（反射对比一级字符串字段）。
func compareStructFields(from, to interface{}, prefix string, diff *WhoisSnapshotDiff) {
	fromVal := reflectValue(from)
	toVal := reflectValue(to)
	if !fromVal.IsValid() || !toVal.IsValid() {
		return
	}
	t := fromVal.Type()
	for i := 0; i < fromVal.NumField(); i++ {
		fieldName := t.Field(i).Name
		fv := fromVal.Field(i)
		tv := toVal.Field(i)
		// 只对比字符串与基本类型
		switch fv.Kind() {
		case reflectKindString:
			oldVal := fv.String()
			newVal := tv.String()
			if oldVal != newVal {
				diff.ChangedFields = append(diff.ChangedFields, FieldChange{
					Field:    prefix + "." + fieldName,
					OldValue: oldVal,
					NewValue: newVal,
				})
			}
		case reflectKindBool:
			if fv.Bool() != tv.Bool() {
				diff.ChangedFields = append(diff.ChangedFields, FieldChange{
					Field:    prefix + "." + fieldName,
					OldValue: formatBool(fv.Bool()),
					NewValue: formatBool(tv.Bool()),
				})
			}
		}
	}
}

// compareStringLists 对比字符串列表。
func compareStringLists(from, to []string, field string, diff *WhoisSnapshotDiff) {
	fromSet := make(map[string]bool)
	for _, s := range from {
		fromSet[s] = true
	}
	toSet := make(map[string]bool)
	for _, s := range to {
		toSet[s] = true
	}
	// 检查新增
	for _, s := range to {
		if !fromSet[s] {
			diff.ChangedFields = append(diff.ChangedFields, FieldChange{
				Field:    field + ":+" + s,
				NewValue: s,
			})
		}
	}
	// 检查删除
	for _, s := range from {
		if !toSet[s] {
			diff.ChangedFields = append(diff.ChangedFields, FieldChange{
				Field:    field + ":-" + s,
				OldValue: s,
			})
		}
	}
}

// reflectValue 返回 interface{} 的 reflect.Value（解引用指针）。
func reflectValue(v interface{}) reflect.Value {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}
		}
		rv = rv.Elem()
	}
	return rv
}

// reflectKind 字符串类型的别名，便于测试隔离。
type reflectKind = reflect.Kind

var (
	reflectKindString = reflect.String
	reflectKindBool   = reflect.Bool
)

// formatBool 格式化布尔值为字符串。
func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}