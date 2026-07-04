package whois

import (
	"container/heap"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/sirupsen/logrus"
)

// QueryOptions 定义WHOIS查询的配置选项
type QueryOptions struct {
	// Domain 要查询的域名
	Domain string `json:"domain"`

	// IntervalMils 重试间隔（毫秒），默认为1000ms
	IntervalMils int `json:"interval_mils,omitempty"`

	// MaxRetries 最大重试次数，默认为5次
	MaxRetries int `json:"max_retries,omitempty"`

	// UseProxy 是否使用代理（如果已配置）
	UseProxy bool `json:"use_proxy,omitempty"`

	// Priority 查询优先级（1-10，数字越小优先级越高）
	Priority int `json:"priority,omitempty"`

	// Timeout 查询超时时间（秒）
	Timeout int `json:"timeout,omitempty"`

	// ValidateResult 是否验证查询结果
	ValidateResult bool `json:"validate_result,omitempty"`

	// RequiredFields 必需的字段列表
	RequiredFields []string `json:"required_fields,omitempty"`

	// FollowReferral 是否跟随WHOIS引导查询，默认为true
	FollowReferral bool `json:"follow_referral,omitempty"`

	// MaxReferrals 最大引导查询次数，默认为3
	MaxReferrals int `json:"max_referrals,omitempty"`
}

// QueryResult WHOIS查询结果
type QueryResult struct {
	// 解析后的WHOIS信息
	Info *whoisparser.WhoisInfo `json:"info"`

	// 原始响应
	RawResponse string `json:"raw_response"`

	// 查询时间
	QueryTime time.Time `json:"query_time"`

	// 查询耗时（毫秒）
	Latency int64 `json:"latency"`

	// 使用的服务器
	Server string `json:"server"`

	// 使用的代理
	UsedProxy bool `json:"used_proxy"`

	// 重试次数
	RetryCount int `json:"retry_count"`

	// 验证结果
	ValidationResult *ValidationResult `json:"validation_result,omitempty"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	// 是否通过验证
	Valid bool `json:"valid"`

	// 缺失的必需字段
	MissingFields []string `json:"missing_fields,omitempty"`

	// 验证错误信息
	Errors []string `json:"errors,omitempty"`
}

// QueryAggregator 查询结果聚合器
type QueryAggregator struct {
	mu sync.RWMutex

	// 查询结果映射
	results map[string]*QueryResult

	// 查询队列（按优先级排序）
	queue PriorityQueue

	// 并发控制
	concurrency    int
	semaphore      chan struct{}
	progressCallback ProgressCallback

	// 统计信息
	stats QueryStats
}

// QueryStats 查询统计信息
type QueryStats struct {
	// 总查询次数
	TotalQueries int64

	// 成功查询次数
	SuccessfulQueries int64

	// 失败查询次数
	FailedQueries int64

	// 平均查询延迟
	AvgLatency int64

	// 最大查询延迟
	MaxLatency int64

	// 最小查询延迟
	MinLatency int64

	// 验证失败次数
	ValidationFailures int64
}

// BatchResult 批量查询结果
type BatchResult struct {
	// 成功结果映射
	Results map[string]*QueryResult `json:"results"`

	// 失败结果映射
	Errors map[string]error `json:"errors"`

	// 统计信息
	Stats QueryStats `json:"stats"`
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(completed int, total int, domain string, result *QueryResult, err error)

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	// 并发数
	Concurrency int

	// 进度回调
	ProgressCallback ProgressCallback
}

// PriorityQueue 优先级队列
type PriorityQueue []*QueryTask

// QueryTask 查询任务
type QueryTask struct {
	Domain   string
	Options  *QueryOptions
	Priority int
	Index    int
}

// GetIntervalMilsOrDefault 返回重试间隔，如未设置则使用默认值
func (q *QueryOptions) GetIntervalMilsOrDefault() int {
	if q.IntervalMils <= 0 {
		q.IntervalMils = 1000
	}
	return q.IntervalMils
}

// GetMaxRetriesOrDefault 返回最大重试次数，如未设置则使用默认值
func (q *QueryOptions) GetMaxRetriesOrDefault() int {
	if q.MaxRetries <= 0 {
		q.MaxRetries = 5
	}
	return q.MaxRetries
}

// GetMaxReferralsOrDefault 返回最大引导查询次数，如未设置则使用默认值
func (q *QueryOptions) GetMaxReferralsOrDefault() int {
	if q.MaxReferrals <= 0 {
		q.MaxReferrals = 3
	}
	return q.MaxReferrals
}

// NewQueryAggregator 创建新的查询聚合器
func NewQueryAggregator(config AggregatorConfig) *QueryAggregator {
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	return &QueryAggregator{
		results:         make(map[string]*QueryResult),
		queue:           make(PriorityQueue, 0),
		concurrency:     concurrency,
		semaphore:       make(chan struct{}, concurrency),
		progressCallback: config.ProgressCallback,
	}
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	wrapped := CheckError(err)
	return wrapped.IsRetryable()
}

// ExecuteQuery 执行WHOIS查询，返回解析后的WHOIS信息
// 这个函数会自动处理国际化域名，错误重试以及WHOIS响应解析
func ExecuteQuery(q *QueryOptions) (*whoisparser.WhoisInfo, error) {
	return ExecuteQueryWithContext(context.Background(), q)
}

// ExecuteQueryWithResult 执行WHOIS查询，返回完整的查询结果
func ExecuteQueryWithResult(q *QueryOptions) (*QueryResult, error) {
	return ExecuteQueryWithResultContext(context.Background(), q)
}

// ExecuteQueryWithContext 使用上下文执行WHOIS查询，返回解析后的WHOIS信息
func ExecuteQueryWithContext(ctx context.Context, q *QueryOptions) (*whoisparser.WhoisInfo, error) {
	result, err := ExecuteQueryWithResultContext(ctx, q)
	if err != nil {
		return nil, err
	}
	return result.Info, nil
}

// ExecuteQueryWithResultContext 使用上下文执行WHOIS查询，返回完整的查询结果
func ExecuteQueryWithResultContext(ctx context.Context, q *QueryOptions) (*QueryResult, error) {
	if q == nil {
		return nil, fmt.Errorf("查询选项不能为空")
	}

	if q.Domain == "" {
		return nil, NewWhoisError(ErrDomainEmpty, "域名不能为空", nil)
	}

	// 设置默认值
	if q.Timeout <= 0 {
		q.Timeout = 10
	}

	maxRetries := q.GetMaxRetriesOrDefault()
	interval := q.GetIntervalMilsOrDefault()

	startTime := time.Now()
	result := &QueryResult{
		QueryTime: startTime,
		UsedProxy: q.UseProxy,
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return nil, NewWhoisError(ErrQueryTimeout, "查询被取消", ctx.Err())
		}

		if attempt > 0 {
			logrus.Infof("重试WHOIS查询 [%s]，第%d次，等待%dms", q.Domain, attempt, interval)
			select {
			case <-ctx.Done():
				return nil, NewWhoisError(ErrQueryTimeout, "查询被取消", ctx.Err())
			case <-time.After(time.Duration(interval) * time.Millisecond):
			}
		}
		result.RetryCount = attempt

		// 获取WHOIS服务器
		server, err := GetServerManager().GetWhoisServer(q.Domain)
		if err != nil {
			lastErr = NewWhoisError(ErrServerNotFound, "获取WHOIS服务器失败", err)
			if !isRetryableError(lastErr) {
				return nil, lastErr
			}
			continue
		}
		result.Server = server

		// 执行查询（走全局 WhoisQueryProvider，默认为 likexian 实现，可注入自定义）
		rawResponse, err := GetWhoisQueryProvider().Query(ctx, q.Domain, server, q.UseProxy)
		if err != nil {
			lastErr = err
			if isRetryableError(err) {
				continue
			}
			return nil, err
		}
		result.RawResponse = rawResponse

		// 解析WHOIS信息（走全局 provider 的 Parse，默认为 whois-parser）
		info, err := GetWhoisQueryProvider().Parse(rawResponse)
		if err != nil {
			// 解析错误一般不可重试
			return nil, NewWhoisError(ErrParseFailed, "WHOIS信息解析失败", err)
		}
		result.Info = &info

		// 计算查询延迟
		result.Latency = time.Since(startTime).Milliseconds()

		// 验证结果
		if q.ValidateResult {
			result.ValidationResult = validateQueryResult(result, q.RequiredFields)
			if !result.ValidationResult.Valid {
				return result, NewWhoisError(ErrValidationFailed,
					fmt.Sprintf("查询结果验证失败: %v", result.ValidationResult.Errors), nil)
			}
		}

		return result, nil
	}

	return nil, NewWhoisError(ErrServerConnectFailed,
		fmt.Sprintf("查询失败，已重试%d次", maxRetries), lastErr)
}

// executeQueryWithTimeout 带超时的查询执行
func executeQueryWithTimeout(ctx context.Context, q *QueryOptions, server string) (string, error) {
	// 如果上下文已有截止时间，使用它；否则应用QueryOptions.Timeout
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(q.Timeout)*time.Second)
		defer cancel()
	}

	type queryResult struct {
		response string
		err      error
	}

	resultChan := make(chan queryResult, 1)
	go func() {
		if q.UseProxy {
			response, err := DirectWhoisWithContext(ctx, q.Domain)
			resultChan <- queryResult{response, err}
		} else {
			response, err := whois.Whois(q.Domain)
			resultChan <- queryResult{response, err}
		}
	}()

	select {
	case <-ctx.Done():
		return "", NewWhoisError(ErrQueryTimeout, "查询超时", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return "", result.err
		}
		return result.response, nil
	}
}

// validateQueryResult 验证查询结果
func validateQueryResult(result *QueryResult, requiredFields []string) *ValidationResult {
	validation := &ValidationResult{
		Valid:         true,
		MissingFields: make([]string, 0),
		Errors:        make([]string, 0),
	}

	if result.Info == nil {
		validation.Valid = false
		validation.Errors = append(validation.Errors, "WHOIS信息为空")
		return validation
	}

	// 验证必需字段
	if len(requiredFields) > 0 {
		missingFields := validateRequiredFields(result.Info, requiredFields)
		if len(missingFields) > 0 {
			validation.Valid = false
			validation.MissingFields = missingFields
			validation.Errors = append(validation.Errors, "缺少必需字段")
		}
	}

	// 验证基本信息
	if result.Info.Domain == nil {
		validation.Valid = false
		validation.Errors = append(validation.Errors, "域名信息为空")
	} else {
		// 验证域名信息完整性
		if result.Info.Domain.Domain == "" {
			validation.Errors = append(validation.Errors, "域名为空")
		}
		if result.Info.Domain.CreatedDate == "" {
			validation.Errors = append(validation.Errors, "创建日期为空")
		}
		if result.Info.Domain.ExpirationDate == "" {
			validation.Errors = append(validation.Errors, "过期日期为空")
		}
	}

	// 验证注册商信息
	if result.Info.Registrar == nil || result.Info.Registrar.Name == "" {
		validation.Valid = false
		validation.Errors = append(validation.Errors, "注册商信息不完整")
	}

	return validation
}

// validateRequiredFields 验证必需字段
func validateRequiredFields(info *whoisparser.WhoisInfo, fields []string) []string {
	missing := make([]string, 0)
	val := reflect.ValueOf(info).Elem()

	for _, field := range fields {
		found := false
		// 遍历所有结构体字段
		for i := 0; i < val.NumField(); i++ {
			if strings.EqualFold(val.Type().Field(i).Name, field) {
				fieldVal := val.Field(i)
				if !fieldVal.IsZero() {
					found = true
					break
				}
			}
		}
		if !found {
			missing = append(missing, field)
		}
	}

	return missing
}

// SetProgressCallback 设置进度回调函数
func (qa *QueryAggregator) SetProgressCallback(callback ProgressCallback) {
	qa.mu.Lock()
	defer qa.mu.Unlock()
	qa.progressCallback = callback
}

// AddQuery 添加查询任务到队列
func (qa *QueryAggregator) AddQuery(domain string, options *QueryOptions) {
	qa.mu.Lock()
	defer qa.mu.Unlock()

	task := &QueryTask{
		Domain:   domain,
		Options:  options,
		Priority: options.Priority,
	}

	heap.Push(&qa.queue, task)
}

// ExecuteAll 执行所有查询任务
func (qa *QueryAggregator) ExecuteAll() *BatchResult {
	batchResult := &BatchResult{
		Results: make(map[string]*QueryResult),
		Errors:  make(map[string]error),
	}

	qa.mu.RLock()
	total := qa.queue.Len()
	callback := qa.progressCallback
	qa.mu.RUnlock()

	var completedVal int32
	var wg sync.WaitGroup

	for qa.queue.Len() > 0 {
		qa.semaphore <- struct{}{} // 获取信号量
		wg.Add(1)

		task := heap.Pop(&qa.queue).(*QueryTask)
		go func(t *QueryTask) {
			defer wg.Done()
			defer func() { <-qa.semaphore }() // 释放信号量

			result, err := ExecuteQueryWithResult(t.Options)

			qa.mu.Lock()
			qa.stats.TotalQueries++
			currentCompleted := atomic.AddInt32(&completedVal, 1)
			if err != nil {
				batchResult.Errors[t.Domain] = err
				qa.stats.FailedQueries++
			} else {
				batchResult.Results[t.Domain] = result
				qa.stats.SuccessfulQueries++
				qa.updateStats(result)
			}
			qa.mu.Unlock()

			if callback != nil {
				callback(int(currentCompleted), total, t.Domain, result, err)
			}
		}(task)
	}

	wg.Wait()
	batchResult.Stats = qa.stats
	return batchResult
}

// updateStats 更新统计信息
func (qa *QueryAggregator) updateStats(result *QueryResult) {
	qa.stats.TotalQueries++
	if result.ValidationResult != nil && !result.ValidationResult.Valid {
		qa.stats.ValidationFailures++
	}

	// 更新延迟统计
	if qa.stats.MinLatency == 0 || result.Latency < qa.stats.MinLatency {
		qa.stats.MinLatency = result.Latency
	}
	if result.Latency > qa.stats.MaxLatency {
		qa.stats.MaxLatency = result.Latency
	}

	// 更新平均延迟
	qa.stats.AvgLatency = (qa.stats.AvgLatency*(qa.stats.TotalQueries-1) + result.Latency) / qa.stats.TotalQueries
}

// GetStats 获取统计信息
func (qa *QueryAggregator) GetStats() QueryStats {
	qa.mu.RLock()
	defer qa.mu.RUnlock()
	return qa.stats
}

// PushTask 类型安全地添加查询任务到优先级队列
func (pq *PriorityQueue) PushTask(task *QueryTask) {
	heap.Push(pq, task)
}

// PopTask 类型安全地从优先级队列弹出查询任务
func (pq *PriorityQueue) PopTask() *QueryTask {
	return heap.Pop(pq).(*QueryTask)
}

// heap.Interface implementation for PriorityQueue
func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	task := x.(*QueryTask)
	task.Index = n
	*pq = append(*pq, task)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.Index = -1
	*pq = old[0 : n-1]
	return task
}

// 为了保持向后兼容性，保留原来的API入口
// 建议新代码使用QueryOptions和ExecuteQuery
type Query struct {
	Domain       string `json:"domain,omitempty"`
	IntervalMils int    `json:"interval_mils,omitempty"`
	UseProxy     bool   `json:"use_proxy,omitempty"`
}

// Execute 执行WHOIS查询（兼容旧API）
func Execute(query *Query) (*whoisparser.WhoisInfo, error) {
	return ExecuteQuery(&QueryOptions{
		Domain:       query.Domain,
		IntervalMils: query.IntervalMils,
		UseProxy:     query.UseProxy,
	})
}
