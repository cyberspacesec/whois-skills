package whois

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// StreamBatchConfig 流式批量查询配置
type StreamBatchConfig struct {
	// 并发数
	Concurrency int `json:"concurrency"`

	// 查询超时（秒）
	Timeout int `json:"timeout"`

	// 最大重试次数
	MaxRetries int `json:"max_retries"`

	// 重试间隔（毫秒）
	RetryInterval int `json:"retry_interval_ms"`

	// 断点续查文件路径
	CheckpointFile string `json:"checkpoint_file,omitempty"`

	// 断点保存间隔（每完成N个查询保存一次）
	CheckpointInterval int `json:"checkpoint_interval"`

	// 域间查询间隔（毫秒，用于限速）
	QueryDelay int `json:"query_delay_ms"`

	// 是否使用代理
	UseProxy bool `json:"use_proxy"`
}

// DefaultStreamBatchConfig 默认流式批量查询配置
func DefaultStreamBatchConfig() StreamBatchConfig {
	return StreamBatchConfig{
		Concurrency:        5,
		Timeout:            10,
		MaxRetries:         3,
		RetryInterval:      1000,
		CheckpointInterval: 10,
		QueryDelay:         200,
		UseProxy:           false,
	}
}

// StreamBatchResult 流式批量查询结果
type StreamBatchResult struct {
	// 查询的域名
	Domain string `json:"domain"`

	// WHOIS信息
	Info *whoisparser.WhoisInfo `json:"info,omitempty"`

	// 原始响应
	RawResponse string `json:"raw_response,omitempty"`

	// 查询耗时（毫秒）
	Latency int64 `json:"latency"`

	// 错误
	Error error `json:"error,omitempty"`

	// 重试次数
	RetryCount int `json:"retry_count"`

	// 是否来自缓存
	FromCache bool `json:"from_cache"`
}

// StreamBatchStats 流式批量查询统计
type StreamBatchStats struct {
	// 总任务数
	TotalTasks int64 `json:"total_tasks"`

	// 已完成
	Completed int64 `json:"completed"`

	// 成功数
	SuccessCount int64 `json:"success_count"`

	// 失败数
	FailureCount int64 `json:"failure_count"`

	// 来自缓存
	CacheHits int64 `json:"cache_hits"`

	// 平均延迟
	AvgLatency int64 `json:"avg_latency_ms"`

	// 已用时间
	Elapsed time.Duration `json:"elapsed"`

	// 预估剩余时间
	EstimatedRemaining time.Duration `json:"estimated_remaining"`
}

// StreamBatchProcessor 流式批量查询处理器
type StreamBatchProcessor struct {
	config StreamBatchConfig
	mu     sync.RWMutex

	// 任务状态
	totalTasks    int64
	completed     int64
	successCount  int64
	failureCount  int64
	cacheHits     int64
	totalLatency  int64

	// 断点续查
	checkpoint *Checkpoint

	// 结果通道
	resultChan chan *StreamBatchResult

	// 进度回调
	progressCallback func(stats StreamBatchStats)

	// 单条结果回调
	resultCallback func(result *StreamBatchResult)

	// 启动时间
	startTime time.Time

	// 取消函数
	cancel context.CancelFunc
}

// Checkpoint 断点续查数据
type Checkpoint struct {
	// 批次ID
	BatchID string `json:"batch_id"`

	// 创建时间
	CreatedAt string `json:"created_at"`

	// 所有待查询域名
	AllDomains []string `json:"all_domains"`

	// 已完成的域名
	CompletedDomains map[string]bool `json:"completed_domains"`

	// 已完成域名的结果（序列化）
	Results map[string]*CheckpointResult `json:"results"`

	// 统计信息
	TotalTasks   int64 `json:"total_tasks"`
	SuccessCount int64 `json:"success_count"`
	FailureCount int64 `json:"failure_count"`
}

// CheckpointResult 断点续查中的结果
type CheckpointResult struct {
	RawResponse string `json:"raw_response,omitempty"`
	Error       string `json:"error,omitempty"`
	Latency     int64  `json:"latency"`
	RetryCount  int    `json:"retry_count"`
	FromCache   bool   `json:"from_cache"`
}

// NewStreamBatchProcessor 创建流式批量查询处理器
func NewStreamBatchProcessor(config StreamBatchConfig) *StreamBatchProcessor {
	if config.Concurrency <= 0 {
		config.Concurrency = 5
	}
	if config.Timeout <= 0 {
		config.Timeout = 10
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.CheckpointInterval <= 0 {
		config.CheckpointInterval = 10
	}

	return &StreamBatchProcessor{
		config:      config,
		resultChan:  make(chan *StreamBatchResult, config.Concurrency*2),
		startTime:   time.Now(),
	}
}

// OnProgress 设置进度回调
func (p *StreamBatchProcessor) OnProgress(callback func(stats StreamBatchStats)) {
	p.progressCallback = callback
}

// OnResult 设置单条结果回调
func (p *StreamBatchProcessor) OnResult(callback func(result *StreamBatchResult)) {
	p.resultCallback = callback
}

// Results 返回结果通道
func (p *StreamBatchProcessor) Results() <-chan *StreamBatchResult {
	return p.resultChan
}

// Process 执行流式批量查询
func (p *StreamBatchProcessor) Process(ctx context.Context, domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("域名列表不能为空")
	}

	// 创建带取消的上下文。注意：cancel 不能用 defer 在 Process 返回时调用，
	// 否则会立即取消 ctx，导致后台 worker 检测到 ctx.Done() 后退出而不产出结果。
	// cancel 在所有 worker 完成、resultChan 关闭后再调用（见下方 wg.Wait goroutine）。
	ctx, p.cancel = context.WithCancel(ctx)

	p.startTime = time.Now()
	p.totalTasks = int64(len(domains))

	// 加载断点续查数据
	pendingDomains := domains
	if p.config.CheckpointFile != "" {
		loaded, err := p.loadCheckpoint()
		if err == nil && loaded != nil {
			p.checkpoint = loaded
			pendingDomains = p.getPendingDomains(domains)
			p.completed = int64(len(loaded.CompletedDomains))
			p.successCount = loaded.SuccessCount
			p.failureCount = loaded.FailureCount
			logrus.Infof("从断点续查恢复: 已完成 %d/%d, 待处理 %d",
				p.completed, p.totalTasks, len(pendingDomains))
		} else {
			p.checkpoint = p.newCheckpoint(domains)
		}
	} else {
		p.checkpoint = p.newCheckpoint(domains)
	}

	if len(pendingDomains) == 0 {
		logrus.Info("所有域名已完成查询")
		close(p.resultChan)
		if p.cancel != nil {
			p.cancel()
		}
		return nil
	}

	// 创建工作池
	domainChan := make(chan string, len(pendingDomains))
	for _, d := range pendingDomains {
		domainChan <- d
	}
	close(domainChan)

	// 启动worker
	var wg sync.WaitGroup
	for i := 0; i < p.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.worker(ctx, domainChan)
		}()
	}

	// 等待所有worker完成，关闭结果通道并取消上下文
	go func() {
		wg.Wait()
		close(p.resultChan)
		if p.cancel != nil {
			p.cancel()
		}
	}()

	return nil
}

// Cancel 取消批量查询
func (p *StreamBatchProcessor) Cancel() {
	if p.cancel != nil {
		p.cancel()
	}
	// 保存断点
	if p.config.CheckpointFile != "" {
		p.saveCheckpoint()
	}
}

// GetStats 获取当前统计信息
func (p *StreamBatchProcessor) GetStats() StreamBatchStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	completed := atomic.LoadInt64(&p.completed)
	success := atomic.LoadInt64(&p.successCount)
	failure := atomic.LoadInt64(&p.failureCount)

	elapsed := time.Since(p.startTime)
	var avgLatency int64
	if completed > 0 {
		avgLatency = p.totalLatency / completed
	}

	var remaining time.Duration
	if completed > 0 {
		remainingDomainCount := p.totalTasks - completed
		avgPerDomain := elapsed / time.Duration(completed)
		remaining = avgPerDomain * time.Duration(remainingDomainCount)
	}

	return StreamBatchStats{
		TotalTasks:         p.totalTasks,
		Completed:          completed,
		SuccessCount:       success,
		FailureCount:       failure,
		CacheHits:          atomic.LoadInt64(&p.cacheHits),
		AvgLatency:         avgLatency,
		Elapsed:            elapsed,
		EstimatedRemaining: remaining,
	}
}

// worker 工作协程
func (p *StreamBatchProcessor) worker(ctx context.Context, domainChan <-chan string) {
	for domain := range domainChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 查询延迟（限速）
		if p.config.QueryDelay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(p.config.QueryDelay) * time.Millisecond):
			}
		}

		result := p.queryDomain(ctx, domain)

		// 发送结果
		select {
		case p.resultChan <- result:
		case <-ctx.Done():
			return
		}

		// 更新统计
		atomic.AddInt64(&p.completed, 1)
		if result.Error == nil {
			atomic.AddInt64(&p.successCount, 1)
		} else {
			atomic.AddInt64(&p.failureCount, 1)
		}
		atomic.AddInt64(&p.totalLatency, result.Latency)

		// 更新断点
		if p.checkpoint != nil {
			p.mu.Lock()
			p.checkpoint.CompletedDomains[domain] = true
			cr := &CheckpointResult{
				Latency:    result.Latency,
				RetryCount: result.RetryCount,
				FromCache:  result.FromCache,
			}
			if result.Error != nil {
				cr.Error = result.Error.Error()
			}
			if result.RawResponse != "" {
				cr.RawResponse = result.RawResponse
			}
			p.checkpoint.Results[domain] = cr
			p.checkpoint.SuccessCount = atomic.LoadInt64(&p.successCount)
			p.checkpoint.FailureCount = atomic.LoadInt64(&p.failureCount)
			p.mu.Unlock()
		}

		// 定期保存断点
		if p.config.CheckpointFile != "" && atomic.LoadInt64(&p.completed)%int64(p.config.CheckpointInterval) == 0 {
			p.saveCheckpoint()
		}

		// 回调
		if p.resultCallback != nil {
			p.resultCallback(result)
		}
		if p.progressCallback != nil {
			p.progressCallback(p.GetStats())
		}
	}
}

// queryDomain 查询单个域名
func (p *StreamBatchProcessor) queryDomain(ctx context.Context, domain string) *StreamBatchResult {
	startTime := time.Now()
	result := &StreamBatchResult{
		Domain: domain,
	}

	opts := &QueryOptions{
		Domain:       domain,
		Timeout:      p.config.Timeout,
		MaxRetries:   p.config.MaxRetries,
		IntervalMils: p.config.RetryInterval,
		UseProxy:     p.config.UseProxy,
	}

	queryResult, err := ExecuteQueryWithResultContext(ctx, opts)
	result.Latency = time.Since(startTime).Milliseconds()

	if err != nil {
		result.Error = err
		return result
	}

	result.RawResponse = queryResult.RawResponse
	result.RetryCount = queryResult.RetryCount
	if queryResult.Info != nil {
		info := *queryResult.Info
		result.Info = &info
	}

	return result
}

// newCheckpoint 创建新的断点数据
func (p *StreamBatchProcessor) newCheckpoint(domains []string) *Checkpoint {
	return &Checkpoint{
		BatchID:          fmt.Sprintf("batch-%d", time.Now().Unix()),
		CreatedAt:        time.Now().Format(time.RFC3339),
		AllDomains:       domains,
		CompletedDomains: make(map[string]bool),
		Results:          make(map[string]*CheckpointResult),
	}
}

// getPendingDomains 获取待处理的域名列表
func (p *StreamBatchProcessor) getPendingDomains(domains []string) []string {
	var pending []string
	for _, d := range domains {
		if !p.checkpoint.CompletedDomains[d] {
			pending = append(pending, d)
		}
	}
	return pending
}

// loadCheckpoint 加载断点数据
func (p *StreamBatchProcessor) loadCheckpoint() (*Checkpoint, error) {
	if p.config.CheckpointFile == "" {
		return nil, fmt.Errorf("未配置断点文件")
	}

	data, err := os.ReadFile(p.config.CheckpointFile)
	if err != nil {
		return nil, err
	}

	checkpoint := &Checkpoint{}
	if err := json.Unmarshal(data, checkpoint); err != nil {
		return nil, err
	}

	return checkpoint, nil
}

// saveCheckpoint 保存断点数据
func (p *StreamBatchProcessor) saveCheckpoint() error {
	if p.config.CheckpointFile == "" || p.checkpoint == nil {
		return nil
	}

	p.mu.RLock()
	data, err := json.MarshalIndent(p.checkpoint, "", "  ")
	p.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("序列化断点数据失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(p.config.CheckpointFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建断点目录失败: %w", err)
	}

	// 原子写入：先写临时文件再重命名
	tmpFile := p.config.CheckpointFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("写入断点临时文件失败: %w", err)
	}

	if err := os.Rename(tmpFile, p.config.CheckpointFile); err != nil {
		return fmt.Errorf("重命名断点文件失败: %w", err)
	}

	return nil
}

// LoadCheckpointFromFile 从文件加载断点续查数据（外部API）
func LoadCheckpointFromFile(filePath string) (*Checkpoint, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取断点文件失败: %w", err)
	}

	checkpoint := &Checkpoint{}
	if err := json.Unmarshal(data, checkpoint); err != nil {
		return nil, fmt.Errorf("解析断点数据失败: %w", err)
	}

	return checkpoint, nil
}

// ResumeFromCheckpoint 从断点恢复批量查询
func ResumeFromCheckpoint(ctx context.Context, config StreamBatchConfig) (*StreamBatchProcessor, error) {
	if config.CheckpointFile == "" {
		return nil, fmt.Errorf("必须指定断点文件路径")
	}

	checkpoint, err := LoadCheckpointFromFile(config.CheckpointFile)
	if err != nil {
		return nil, err
	}

	processor := NewStreamBatchProcessor(config)
	processor.checkpoint = checkpoint

	// 获取待处理域名
	pending := processor.getPendingDomains(checkpoint.AllDomains)
	if len(pending) == 0 {
		return processor, nil
	}

	// 启动处理
	if err := processor.Process(ctx, pending); err != nil {
		return nil, err
	}

	return processor, nil
}

// CollectResults 收集所有结果（阻塞直到完成）
func CollectResults(resultChan <-chan *StreamBatchResult) []*StreamBatchResult {
	var results []*StreamBatchResult
	for result := range resultChan {
		results = append(results, result)
	}
	return results
}
