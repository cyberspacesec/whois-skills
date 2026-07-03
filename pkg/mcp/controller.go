package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/cyberspacesec/whois-skills/pkg/whois"
	"github.com/google/uuid"
	whoisparser "github.com/likexian/whois-parser"
)

// Controller 是MCP控制器，管理所有MCP协议操作
type Controller struct {
	store *RequestStore
}

// NewController 创建一个新的MCP控制器
func NewController() *Controller {
	return &Controller{
		store: NewRequestStore(),
	}
}

// ============================================================
// MCP 任务管理 (原有功能)
// ============================================================

// TaskInput 表示创建任务的输入
type TaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// RequestPlanningInput 表示请求规划的输入
type RequestPlanningInput struct {
	OriginalRequest string      `json:"originalRequest"`
	Tasks           []TaskInput `json:"tasks"`
	SplitDetails    string      `json:"splitDetails,omitempty"`
}

// RequestPlanningOutput 表示请求规划的输出
type RequestPlanningOutput struct {
	RequestID string              `json:"requestId"`
	Tasks     []map[string]string `json:"tasks"`
	Message   string              `json:"message"`
}

// PlanRequest 规划一个新请求及其任务
func (c *Controller) PlanRequest(input RequestPlanningInput) (*RequestPlanningOutput, error) {
	if input.OriginalRequest == "" {
		return nil, fmt.Errorf("原始请求不能为空")
	}

	if len(input.Tasks) == 0 {
		return nil, fmt.Errorf("任务列表不能为空")
	}

	// 创建一个新请求
	requestID := uuid.New().String()
	request := &Request{
		ID:              requestID,
		OriginalRequest: input.OriginalRequest,
		SplitDetails:    input.SplitDetails,
		Status:          RequestStatusPending,
		Tasks:           make([]*Task, 0, len(input.Tasks)),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 创建任务
	taskSummary := make([]map[string]string, 0, len(input.Tasks))

	for _, taskInput := range input.Tasks {
		taskID := uuid.New().String()

		task := &Task{
			ID:          taskID,
			Title:       taskInput.Title,
			Description: taskInput.Description,
			Status:      TaskStatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		request.Tasks = append(request.Tasks, task)

		taskSummary = append(taskSummary, map[string]string{
			"id":          taskID,
			"title":       taskInput.Title,
			"description": taskInput.Description,
		})
	}

	// 保存请求
	c.store.AddRequest(request)

	return &RequestPlanningOutput{
		RequestID: requestID,
		Tasks:     taskSummary,
		Message:   fmt.Sprintf("已创建请求并添加 %d 个任务", len(input.Tasks)),
	}, nil
}

// GetNextTaskInput 表示获取下一个任务的输入
type GetNextTaskInput struct {
	RequestID string `json:"requestId"`
}

// TaskOutput 表示任务的输出格式
type TaskOutput struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// GetNextTaskOutput 表示获取下一个任务的输出
type GetNextTaskOutput struct {
	RequestID    string      `json:"requestId"`
	AllTasksDone bool        `json:"all_tasks_done"`
	Task         *TaskOutput `json:"task,omitempty"`
	Progress     string      `json:"progress"`
}

// GetNextTask 获取请求的下一个待处理任务
func (c *Controller) GetNextTask(input GetNextTaskInput) (*GetNextTaskOutput, error) {
	if input.RequestID == "" {
		return nil, fmt.Errorf("请求ID不能为空")
	}

	// 获取请求
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	// 获取下一个待处理的任务
	nextTask, err := c.store.GetNextPendingTask(input.RequestID)
	if err != nil {
		return nil, err
	}

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	if nextTask == nil {
		// 所有任务已完成
		return &GetNextTaskOutput{
			RequestID:    input.RequestID,
			AllTasksDone: true,
			Progress:     progressInfo,
		}, nil
	}

	// 返回下一个任务
	return &GetNextTaskOutput{
		RequestID:    input.RequestID,
		AllTasksDone: false,
		Task: &TaskOutput{
			ID:          nextTask.ID,
			Title:       nextTask.Title,
			Description: nextTask.Description,
			Status:      string(nextTask.Status),
		},
		Progress: progressInfo,
	}, nil
}

// MarkTaskDoneInput 表示标记任务完成的输入
type MarkTaskDoneInput struct {
	RequestID        string `json:"requestId"`
	TaskID           string `json:"taskId"`
	CompletedDetails string `json:"completedDetails,omitempty"`
}

// MarkTaskDoneOutput 表示标记任务完成的输出
type MarkTaskDoneOutput struct {
	RequestID string `json:"requestId"`
	TaskID    string `json:"taskId"`
	Message   string `json:"message"`
	Progress  string `json:"progress"`
}

// MarkTaskDone 标记任务为已完成
func (c *Controller) MarkTaskDone(input MarkTaskDoneInput) (*MarkTaskDoneOutput, error) {
	if input.RequestID == "" || input.TaskID == "" {
		return nil, fmt.Errorf("请求ID和任务ID不能为空")
	}

	// 获取请求和任务
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	task, err := c.store.GetTask(input.TaskID)
	if err != nil {
		return nil, err
	}

	// 检查任务是否属于请求
	belongsToRequest := false
	for _, t := range request.Tasks {
		if t.ID == input.TaskID {
			belongsToRequest = true
			break
		}
	}

	if !belongsToRequest {
		return nil, fmt.Errorf("任务不属于指定的请求")
	}

	// 标记任务为已完成
	err = c.store.UpdateTask(input.TaskID, TaskStatusDone, input.CompletedDetails)
	if err != nil {
		return nil, err
	}

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &MarkTaskDoneOutput{
		RequestID: input.RequestID,
		TaskID:    input.TaskID,
		Message:   fmt.Sprintf("任务 %s 已标记为完成", task.Title),
		Progress:  progressInfo,
	}, nil
}

// ApproveTaskInput 表示批准任务的输入
type ApproveTaskInput struct {
	RequestID string `json:"requestId"`
	TaskID    string `json:"taskId"`
}

// ApproveTaskOutput 表示批准任务的输出
type ApproveTaskOutput struct {
	RequestID string `json:"requestId"`
	TaskID    string `json:"taskId"`
	Message   string `json:"message"`
	Progress  string `json:"progress"`
}

// ApproveTaskCompletion 批准任务完成
func (c *Controller) ApproveTaskCompletion(input ApproveTaskInput) (*ApproveTaskOutput, error) {
	if input.RequestID == "" || input.TaskID == "" {
		return nil, fmt.Errorf("请求ID和任务ID不能为空")
	}

	// 获取请求和任务
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	task, err := c.store.GetTask(input.TaskID)
	if err != nil {
		return nil, err
	}

	// 检查任务是否已标记为完成
	if task.Status != TaskStatusDone {
		return nil, fmt.Errorf("任务必须先标记为完成才能批准")
	}

	// 检查任务是否属于请求
	belongsToRequest := false
	for _, t := range request.Tasks {
		if t.ID == input.TaskID {
			belongsToRequest = true
			break
		}
	}

	if !belongsToRequest {
		return nil, fmt.Errorf("任务不属于指定的请求")
	}

	// 标记任务为已批准
	err = c.store.UpdateTask(input.TaskID, TaskStatusApproved, "")
	if err != nil {
		return nil, err
	}

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &ApproveTaskOutput{
		RequestID: input.RequestID,
		TaskID:    input.TaskID,
		Message:   fmt.Sprintf("任务 %s 已批准完成", task.Title),
		Progress:  progressInfo,
	}, nil
}

// ApproveRequestInput 表示批准请求的输入
type ApproveRequestInput struct {
	RequestID string `json:"requestId"`
}

// ApproveRequestOutput 表示批准请求的输出
type ApproveRequestOutput struct {
	RequestID string `json:"requestId"`
	Message   string `json:"message"`
	Progress  string `json:"progress"`
}

// ApproveRequestCompletion 批准请求完成
func (c *Controller) ApproveRequestCompletion(input ApproveRequestInput) (*ApproveRequestOutput, error) {
	if input.RequestID == "" {
		return nil, fmt.Errorf("请求ID不能为空")
	}

	// 获取请求
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	// 检查是否所有任务都已批准
	for _, task := range request.Tasks {
		if task.Status != TaskStatusApproved {
			return nil, fmt.Errorf("所有任务必须先被批准才能完成请求")
		}
	}

	// 标记请求为已完成
	err = c.store.UpdateRequestStatus(input.RequestID, RequestStatusDone)
	if err != nil {
		return nil, err
	}

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &ApproveRequestOutput{
		RequestID: input.RequestID,
		Message:   "请求已完成并批准",
		Progress:  progressInfo,
	}, nil
}

// ListRequestsOutput 表示列出所有请求的输出
type ListRequestsOutput struct {
	Requests []RequestSummary `json:"requests"`
	Message  string           `json:"message"`
}

// RequestSummary 表示请求摘要
type RequestSummary struct {
	ID              string        `json:"id"`
	OriginalRequest string        `json:"original_request"`
	Status          RequestStatus `json:"status"`
	TaskCount       int           `json:"task_count"`
	CompletedTasks  int           `json:"completed_tasks"`
	CreatedAt       time.Time     `json:"created_at"`
}

// ListRequests 列出所有请求
func (c *Controller) ListRequests() (*ListRequestsOutput, error) {
	requests := c.store.GetAllRequests()

	summaries := make([]RequestSummary, 0, len(requests))
	for _, req := range requests {
		completed := 0
		for _, task := range req.Tasks {
			if task.Status == TaskStatusApproved {
				completed++
			}
		}

		summaries = append(summaries, RequestSummary{
			ID:              req.ID,
			OriginalRequest: req.OriginalRequest,
			Status:          req.Status,
			TaskCount:       len(req.Tasks),
			CompletedTasks:  completed,
			CreatedAt:       req.CreatedAt,
		})
	}

	return &ListRequestsOutput{
		Requests: summaries,
		Message:  fmt.Sprintf("共有 %d 个请求", len(summaries)),
	}, nil
}

// AddTasksInput 表示向请求添加任务的输入
type AddTasksInput struct {
	RequestID string      `json:"requestId"`
	Tasks     []TaskInput `json:"tasks"`
}

// AddTasksOutput 表示向请求添加任务的输出
type AddTasksOutput struct {
	RequestID string              `json:"requestId"`
	Tasks     []map[string]string `json:"tasks"`
	Message   string              `json:"message"`
	Progress  string              `json:"progress"`
}

// AddTasksToRequest 向请求添加新任务
func (c *Controller) AddTasksToRequest(input AddTasksInput) (*AddTasksOutput, error) {
	if input.RequestID == "" {
		return nil, fmt.Errorf("请求ID不能为空")
	}

	if len(input.Tasks) == 0 {
		return nil, fmt.Errorf("任务列表不能为空")
	}

	// 获取请求
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	// 如果请求已完成，无法添加任务
	if request.Status == RequestStatusDone {
		return nil, fmt.Errorf("无法向已完成的请求添加任务")
	}

	// 创建新任务
	newTasks := make([]*Task, 0, len(input.Tasks))
	taskSummary := make([]map[string]string, 0, len(input.Tasks))

	for _, taskInput := range input.Tasks {
		taskID := uuid.New().String()

		task := &Task{
			ID:          taskID,
			Title:       taskInput.Title,
			Description: taskInput.Description,
			Status:      TaskStatusPending,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		newTasks = append(newTasks, task)

		taskSummary = append(taskSummary, map[string]string{
			"id":          taskID,
			"title":       taskInput.Title,
			"description": taskInput.Description,
		})
	}

	// 将新任务添加到请求
	err = c.store.AddTasksToRequest(input.RequestID, newTasks)
	if err != nil {
		return nil, err
	}

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &AddTasksOutput{
		RequestID: input.RequestID,
		Tasks:     taskSummary,
		Message:   fmt.Sprintf("已向请求添加 %d 个新任务", len(input.Tasks)),
		Progress:  progressInfo,
	}, nil
}

// UpdateTaskInput 表示更新任务的输入
type UpdateTaskInput struct {
	RequestID   string `json:"requestId"`
	TaskID      string `json:"taskId"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateTaskOutput 表示更新任务的输出
type UpdateTaskOutput struct {
	RequestID string `json:"requestId"`
	TaskID    string `json:"taskId"`
	Message   string `json:"message"`
	Progress  string `json:"progress"`
}

// UpdateTask 更新任务信息
func (c *Controller) UpdateTask(input UpdateTaskInput) (*UpdateTaskOutput, error) {
	if input.RequestID == "" || input.TaskID == "" {
		return nil, fmt.Errorf("请求ID和任务ID不能为空")
	}

	// 获取请求和任务
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	task, err := c.store.GetTask(input.TaskID)
	if err != nil {
		return nil, err
	}

	// 检查任务是否属于请求
	belongsToRequest := false
	for _, t := range request.Tasks {
		if t.ID == input.TaskID {
			belongsToRequest = true
			break
		}
	}

	if !belongsToRequest {
		return nil, fmt.Errorf("任务不属于指定的请求")
	}

	// 检查任务是否已完成或已批准
	if task.Status == TaskStatusDone || task.Status == TaskStatusApproved {
		return nil, fmt.Errorf("无法更新已完成或已批准的任务")
	}

	// 更新任务
	if input.Title != "" {
		task.Title = input.Title
	}

	if input.Description != "" {
		task.Description = input.Description
	}

	task.UpdatedAt = time.Now()

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &UpdateTaskOutput{
		RequestID: input.RequestID,
		TaskID:    input.TaskID,
		Message:   "任务信息已更新",
		Progress:  progressInfo,
	}, nil
}

// DeleteTaskInput 表示删除任务的输入
type DeleteTaskInput struct {
	RequestID string `json:"requestId"`
	TaskID    string `json:"taskId"`
}

// DeleteTaskOutput 表示删除任务的输出
type DeleteTaskOutput struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	Progress  string `json:"progress"`
}

// DeleteTask 删除任务
func (c *Controller) DeleteTask(input DeleteTaskInput) (*DeleteTaskOutput, error) {
	if input.RequestID == "" || input.TaskID == "" {
		return nil, fmt.Errorf("请求ID和任务ID不能为空")
	}

	// 获取请求
	request, err := c.store.GetRequest(input.RequestID)
	if err != nil {
		return nil, err
	}

	// 检查任务是否存在
	taskIndex := -1
	var taskToDelete *Task

	for i, task := range request.Tasks {
		if task.ID == input.TaskID {
			taskIndex = i
			taskToDelete = task
			break
		}
	}

	if taskIndex == -1 {
		return nil, fmt.Errorf("任务不存在于请求中")
	}

	// 检查任务是否已完成或已批准
	if taskToDelete.Status == TaskStatusDone || taskToDelete.Status == TaskStatusApproved {
		return nil, fmt.Errorf("无法删除已完成或已批准的任务")
	}

	// 删除任务
	request.Tasks = append(request.Tasks[:taskIndex], request.Tasks[taskIndex+1:]...)
	request.UpdatedAt = time.Now()

	// 从任务映射表中删除
	c.store.mu.Lock()
	delete(c.store.tasks, input.TaskID)
	c.store.mu.Unlock()

	// 生成进度信息
	progressInfo := c.generateProgressInfo(request)

	return &DeleteTaskOutput{
		RequestID: input.RequestID,
		Message:   fmt.Sprintf("任务 '%s' 已删除", taskToDelete.Title),
		Progress:  progressInfo,
	}, nil
}

// TaskDetailsInput 表示获取任务详情的输入
type TaskDetailsInput struct {
	TaskID string `json:"taskId"`
}

// TaskDetailsOutput 表示任务详情的输出
type TaskDetailsOutput struct {
	Task    *Task  `json:"task"`
	Message string `json:"message"`
}

// GetTaskDetails 获取任务的详细信息
func (c *Controller) GetTaskDetails(input TaskDetailsInput) (*TaskDetailsOutput, error) {
	if input.TaskID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}

	// 获取任务
	task, err := c.store.GetTask(input.TaskID)
	if err != nil {
		return nil, err
	}

	return &TaskDetailsOutput{
		Task:    task,
		Message: "任务详情获取成功",
	}, nil
}

// 内部辅助方法

// generateProgressInfo 生成请求的进度信息
func (c *Controller) generateProgressInfo(request *Request) string {
	total := len(request.Tasks)
	done := 0
	approved := 0

	for _, task := range request.Tasks {
		if task.Status == TaskStatusDone {
			done++
		} else if task.Status == TaskStatusApproved {
			approved++
			done++
		}
	}

	return fmt.Sprintf("进度: %d/%d 完成, %d/%d 已批准", done, total, approved, total)
}

// ============================================================
// WHOIS 查询能力 (新增)
// ============================================================

// WhoisQueryInput 域名WHOIS查询输入
type WhoisQueryInput struct {
	Domain         string   `json:"domain"`
	UseProxy       bool     `json:"use_proxy,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	MaxRetries     int      `json:"max_retries,omitempty"`
	ValidateResult bool     `json:"validate_result,omitempty"`
	RequiredFields []string `json:"required_fields,omitempty"`
}

// WhoisQueryOutput 域名WHOIS查询输出
type WhoisQueryOutput struct {
	Info      *whoisparser.WhoisInfo `json:"info"`
	Raw       string                 `json:"raw_response,omitempty"`
	Server    string                 `json:"server,omitempty"`
	Latency   int64                  `json:"latency,omitempty"`
	Retries   int                    `json:"retry_count,omitempty"`
	Valid     bool                   `json:"valid,omitempty"`
	Errors    []string               `json:"validation_errors,omitempty"`
}

// ExecuteWhoisQuery 执行WHOIS查询
func (c *Controller) ExecuteWhoisQuery(domain string) (*whoisparser.WhoisInfo, error) {
	queryOpts := &whois.Query{
		Domain: domain,
	}
	return whois.Execute(queryOpts)
}

// ExecuteWhoisQueryFull 执行完整WHOIS查询（新版API，返回更多信息）
func (c *Controller) ExecuteWhoisQueryFull(ctx context.Context, input WhoisQueryInput) (*WhoisQueryOutput, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	if input.Timeout <= 0 {
		input.Timeout = 10
	}

	result, err := whois.ExecuteQueryWithResultContext(ctx, &whois.QueryOptions{
		Domain:         input.Domain,
		UseProxy:       input.UseProxy,
		Timeout:        input.Timeout,
		MaxRetries:     input.MaxRetries,
		ValidateResult: input.ValidateResult,
		RequiredFields: input.RequiredFields,
		Priority:       1,
	})
	if err != nil {
		return nil, err
	}

	output := &WhoisQueryOutput{
		Info:    result.Info,
		Raw:     result.RawResponse,
		Server:  result.Server,
		Latency: result.Latency,
		Retries: result.RetryCount,
	}

	if result.ValidationResult != nil {
		output.Valid = result.ValidationResult.Valid
		output.Errors = result.ValidationResult.Errors
	}

	return output, nil
}

// IPQueryInput IP WHOIS查询输入
type IPQueryInput struct {
	IP       string `json:"ip"`
	Timeout  int    `json:"timeout,omitempty"`
	UseProxy bool   `json:"use_proxy,omitempty"`
}

// IPQueryOutput IP WHOIS查询输出
type IPQueryOutput struct {
	IP          string                 `json:"ip"`
	RawResponse string                 `json:"raw_response"`
	Server      string                 `json:"server"`
	Latency     int64                  `json:"latency"`
	Info        *whoisparser.WhoisInfo `json:"info,omitempty"`
}

// ExecuteIPWhoisQuery 执行IP WHOIS查询
func (c *Controller) ExecuteIPWhoisQuery(ctx context.Context, input IPQueryInput) (*IPQueryOutput, error) {
	if input.IP == "" {
		return nil, fmt.Errorf("IP地址不能为空")
	}

	result, err := whois.QueryIPWithContext(ctx, &whois.IPWhoisOptions{
		IP:       input.IP,
		Timeout:  input.Timeout,
		UseProxy: input.UseProxy,
	})
	if err != nil {
		return nil, err
	}

	return &IPQueryOutput{
		IP:          result.IP,
		RawResponse: result.RawResponse,
		Server:      result.Server,
		Latency:     result.Latency,
		Info:        result.Info,
	}, nil
}

// ASNQueryInput ASN查询输入
type ASNQueryInput struct {
	ASN             int    `json:"asn"`
	Source          string `json:"source,omitempty"` // radb, rdap, all
	Timeout         int    `json:"timeout,omitempty"`
	IncludePrefixes bool   `json:"include_prefixes,omitempty"`
	IncludeBGP      bool   `json:"include_bgp,omitempty"`
}

// ASNQueryOutput ASN查询输出
type ASNQueryOutput struct {
	ASN          int      `json:"asn"`
	Name         string   `json:"name,omitempty"`
	Organization string   `json:"organization,omitempty"`
	Country      string   `json:"country,omitempty"`
	RIR          string   `json:"rir,omitempty"`
	Description  string   `json:"description,omitempty"`
	IPv4Prefixes []string `json:"ipv4_prefixes,omitempty"`
	IPv6Prefixes []string `json:"ipv6_prefixes,omitempty"`
}

// ExecuteASNQuery 执行ASN查询
func (c *Controller) ExecuteASNQuery(ctx context.Context, input ASNQueryInput) (*ASNQueryOutput, error) {
	if input.ASN <= 0 {
		return nil, fmt.Errorf("ASN必须为正整数")
	}

	source := whois.ASNSourceAll
	switch input.Source {
	case "radb":
		source = whois.ASNSourceRADB
	case "rdap":
		source = whois.ASNSourceRDAP
	}

	result, err := whois.QueryASNWithContext(ctx, &whois.ASNQueryOptions{
		ASN:             input.ASN,
		Source:          source,
		Timeout:         input.Timeout,
		IncludePrefixes: input.IncludePrefixes,
		IncludeBGP:      input.IncludeBGP,
	})
	if err != nil {
		return nil, err
	}

	return &ASNQueryOutput{
		ASN:          result.ASN,
		Name:         result.Name,
		Organization: result.Organization,
		Country:      result.Country,
		RIR:          result.RIR,
		Description:  result.Description,
		IPv4Prefixes: result.IPv4Prefixes,
		IPv6Prefixes: result.IPv6Prefixes,
	}, nil
}

// RDAPQueryInput RDAP查询输入
type RDAPQueryInput struct {
	Type    string `json:"type"` // domain, ip, asn
	Target  string `json:"target"`
	Timeout int    `json:"timeout,omitempty"`
}

// RDAPQueryOutput RDAP查询输出
type RDAPQueryOutput struct {
	Type    string      `json:"type"`
	Target  string      `json:"target"`
	Result  interface{} `json:"result"`
	Server  string      `json:"server,omitempty"`
}

// ExecuteRDAPQuery 执行RDAP查询（统一入口）
func (c *Controller) ExecuteRDAPQuery(ctx context.Context, input RDAPQueryInput) (*RDAPQueryOutput, error) {
	if input.Type == "" || input.Target == "" {
		return nil, fmt.Errorf("type和target不能为空")
	}

	opts := &whois.RDAPQueryOptions{Timeout: input.Timeout}

	switch input.Type {
	case "domain":
		opts.Domain = input.Target
		result, err := whois.QueryRDAPWithContext(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &RDAPQueryOutput{
			Type:   "domain",
			Target: input.Target,
			Result: result,
			Server: result.Server,
		}, nil

	case "ip":
		opts.IP = input.Target
		result, err := whois.QueryRDAP_IPWithContext(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &RDAPQueryOutput{
			Type:   "ip",
			Target: input.Target,
			Result: result,
			Server: result.Server,
		}, nil

	case "asn":
		opts.ASN = input.Target
		result, err := whois.QueryRDAP_ASNWithContext(ctx, opts)
		if err != nil {
			return nil, err
		}
		return &RDAPQueryOutput{
			Type:   "asn",
			Target: input.Target,
			Result: result,
			Server: result.Server,
		}, nil

	default:
		return nil, fmt.Errorf("不支持的type: %s，支持: domain, ip, asn", input.Type)
	}
}

// AvailabilityInput 域名可用性检查输入
type AvailabilityInput struct {
	Domain string `json:"domain"`
}

// AvailabilityOutput 域名可用性检查输出
type AvailabilityOutput struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

// CheckAvailability 检查域名可用性
func (c *Controller) CheckAvailability(ctx context.Context, input AvailabilityInput) (*AvailabilityOutput, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	result, err := whois.CheckDomainAvailabilityWithContext(ctx, input.Domain)
	if err != nil {
		return nil, err
	}

	return &AvailabilityOutput{
		Domain:    result.Domain,
		Available: result.Available,
		Status:    result.Status,
		Message:   result.Message,
	}, nil
}

// WhoisCompareInput WHOIS对比输入
type WhoisCompareInput struct {
	Domain1 string `json:"domain1"`
	Domain2 string `json:"domain2"`
}

// WhoisCompareOutput WHOIS对比输出
type WhoisCompareOutput struct {
	Domain1 string              `json:"domain1"`
	Domain2 string              `json:"domain2"`
	Changes []*whois.WhoisChange `json:"changes"`
	Count   int                 `json:"count"`
}

// CompareWhoisInfo 对比两个域名的WHOIS信息
func (c *Controller) CompareWhoisInfo(ctx context.Context, input WhoisCompareInput) (*WhoisCompareOutput, error) {
	if input.Domain1 == "" || input.Domain2 == "" {
		return nil, fmt.Errorf("两个域名都不能为空")
	}

	info1, err := whois.ExecuteQueryWithContext(ctx, &whois.QueryOptions{Domain: input.Domain1})
	if err != nil {
		return nil, fmt.Errorf("查询 %s 失败: %w", input.Domain1, err)
	}

	info2, err := whois.ExecuteQueryWithContext(ctx, &whois.QueryOptions{Domain: input.Domain2})
	if err != nil {
		return nil, fmt.Errorf("查询 %s 失败: %w", input.Domain2, err)
	}

	changes := whois.CompareWhois(info1, info2)

	return &WhoisCompareOutput{
		Domain1: input.Domain1,
		Domain2: input.Domain2,
		Changes: changes,
		Count:   len(changes),
	}, nil
}

// QualityInput 质量评估输入
type QualityInput struct {
	Domain string `json:"domain"`
}

// QualityOutput 质量评估输出
type QualityOutput struct {
	Domain       string `json:"domain"`
	TotalScore   int    `json:"total_score"`
	Completeness int    `json:"completeness"`
	Timeliness   int    `json:"timeliness"`
	Reliability  int    `json:"reliability"`
	Level        string `json:"level"`
}

// AssessWhoisQuality 评估WHOIS数据质量
func (c *Controller) AssessWhoisQuality(ctx context.Context, input QualityInput) (*QualityOutput, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	info, err := whois.ExecuteQueryWithContext(ctx, &whois.QueryOptions{Domain: input.Domain})
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}

	score := whois.AssessQuality(info)

	return &QualityOutput{
		Domain:       input.Domain,
		TotalScore:   score.Total,
		Completeness: score.Completeness,
		Timeliness:   score.Timeliness,
		Reliability:  score.Reliability,
		Level:        string(score.Level),
	}, nil
}

// NormalizeInput 域名规范化输入
type NormalizeInput struct {
	Domain string `json:"domain"`
	Action string `json:"action,omitempty"` // normalize, to_punycode, to_unicode
}

// NormalizeOutput 域名规范化输出
type NormalizeOutput struct {
	Original   string `json:"original"`
	Result     string `json:"result"`
	IsIDN      bool   `json:"is_idn"`
	Action     string `json:"action"`
}

// NormalizeDomainName 规范化域名
func (c *Controller) NormalizeDomainName(input NormalizeInput) (*NormalizeOutput, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	if input.Action == "" {
		input.Action = "normalize"
	}

	var result string
	var err error

	switch input.Action {
	case "normalize":
		result, err = whois.NormalizeDomain(input.Domain)
	case "to_punycode":
		result, err = whois.UnicodeToPunycode(input.Domain)
	case "to_unicode":
		result, err = whois.PunycodeToUnicode(input.Domain)
	default:
		return nil, fmt.Errorf("不支持的action: %s，支持: normalize, to_punycode, to_unicode", input.Action)
	}

	if err != nil {
		return nil, err
	}

	return &NormalizeOutput{
		Original: input.Domain,
		Result:   result,
		IsIDN:    whois.IsIDN(input.Domain),
		Action:   input.Action,
	}, nil
}
