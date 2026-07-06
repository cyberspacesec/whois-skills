package whois

import (
	"context"
	"testing"
	"time"

	whoisparser "github.com/likexian/whois-parser"
)

// TestBatchSaveToHistory 验证批量查询成功结果落盘到 HistoryProvider。
func TestBatchSaveToHistory(t *testing.T) {
	// 保存并恢复全局状态
	originalProvider := globalWhoisQueryProvider
	originalHistory := globalHistoryProvider
	originalStorage := globalStorageProvider
	defer func() {
		globalWhoisQueryProvider = originalProvider
		globalHistoryProvider = originalHistory
		globalStorageProvider = originalStorage
	}()

	// 注入 stub provider（返回固定结果）
	stub := &stubWhoisQueryProvider{
		raw: "raw whois text",
		info: whoisparser.WhoisInfo{
			Domain: &whoisparser.Domain{Domain: "test.example"},
		},
	}
	SetWhoisQueryProvider(stub)

	// 注入本地历史存储
	storage, err := NewLocalFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalFileStorage 失败: %v", err)
	}
	SetStorageProvider(storage)
	hp := NewLocalHistoryStorage(storage)
	SetHistoryProvider(hp)

	// 运行批量查询，启用 SaveToHistory
	config := DefaultStreamBatchConfig()
	config.Concurrency = 2
	config.Timeout = 5
	config.MaxRetries = 1
	config.QueryDelay = 0
	config.SaveToHistory = true

	processor := NewStreamBatchProcessor(config)
	domains := []string{"a.test", "b.test"}
	// stub 返回固定 domain=test.example，但保存时用查询的 domain
	// 调整 stub 让 Query 返回的 info.Domain 跟随查询域名
	stub2 := &domainAwareStubProvider{}
	SetWhoisQueryProvider(stub2)

	err = processor.Process(context.Background(), domains)
	if err != nil {
		t.Fatalf("Process 失败: %v", err)
	}

	// 等待结果消费完
	results := CollectResults(processor.Results())
	if len(results) != 2 {
		t.Fatalf("应得到 2 个结果，得到 %d", len(results))
	}

	// 给落盘 goroutine 一点时间（SaveHistorySnapshot 在 worker 内同步调用，无需等待）
	time.Sleep(50 * time.Millisecond)

	// 验证历史快照已落盘
	for _, domain := range domains {
		snaps, err := hp.QuerySnapshots(context.Background(), domain)
		if err != nil {
			t.Fatalf("QuerySnapshots(%s) 失败: %v", domain, err)
		}
		if len(snaps) == 0 {
			t.Errorf("域名 %s 应有历史快照，得到 0", domain)
		}
	}
}

// TestBatchSaveToHistoryDisabled 验证未启用时不落盘。
func TestBatchSaveToHistoryDisabled(t *testing.T) {
	originalProvider := globalWhoisQueryProvider
	originalHistory := globalHistoryProvider
	originalStorage := globalStorageProvider
	defer func() {
		globalWhoisQueryProvider = originalProvider
		globalHistoryProvider = originalHistory
		globalStorageProvider = originalStorage
	}()

	SetWhoisQueryProvider(&domainAwareStubProvider{})
	storage, _ := NewLocalFileStorage(t.TempDir())
	SetStorageProvider(storage)
	hp := NewLocalHistoryStorage(storage)
	SetHistoryProvider(hp)

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.Timeout = 5
	config.MaxRetries = 1
	config.QueryDelay = 0
	config.SaveToHistory = false // 未启用

	processor := NewStreamBatchProcessor(config)
	_ = processor.Process(context.Background(), []string{"x.test"})
	CollectResults(processor.Results())
	time.Sleep(50 * time.Millisecond)

	snaps, _ := hp.QuerySnapshots(context.Background(), "x.test")
	if len(snaps) != 0 {
		t.Errorf("未启用 SaveToHistory 时不应有快照，得到 %d", len(snaps))
	}
}

// TestBatchSaveToHistoryNoProvider 验证未注入 HistoryProvider 时不报错。
func TestBatchSaveToHistoryNoProvider(t *testing.T) {
	originalProvider := globalWhoisQueryProvider
	originalHistory := globalHistoryProvider
	defer func() {
		globalWhoisQueryProvider = originalProvider
		globalHistoryProvider = originalHistory
	}()

	SetWhoisQueryProvider(&domainAwareStubProvider{})
	globalHistoryProvider = nil

	config := DefaultStreamBatchConfig()
	config.Concurrency = 1
	config.Timeout = 5
	config.MaxRetries = 1
	config.QueryDelay = 0
	config.SaveToHistory = true

	processor := NewStreamBatchProcessor(config)
	err := processor.Process(context.Background(), []string{"y.test"})
	if err != nil {
		t.Fatalf("未注入 HistoryProvider 时应静默跳过，Process 不应失败: %v", err)
	}
	CollectResults(processor.Results())
}

// domainAwareStubProvider 根据查询域名返回对应 info 的桩 provider。
type domainAwareStubProvider struct{}

func (s *domainAwareStubProvider) Query(ctx context.Context, domain, server string, useProxy bool) (string, error) {
	return "raw response for " + domain, nil
}

func (s *domainAwareStubProvider) Parse(raw string) (whoisparser.WhoisInfo, error) {
	// 从 raw 提取域名（测试用）
	return whoisparser.WhoisInfo{
		Domain: &whoisparser.Domain{Domain: "stub"},
	}, nil
}