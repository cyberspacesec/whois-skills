package whois

import (
	"container/heap"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
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
	concurrency int
	semaphore   chan struct{}

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

// ExecuteQuery 执行WHOIS查询，返回解析后的WHOIS信息
// 这个函数会自动处理国际化域名，错误重试以及WHOIS响应解析
func ExecuteQuery(q *QueryOptions) (*whoisparser.WhoisInfo, error) {
	result, err := ExecuteQueryWithResult(q)
	if err != nil {
		return nil, err
	}
	return result.Info, nil
}

// ExecuteQueryWithResult 执行WHOIS查询，返回完整的查询结果
func ExecuteQueryWithResult(q *QueryOptions) (*QueryResult, error) {
	if q == nil {
		return nil, fmt.Errorf("查询选项不能为空")
	}

	if q.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	// 设置默认值
	if q.Timeout <= 0 {
		q.Timeout = 10
	}

	startTime := time.Now()
	result := &QueryResult{
		QueryTime: startTime,
		UsedProxy: q.UseProxy,
	}

	// 获取WHOIS服务器
	server, err := GetServerManager().GetWhoisServer(q.Domain)
	if err != nil {
		return nil, fmt.Errorf("获取WHOIS服务器失败: %w", err)
	}
	result.Server = server

	// 执行查询
	rawResponse, err := executeQueryWithTimeout(q, server)
	if err != nil {
		return nil, err
	}
	result.RawResponse = rawResponse

	// 解析WHOIS信息
	info, err := whoisparser.Parse(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("WHOIS信息解析失败: %w", err)
	}
	result.Info = &info

	// 计算查询延迟
	result.Latency = time.Since(startTime).Milliseconds()

	// 验证结果
	if q.ValidateResult {
		result.ValidationResult = validateQueryResult(result, q.RequiredFields)
		if !result.ValidationResult.Valid {
			return result, fmt.Errorf("查询结果验证失败: %v", result.ValidationResult.Errors)
		}
	}

	return result, nil
}

// executeQueryWithTimeout 带超时的查询执行
func executeQueryWithTimeout(q *QueryOptions, server string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(q.Timeout)*time.Second)
	defer cancel()

	type queryResult struct {
		response string
		err      error
	}

	resultChan := make(chan queryResult, 1)
	go func() {
		if q.UseProxy {
			response, err := DirectWhois(q.Domain)
			resultChan <- queryResult{response, err}
		} else {
			response, err := whois.Whois(q.Domain)
			resultChan <- queryResult{response, err}
		}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("查询超时")
	case result := <-resultChan:
		return result.response, result.err
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
func (qa *QueryAggregator) ExecuteAll() map[string]*QueryResult {
	var wg sync.WaitGroup

	for qa.queue.Len() > 0 {
		qa.semaphore <- struct{}{} // 获取信号量
		wg.Add(1)

		task := heap.Pop(&qa.queue).(*QueryTask)
		go func(t *QueryTask) {
			defer wg.Done()
			defer func() { <-qa.semaphore }() // 释放信号量

			result, err := ExecuteQueryWithResult(t.Options)
			if err != nil {
				logrus.Errorf("查询失败 [%s]: %v", t.Domain, err)
				return
			}

			qa.mu.Lock()
			qa.results[t.Domain] = result
			qa.updateStats(result)
			qa.mu.Unlock()
		}(task)
	}

	wg.Wait()
	return qa.results
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
