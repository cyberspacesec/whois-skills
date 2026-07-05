package whois

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 反向 WHOIS 本地实现
//
// 现有 reverse.go 定义了 ReverseWhoisProvider 接口（SearchByRegistrant/Email/
// Organization + Name）。本文件提供：
//   - LocalReverseWhoisIndex：基于已采集 WHOIS 快照构建的内存倒排索引，
//     实现该接口（不接入外部付费 API，符合"接口抽象+本地存储实现"策略）
//   - 全局 provider 注入与配置初始化
//   - IndexSnapshot/RebuildFromSnapshots：构建本地索引
// ============================================================================

// ReverseWhoisConfig 反向 WHOIS 配置。
type ReverseWhoisConfig struct {
	// 是否启用反向查询
	Enabled bool `json:"enabled"`

	// 数据源类型 (local/custom)
	Type string `json:"type"`
}

// ---- 全局 Provider ----

var globalReverseWhoisProvider ReverseWhoisProvider

// GetReverseWhoisProvider 返回全局反向 WHOIS 提供者。
func GetReverseWhoisProvider() ReverseWhoisProvider {
	return globalReverseWhoisProvider
}

// SetReverseWhoisProvider 注入自定义反向 WHOIS 提供者。
func SetReverseWhoisProvider(p ReverseWhoisProvider) {
	globalReverseWhoisProvider = p
}

// InitReverseWhoisFromConfig 从配置初始化全局 ReverseWhoisProvider。
func InitReverseWhoisFromConfig(cfg *ReverseWhoisConfig) error {
	if !cfg.Enabled {
		globalReverseWhoisProvider = nil
		return nil
	}
	switch cfg.Type {
	case "local":
		// 本地实现依赖 HistoryProvider 的快照
		hp := GetHistoryProvider()
		if hp == nil {
			return fmt.Errorf("反向 WHOIS 本地实现需要先启用 HistoryProvider")
		}
		globalReverseWhoisProvider = NewLocalReverseWhoisIndex(hp)
		return nil
	default:
		return fmt.Errorf("未知反向 WHOIS 数据源类型: %s", cfg.Type)
	}
}

// LocalReverseWhoisIndex 基于内存倒排索引的反向 WHOIS 实现。
// 索引键格式：<field>:<value>，值为域名集合。
type LocalReverseWhoisIndex struct {
	mu     sync.RWMutex
	// 倒排索引：field:value → domain set
	index map[string]map[string]bool
	// 历史快照提供者（用于扩展查询，可选）
	history HistoryProvider
}

// NewLocalReverseWhoisIndex 创建本地反向 WHOIS 索引。
func NewLocalReverseWhoisIndex(history HistoryProvider) *LocalReverseWhoisIndex {
	return &LocalReverseWhoisIndex{
		index:   make(map[string]map[string]bool),
		history: history,
	}
}

// Name 返回提供者名称。
func (idx *LocalReverseWhoisIndex) Name() string { return "local-index" }

// indexKey 生成索引键。
func indexKey(field, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return field + ":" + value
}

// indexContact 索引联系人字段。
func (idx *LocalReverseWhoisIndex) indexContact(domain string, contact *whoisparser.Contact) {
	if contact == nil {
		return
	}
	if contact.Email != "" {
		idx.addToIndex(indexKey("email", contact.Email), domain)
	}
	if contact.Name != "" {
		idx.addToIndex(indexKey("name", contact.Name), domain)
	}
	if contact.Organization != "" {
		idx.addToIndex(indexKey("organization", contact.Organization), domain)
	}
	if contact.Phone != "" {
		idx.addToIndex(indexKey("phone", contact.Phone), domain)
	}
}

// addToIndex 添加到索引（调用方需持锁）。
func (idx *LocalReverseWhoisIndex) addToIndex(key, domain string) {
	if idx.index[key] == nil {
		idx.index[key] = make(map[string]bool)
	}
	idx.index[key][domain] = true
}

// IndexSnapshot 索引单个 WHOIS 快照（构建本地索引时调用）。
func (idx *LocalReverseWhoisIndex) IndexSnapshot(ctx context.Context, snapshot *WhoisSnapshot) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if snapshot == nil || snapshot.Domain == "" {
		return nil
	}
	idx.indexContact(snapshot.Domain, snapshot.Info.Registrant)
	idx.indexContact(snapshot.Domain, snapshot.Info.Administrative)
	idx.indexContact(snapshot.Domain, snapshot.Info.Technical)
	idx.indexContact(snapshot.Domain, snapshot.Info.Billing)
	return nil
}

// RebuildFromSnapshots 从给定快照列表重建索引。
func (idx *LocalReverseWhoisIndex) RebuildFromSnapshots(ctx context.Context, snapshots []WhoisSnapshot) (int, error) {
	idx.mu.Lock()
	idx.index = make(map[string]map[string]bool)
	idx.mu.Unlock()

	count := 0
	for i := range snapshots {
		if err := idx.IndexSnapshot(ctx, &snapshots[i]); err != nil {
			logrus.Warnf("索引快照 %s 失败: %v", snapshots[i].Domain, err)
			continue
		}
		count++
	}
	return count, nil
}

// SearchByRegistrant 根据注册人姓名搜索域名。
func (idx *LocalReverseWhoisIndex) SearchByRegistrant(ctx context.Context, query string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return idx.searchByField(ctx, "name", query, opts)
}

// SearchByEmail 根据邮箱搜索域名。
func (idx *LocalReverseWhoisIndex) SearchByEmail(ctx context.Context, email string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return idx.searchByField(ctx, "email", email, opts)
}

// SearchByOrganization 根据组织搜索域名。
func (idx *LocalReverseWhoisIndex) SearchByOrganization(ctx context.Context, org string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	return idx.searchByField(ctx, "organization", org, opts)
}

// searchByField 通用字段搜索。
func (idx *LocalReverseWhoisIndex) searchByField(ctx context.Context, field, value string, opts *ReverseWhoisOptions) ([]*ReverseWhoisResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if value == "" {
		return nil, fmt.Errorf("查询值不能为空")
	}

	key := indexKey(field, value)
	domains := idx.index[key]
	if len(domains) == 0 {
		// 模糊匹配：value 作为子串
		var matched []string
		prefix := field + ":" + strings.ToLower(value)
		for k, dset := range idx.index {
			if strings.HasPrefix(k, prefix) || strings.Contains(k, value) {
				for d := range dset {
					matched = append(matched, d)
				}
			}
		}
		// 去重
		seen := make(map[string]bool)
		for _, d := range matched {
			seen[d] = true
		}
		domains = seen
	}

	var results []*ReverseWhoisResult
	for domain := range domains {
		results = append(results, &ReverseWhoisResult{
			Domain: domain,
		})
	}

	// 排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Domain < results[j].Domain
	})

	// 应用限制
	if opts != nil && opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// Close 无操作。
func (idx *LocalReverseWhoisIndex) Close() error { return nil }

// IndexWhoisSnapshot 索引 WHOIS 快照到全局反向索引（若已注入）。
func IndexWhoisSnapshot(ctx context.Context, snapshot *WhoisSnapshot) error {
	provider := GetReverseWhoisProvider()
	if provider == nil {
		return nil
	}
	if idx, ok := provider.(*LocalReverseWhoisIndex); ok {
		return idx.IndexSnapshot(ctx, snapshot)
	}
	return nil
}