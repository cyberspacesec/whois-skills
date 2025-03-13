package mcp

import (
	"fmt"
	"sync"
	"time"
)

// RequestStatus 表示请求的状态
type RequestStatus string

// TaskStatus 表示任务的状态
type TaskStatus string

const (
	// 请求状态
	RequestStatusPending  RequestStatus = "pending"
	RequestStatusProgress RequestStatus = "in_progress"
	RequestStatusDone     RequestStatus = "done"

	// 任务状态
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusDone     TaskStatus = "done"
	TaskStatusApproved TaskStatus = "approved"
	TaskStatusFailed   TaskStatus = "failed"
)

// Task 表示一个MCP任务
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Details     string     `json:"details,omitempty"`
}

// Request 表示一个MCP请求
type Request struct {
	ID              string        `json:"id"`
	OriginalRequest string        `json:"original_request"`
	SplitDetails    string        `json:"split_details,omitempty"`
	Status          RequestStatus `json:"status"`
	Tasks           []*Task       `json:"tasks"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
}

// RequestStore 管理所有MCP请求
type RequestStore struct {
	// 请求映射表
	requests map[string]*Request
	// 任务映射表
	tasks map[string]*Task

	// 互斥锁
	mu sync.RWMutex
}

// NewRequestStore 创建一个新的请求存储
func NewRequestStore() *RequestStore {
	return &RequestStore{
		requests: make(map[string]*Request),
		tasks:    make(map[string]*Task),
	}
}

// AddRequest 添加一个新请求
func (s *RequestStore) AddRequest(request *Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests[request.ID] = request

	// 添加任务到任务映射表
	for _, task := range request.Tasks {
		s.tasks[task.ID] = task
	}
}

// GetRequest 获取一个请求
func (s *RequestStore) GetRequest(id string) (*Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	request, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("请求 ID %s 不存在", id)
	}

	return request, nil
}

// GetTask 获取一个任务
func (s *RequestStore) GetTask(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("任务 ID %s 不存在", id)
	}

	return task, nil
}

// GetNextPendingTask 获取请求的下一个待处理任务
func (s *RequestStore) GetNextPendingTask(requestID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	request, ok := s.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("请求 ID %s 不存在", requestID)
	}

	for _, task := range request.Tasks {
		if task.Status == TaskStatusPending {
			return task, nil
		}
	}

	return nil, nil // 没有待处理的任务
}

// UpdateTask 更新任务状态
func (s *RequestStore) UpdateTask(taskID string, status TaskStatus, details string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务 ID %s 不存在", taskID)
	}

	task.Status = status
	task.UpdatedAt = time.Now()

	if status == TaskStatusDone || status == TaskStatusApproved {
		now := time.Now()
		task.CompletedAt = &now
	}

	if details != "" {
		task.Details = details
	}

	// 更新请求状态
	for _, request := range s.requests {
		for _, t := range request.Tasks {
			if t.ID == taskID {
				request.UpdatedAt = time.Now()

				// 检查是否所有任务都已批准
				allApproved := true
				for _, reqTask := range request.Tasks {
					if reqTask.Status != TaskStatusApproved {
						allApproved = false
						break
					}
				}

				if allApproved {
					request.Status = RequestStatusDone
					now := time.Now()
					request.CompletedAt = &now
				} else {
					request.Status = RequestStatusProgress
				}

				break
			}
		}
	}

	return nil
}

// AddTasksToRequest 向请求添加新任务
func (s *RequestStore) AddTasksToRequest(requestID string, tasks []*Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.requests[requestID]
	if !ok {
		return fmt.Errorf("请求 ID %s 不存在", requestID)
	}

	// 添加新任务
	request.Tasks = append(request.Tasks, tasks...)
	request.UpdatedAt = time.Now()

	// 添加任务到任务映射表
	for _, task := range tasks {
		s.tasks[task.ID] = task
	}

	return nil
}

// UpdateRequestStatus 更新请求状态
func (s *RequestStore) UpdateRequestStatus(requestID string, status RequestStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.requests[requestID]
	if !ok {
		return fmt.Errorf("请求 ID %s 不存在", requestID)
	}

	request.Status = status
	request.UpdatedAt = time.Now()

	if status == RequestStatusDone {
		now := time.Now()
		request.CompletedAt = &now
	}

	return nil
}

// GetAllRequests 获取所有请求
func (s *RequestStore) GetAllRequests() []*Request {
	s.mu.RLock()
	defer s.mu.RUnlock()

	requests := make([]*Request, 0, len(s.requests))
	for _, request := range s.requests {
		requests = append(requests, request)
	}

	return requests
}

// IsRequestDone 检查请求是否完成
func (s *RequestStore) IsRequestDone(requestID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	request, ok := s.requests[requestID]
	if !ok {
		return false, fmt.Errorf("请求 ID %s 不存在", requestID)
	}

	return request.Status == RequestStatusDone, nil
}
