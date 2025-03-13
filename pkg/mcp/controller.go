package mcp

import (
	"fmt"
	"time"

	"github.com/cyberspacesec/whois-hacker/pkg/whois"
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
	RequestID string `json:"requestId"`
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

// ExecuteWhoisQuery 执行WHOIS查询
func (c *Controller) ExecuteWhoisQuery(domain string) (*whoisparser.WhoisInfo, error) {
	queryOpts := &whois.Query{
		Domain: domain,
	}
	return whois.Execute(queryOpts)
}
