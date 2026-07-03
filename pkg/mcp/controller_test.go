package mcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// RequestStore 测试
// ============================================================

func TestNewRequestStore(t *testing.T) {
	store := NewRequestStore()
	assert.NotNil(t, store)
	assert.NotNil(t, store.requests)
	assert.NotNil(t, store.tasks)
}

func TestRequestStore_AddRequest(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "查询 example.com 的WHOIS信息",
		Status:          RequestStatusPending,
		Tasks: []*Task{
			{ID: "task-1", Title: "查询WHOIS", Status: TaskStatusPending},
			{ID: "task-2", Title: "解析结果", Status: TaskStatusPending},
		},
	}

	store.AddRequest(req)

	// 验证请求可以获取
	gotReq, err := store.GetRequest("req-1")
	assert.NoError(t, err)
	assert.Equal(t, "req-1", gotReq.ID)
	assert.Equal(t, RequestStatusPending, gotReq.Status)
	assert.Len(t, gotReq.Tasks, 2)

	// 验证任务在映射表中
	task, err := store.GetTask("task-1")
	assert.NoError(t, err)
	assert.Equal(t, "查询WHOIS", task.Title)
}

func TestRequestStore_GetRequest_NotFound(t *testing.T) {
	store := NewRequestStore()

	_, err := store.GetRequest("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestRequestStore_GetTask_NotFound(t *testing.T) {
	store := NewRequestStore()

	_, err := store.GetTask("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestRequestStore_GetNextPendingTask(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks: []*Task{
			{ID: "task-1", Title: "第一步", Status: TaskStatusDone},
			{ID: "task-2", Title: "第二步", Status: TaskStatusPending},
			{ID: "task-3", Title: "第三步", Status: TaskStatusPending},
		},
	}

	store.AddRequest(req)

	// 应返回第一个 pending 的任务
	task, err := store.GetNextPendingTask("req-1")
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "task-2", task.ID)
}

func TestRequestStore_GetNextPendingTask_AllDone(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusProgress,
		Tasks: []*Task{
			{ID: "task-1", Title: "第一步", Status: TaskStatusDone},
			{ID: "task-2", Title: "第二步", Status: TaskStatusApproved},
		},
	}

	store.AddRequest(req)

	// 所有任务已完成，应返回 nil
	task, err := store.GetNextPendingTask("req-1")
	assert.NoError(t, err)
	assert.Nil(t, task)
}

func TestRequestStore_GetNextPendingTask_RequestNotFound(t *testing.T) {
	store := NewRequestStore()

	_, err := store.GetNextPendingTask("nonexistent")
	assert.Error(t, err)
}

func TestRequestStore_UpdateTask(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks: []*Task{
			{ID: "task-1", Title: "第一步", Status: TaskStatusPending},
		},
	}

	store.AddRequest(req)

	// 标记任务为完成
	err := store.UpdateTask("task-1", TaskStatusDone, "完成详情")
	assert.NoError(t, err)

	task, err := store.GetTask("task-1")
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusDone, task.Status)
	assert.Equal(t, "完成详情", task.Details)
	assert.NotNil(t, task.CompletedAt)
}

func TestRequestStore_UpdateTask_NotFound(t *testing.T) {
	store := NewRequestStore()

	err := store.UpdateTask("nonexistent", TaskStatusDone, "")
	assert.Error(t, err)
}

func TestRequestStore_UpdateTask_ApprovedChangesRequestStatus(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks: []*Task{
			{ID: "task-1", Title: "第一步", Status: TaskStatusPending},
		},
	}

	store.AddRequest(req)

	// 标记任务为 approved -> 请求状态应该变为 done
	err := store.UpdateTask("task-1", TaskStatusApproved, "")
	assert.NoError(t, err)

	gotReq, _ := store.GetRequest("req-1")
	assert.Equal(t, RequestStatusDone, gotReq.Status)
	assert.NotNil(t, gotReq.CompletedAt)
}

func TestRequestStore_AddTasksToRequest(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks: []*Task{
			{ID: "task-1", Title: "第一步", Status: TaskStatusPending},
		},
	}

	store.AddRequest(req)

	newTasks := []*Task{
		{ID: "task-2", Title: "新增任务", Status: TaskStatusPending},
		{ID: "task-3", Title: "再新增一个", Status: TaskStatusPending},
	}

	err := store.AddTasksToRequest("req-1", newTasks)
	assert.NoError(t, err)

	gotReq, _ := store.GetRequest("req-1")
	assert.Len(t, gotReq.Tasks, 3)

	task2, err := store.GetTask("task-2")
	assert.NoError(t, err)
	assert.Equal(t, "新增任务", task2.Title)
}

func TestRequestStore_AddTasksToRequest_RequestNotFound(t *testing.T) {
	store := NewRequestStore()

	err := store.AddTasksToRequest("nonexistent", []*Task{})
	assert.Error(t, err)
}

func TestRequestStore_UpdateRequestStatus(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks:           []*Task{},
	}

	store.AddRequest(req)

	err := store.UpdateRequestStatus("req-1", RequestStatusDone)
	assert.NoError(t, err)

	gotReq, _ := store.GetRequest("req-1")
	assert.Equal(t, RequestStatusDone, gotReq.Status)
	assert.NotNil(t, gotReq.CompletedAt)
}

func TestRequestStore_UpdateRequestStatus_NotFound(t *testing.T) {
	store := NewRequestStore()

	err := store.UpdateRequestStatus("nonexistent", RequestStatusDone)
	assert.Error(t, err)
}

func TestRequestStore_GetAllRequests(t *testing.T) {
	store := NewRequestStore()

	for i := 0; i < 3; i++ {
		req := &Request{
			ID:              fmt.Sprintf("req-%d", i),
			OriginalRequest: fmt.Sprintf("请求 %d", i),
			Status:          RequestStatusPending,
			Tasks:           []*Task{},
		}
		store.AddRequest(req)
	}

	all := store.GetAllRequests()
	assert.Len(t, all, 3)
}

func TestRequestStore_IsRequestDone(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusDone,
		Tasks:           []*Task{},
	}

	store.AddRequest(req)

	done, err := store.IsRequestDone("req-1")
	assert.NoError(t, err)
	assert.True(t, done)
}

func TestRequestStore_IsRequestDone_NotDone(t *testing.T) {
	store := NewRequestStore()

	req := &Request{
		ID:              "req-1",
		OriginalRequest: "test request",
		Status:          RequestStatusPending,
		Tasks:           []*Task{},
	}

	store.AddRequest(req)

	done, err := store.IsRequestDone("req-1")
	assert.NoError(t, err)
	assert.False(t, done)
}

// ============================================================
// Controller 测试
// ============================================================

func TestController_PlanRequest(t *testing.T) {
	ctrl := NewController()

	input := RequestPlanningInput{
		OriginalRequest: "查询多个域名的WHOIS信息",
		Tasks: []TaskInput{
			{Title: "查询 example.com", Description: "查询example.com的WHOIS信息"},
			{Title: "查询 test.com", Description: "查询test.com的WHOIS信息"},
		},
	}

	output, err := ctrl.PlanRequest(input)
	assert.NoError(t, err)
	assert.NotEmpty(t, output.RequestID)
	assert.Len(t, output.Tasks, 2)
	assert.Contains(t, output.Message, "2")
}

func TestController_PlanRequest_EmptyOriginalRequest(t *testing.T) {
	ctrl := NewController()

	input := RequestPlanningInput{
		OriginalRequest: "",
		Tasks:           []TaskInput{{Title: "task"}},
	}

	_, err := ctrl.PlanRequest(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "原始请求不能为空")
}

func TestController_PlanRequest_EmptyTasks(t *testing.T) {
	ctrl := NewController()

	input := RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{},
	}

	_, err := ctrl.PlanRequest(input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务列表不能为空")
}

func TestController_GetNextTask(t *testing.T) {
	ctrl := NewController()

	// 先创建请求
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test request",
		Tasks: []TaskInput{
			{Title: "第一步", Description: "第一步描述"},
		},
	})

	// 获取下一个任务
	output, err := ctrl.GetNextTask(GetNextTaskInput{RequestID: planOutput.RequestID})
	assert.NoError(t, err)
	assert.False(t, output.AllTasksDone)
	assert.NotNil(t, output.Task)
	assert.Equal(t, "第一步", output.Task.Title)
}

func TestController_GetNextTask_EmptyRequestID(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.GetNextTask(GetNextTaskInput{RequestID: ""})
	assert.Error(t, err)
}

func TestController_GetNextTask_InvalidRequestID(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.GetNextTask(GetNextTaskInput{RequestID: "nonexistent"})
	assert.Error(t, err)
}

func TestController_MarkTaskDone(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "task1", Description: "desc1"},
		},
	})

	taskID := planOutput.Tasks[0]["id"]

	output, err := ctrl.MarkTaskDone(MarkTaskDoneInput{
		RequestID:        planOutput.RequestID,
		TaskID:           taskID,
		CompletedDetails: "已完成",
	})
	assert.NoError(t, err)
	assert.Contains(t, output.Message, "已标记为完成")
}

func TestController_MarkTaskDone_EmptyIDs(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: "", TaskID: "task"})
	assert.Error(t, err)

	_, err = ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: "req", TaskID: ""})
	assert.Error(t, err)
}

func TestController_ApproveTaskCompletion(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "task1", Description: "desc1"},
		},
	})

	taskID := planOutput.Tasks[0]["id"]

	// 先标记完成
	ctrl.MarkTaskDone(MarkTaskDoneInput{
		RequestID: planOutput.RequestID,
		TaskID:    taskID,
	})

	// 再批准
	output, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{
		RequestID: planOutput.RequestID,
		TaskID:    taskID,
	})
	assert.NoError(t, err)
	assert.Contains(t, output.Message, "已批准完成")
}

func TestController_ApproveTaskCompletion_NotDoneYet(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "task1", Description: "desc1"},
		},
	})

	taskID := planOutput.Tasks[0]["id"]

	// 任务还没标记完成，直接批准应失败
	_, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{
		RequestID: planOutput.RequestID,
		TaskID:    taskID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须先标记为完成")
}

func TestController_ApproveRequestCompletion(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "task1", Description: "desc1"},
		},
	})

	taskID := planOutput.Tasks[0]["id"]

	// 标记完成并批准所有任务
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID})

	// 批准请求完成
	output, err := ctrl.ApproveRequestCompletion(ApproveRequestInput{RequestID: planOutput.RequestID})
	assert.NoError(t, err)
	assert.Contains(t, output.Message, "已完成并批准")
}

func TestController_ApproveRequestCompletion_NotAllApproved(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "task1", Description: "desc1"},
			{Title: "task2", Description: "desc2"},
		},
	})

	taskID1 := planOutput.Tasks[0]["id"]

	// 只批准了一个任务
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID1})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID1})

	// 尝试批准请求应失败
	_, err := ctrl.ApproveRequestCompletion(ApproveRequestInput{RequestID: planOutput.RequestID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "所有任务必须先被批准")
}

func TestController_ListRequests(t *testing.T) {
	ctrl := NewController()

	ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "request 1",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "request 2",
		Tasks:           []TaskInput{{Title: "task2"}},
	})

	output, err := ctrl.ListRequests()
	assert.NoError(t, err)
	assert.Len(t, output.Requests, 2)
	assert.Contains(t, output.Message, "2")
}

func TestController_AddTasksToRequest(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})

	output, err := ctrl.AddTasksToRequest(AddTasksInput{
		RequestID: planOutput.RequestID,
		Tasks:     []TaskInput{{Title: "新增任务"}, {Title: "再新增"}},
	})
	assert.NoError(t, err)
	assert.Len(t, output.Tasks, 2)
}

func TestController_AddTasksToRequest_EmptyRequestID(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.AddTasksToRequest(AddTasksInput{RequestID: ""})
	assert.Error(t, err)
}

func TestController_AddTasksToRequest_EmptyTasks(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.AddTasksToRequest(AddTasksInput{RequestID: "req", Tasks: []TaskInput{}})
	assert.Error(t, err)
}

func TestController_UpdateTask(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "旧标题", Description: "旧描述"}},
	})

	taskID := planOutput.Tasks[0]["id"]

	output, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID:   planOutput.RequestID,
		TaskID:      taskID,
		Title:       "新标题",
		Description: "新描述",
	})
	assert.NoError(t, err)
	assert.Equal(t, "任务信息已更新", output.Message)
}

func TestController_UpdateTask_EmptyIDs(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.UpdateTask(UpdateTaskInput{RequestID: "", TaskID: "task"})
	assert.Error(t, err)

	_, err = ctrl.UpdateTask(UpdateTaskInput{RequestID: "req", TaskID: ""})
	assert.Error(t, err)
}

func TestController_DeleteTask(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks: []TaskInput{
			{Title: "保留任务", Description: "保留"},
			{Title: "删除任务", Description: "删除"},
		},
	})

	taskID := planOutput.Tasks[1]["id"]

	output, err := ctrl.DeleteTask(DeleteTaskInput{
		RequestID: planOutput.RequestID,
		TaskID:    taskID,
	})
	assert.NoError(t, err)
	assert.Contains(t, output.Message, "已删除")
}

func TestController_DeleteTask_EmptyIDs(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.DeleteTask(DeleteTaskInput{RequestID: "", TaskID: "task"})
	assert.Error(t, err)
}

func TestController_GetTaskDetails(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1", Description: "desc1"}},
	})

	taskID := planOutput.Tasks[0]["id"]

	output, err := ctrl.GetTaskDetails(TaskDetailsInput{TaskID: taskID})
	assert.NoError(t, err)
	assert.NotNil(t, output.Task)
	assert.Equal(t, "task1", output.Task.Title)
}

func TestController_GetTaskDetails_EmptyTaskID(t *testing.T) {
	ctrl := NewController()

	_, err := ctrl.GetTaskDetails(TaskDetailsInput{TaskID: ""})
	assert.Error(t, err)
}

func TestController_GenerateProgressInfo(t *testing.T) {
	ctrl := NewController()

	req := &Request{
		ID:    "req-1",
		Tasks: []*Task{
			{ID: "t1", Status: TaskStatusPending},
			{ID: "t2", Status: TaskStatusDone},
			{ID: "t3", Status: TaskStatusApproved},
		},
	}

	progress := ctrl.generateProgressInfo(req)
	assert.Contains(t, progress, "2/3")
	assert.Contains(t, progress, "1/3")
}
