package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// GetNextTask 全部任务完成分支（AllTasksDone）
// ============================================================

func TestController_GetNextTask_AllTasksDone(t *testing.T) {
	ctrl := NewController()

	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]

	// 完成 + 批准该任务，使其不再 pending
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID})

	// 此时无 pending 任务，应返回 AllTasksDone=true，Task=nil
	output, err := ctrl.GetNextTask(GetNextTaskInput{RequestID: planOutput.RequestID})
	assert.NoError(t, err)
	assert.True(t, output.AllTasksDone)
	assert.Nil(t, output.Task)
}

// ============================================================
// MarkTaskDone 错误分支：请求不存在 / 任务不存在 / 跨请求
// ============================================================

func TestController_MarkTaskDone_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: "nonexistent", TaskID: "task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestController_MarkTaskDone_TaskNotFound(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	// 请求存在但 taskID 不在 store 中
	_, err := ctrl.MarkTaskDone(MarkTaskDoneInput{
		RequestID: planOutput.RequestID, TaskID: "nonexistent-task",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestController_MarkTaskDone_TaskNotBelongToRequest(t *testing.T) {
	ctrl := NewController()
	// 请求 A 和请求 B，各自有一个任务
	planA, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "A", Tasks: []TaskInput{{Title: "a1"}},
	})
	planB, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "B", Tasks: []TaskInput{{Title: "b1"}},
	})
	taskB := planB.Tasks[0]["id"]

	// 用请求 A + 任务 B（任务存在但不属于请求 A）
	_, err := ctrl.MarkTaskDone(MarkTaskDoneInput{
		RequestID: planA.RequestID, TaskID: taskB,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不属于")
}

// ============================================================
// ApproveTaskCompletion 错误分支
// ============================================================

func TestController_ApproveTaskCompletion_EmptyIDs(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: "", TaskID: ""})
	assert.Error(t, err)
}

func TestController_ApproveTaskCompletion_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: "nonexistent", TaskID: "task"})
	assert.Error(t, err)
}

func TestController_ApproveTaskCompletion_TaskNotFound(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	_, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{
		RequestID: planOutput.RequestID, TaskID: "nonexistent-task",
	})
	assert.Error(t, err)
}

func TestController_ApproveTaskCompletion_TaskNotBelongToRequest(t *testing.T) {
	ctrl := NewController()
	planA, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "A", Tasks: []TaskInput{{Title: "a1"}},
	})
	planB, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "B", Tasks: []TaskInput{{Title: "b1"}},
	})
	taskB := planB.Tasks[0]["id"]
	// 先把任务 B 标记为完成，使其通过 status 检查
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planB.RequestID, TaskID: taskB})

	_, err := ctrl.ApproveTaskCompletion(ApproveTaskInput{
		RequestID: planA.RequestID, TaskID: taskB,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不属于")
}

// ============================================================
// ApproveRequestCompletion 错误分支：空ID / 请求不存在
// ============================================================

func TestController_ApproveRequestCompletion_EmptyID(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ApproveRequestCompletion(ApproveRequestInput{RequestID: ""})
	assert.Error(t, err)
}

func TestController_ApproveRequestCompletion_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ApproveRequestCompletion(ApproveRequestInput{RequestID: "nonexistent"})
	assert.Error(t, err)
}

// ============================================================
// AddTasksToRequest 错误分支：请求不存在 / 请求已完成
// ============================================================

func TestController_AddTasksToRequest_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.AddTasksToRequest(AddTasksInput{
		RequestID: "nonexistent",
		Tasks:     []TaskInput{{Title: "t"}},
	})
	assert.Error(t, err)
}

func TestController_AddTasksToRequest_RequestDone(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]
	// 完成 + 批准 -> 请求变 done
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveRequestCompletion(ApproveRequestInput{RequestID: planOutput.RequestID})

	// 向已完成的请求添加任务应失败
	_, err := ctrl.AddTasksToRequest(AddTasksInput{
		RequestID: planOutput.RequestID,
		Tasks:     []TaskInput{{Title: "new"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已完成")
}

// ============================================================
// UpdateTask 错误分支：请求/任务不存在、跨请求、已完成/已批准
// ============================================================

func TestController_UpdateTask_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.UpdateTask(UpdateTaskInput{RequestID: "nonexistent", TaskID: "task"})
	assert.Error(t, err)
}

func TestController_UpdateTask_TaskNotFound(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	_, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planOutput.RequestID, TaskID: "nonexistent-task",
	})
	assert.Error(t, err)
}

func TestController_UpdateTask_TaskNotBelongToRequest(t *testing.T) {
	ctrl := NewController()
	planA, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "A", Tasks: []TaskInput{{Title: "a1"}},
	})
	planB, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "B", Tasks: []TaskInput{{Title: "b1"}},
	})
	taskB := planB.Tasks[0]["id"]

	_, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planA.RequestID, TaskID: taskB,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不属于")
}

func TestController_UpdateTask_AlreadyDone(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})

	_, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID, Title: "new",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已完成或已批准")
}

func TestController_UpdateTask_AlreadyApproved(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID})

	_, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID, Title: "new",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已完成或已批准")
}

// 仅更新 title 或仅更新 description 的分支
func TestController_UpdateTask_OnlyTitleOrDescription(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1", Description: "d1"}},
	})
	taskID := planOutput.Tasks[0]["id"]

	// 仅传 title，description 为空 -> 不更新 description
	out, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID, Title: "new-title",
	})
	assert.NoError(t, err)
	assert.Equal(t, "任务信息已更新", out.Message)

	// 仅传 description
	out2, err := ctrl.UpdateTask(UpdateTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID, Description: "new-desc",
	})
	assert.NoError(t, err)
	assert.Equal(t, "任务信息已更新", out2.Message)
}

// ============================================================
// DeleteTask 错误分支：空ID(TaskID空)、任务不存在于请求、已完成/已批准
// ============================================================

func TestController_DeleteTask_EmptyTaskID(t *testing.T) {
	ctrl := NewController()
	// RequestID 空、TaskID 非空 -> 命中空校验
	_, err := ctrl.DeleteTask(DeleteTaskInput{RequestID: "", TaskID: "task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestController_DeleteTask_RequestNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.DeleteTask(DeleteTaskInput{RequestID: "nonexistent", TaskID: "task"})
	assert.Error(t, err)
}

func TestController_DeleteTask_TaskNotInRequest(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	// taskID 不在请求的任务列表里
	_, err := ctrl.DeleteTask(DeleteTaskInput{
		RequestID: planOutput.RequestID, TaskID: "nonexistent-task",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在于请求中")
}

func TestController_DeleteTask_AlreadyDone(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})

	_, err := ctrl.DeleteTask(DeleteTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已完成或已批准")
}

func TestController_DeleteTask_AlreadyApproved(t *testing.T) {
	ctrl := NewController()
	planOutput, _ := ctrl.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test",
		Tasks:           []TaskInput{{Title: "task1"}},
	})
	taskID := planOutput.Tasks[0]["id"]
	ctrl.MarkTaskDone(MarkTaskDoneInput{RequestID: planOutput.RequestID, TaskID: taskID})
	ctrl.ApproveTaskCompletion(ApproveTaskInput{RequestID: planOutput.RequestID, TaskID: taskID})

	_, err := ctrl.DeleteTask(DeleteTaskInput{
		RequestID: planOutput.RequestID, TaskID: taskID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已完成或已批准")
}

// ============================================================
// GetTaskDetails 错误分支：任务不存在
// ============================================================

func TestController_GetTaskDetails_TaskNotFound(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.GetTaskDetails(TaskDetailsInput{TaskID: "nonexistent"})
	assert.Error(t, err)
}

// ============================================================
// ListRequests 空列表分支
// ============================================================

func TestController_ListRequests_Empty(t *testing.T) {
	ctrl := NewController()
	out, err := ctrl.ListRequests()
	assert.NoError(t, err)
	assert.Empty(t, out.Requests)
	assert.Contains(t, out.Message, "0")
}

// ============================================================
// IsRequestDone 不存在分支
// ============================================================

func TestRequestStore_IsRequestDone_NotFound(t *testing.T) {
	store := NewRequestStore()
	done, err := store.IsRequestDone("nonexistent")
	assert.Error(t, err)
	assert.False(t, done)
}

// ============================================================
// NormalizeDomainName 全分支
// ============================================================

func TestController_NormalizeDomainName_EmptyDomain(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}

func TestController_NormalizeDomainName_DefaultAction(t *testing.T) {
	ctrl := NewController()
	out, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "Example.COM."})
	assert.NoError(t, err)
	// action 默认为 normalize
	assert.Equal(t, "normalize", out.Action)
	assert.Equal(t, "Example.COM.", out.Original)
}

func TestController_NormalizeDomainName_Normalize(t *testing.T) {
	ctrl := NewController()
	out, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "https://Example.com/path", Action: "normalize"})
	assert.NoError(t, err)
	assert.Equal(t, "normalize", out.Action)
}

func TestController_NormalizeDomainName_ToPunycode(t *testing.T) {
	ctrl := NewController()
	out, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "例子.测试", Action: "to_punycode"})
	assert.NoError(t, err)
	assert.Equal(t, "to_punycode", out.Action)
	assert.NotEmpty(t, out.Result)
	// 转成 punycode 后应包含 xn-- 前缀
	assert.Contains(t, out.Result, "xn--")
}

func TestController_NormalizeDomainName_ToUnicode(t *testing.T) {
	ctrl := NewController()
	// 先转 punycode 再转回来
	punyOut, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "例子.测试", Action: "to_punycode"})
	assert.NoError(t, err)
	uniOut, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: punyOut.Result, Action: "to_unicode"})
	assert.NoError(t, err)
	assert.Equal(t, "to_unicode", uniOut.Action)
	assert.Contains(t, uniOut.Result, "例子")
}

func TestController_NormalizeDomainName_InvalidAction(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "example.com", Action: "bogus"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的action")
}

func TestController_NormalizeDomainName_IsIDN(t *testing.T) {
	ctrl := NewController()
	out, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "例子.com", Action: "normalize"})
	assert.NoError(t, err)
	assert.True(t, out.IsIDN)
}

func TestController_NormalizeDomainName_ToPunycodeError(t *testing.T) {
	ctrl := NewController()
	// xn--- 是非法 punycode label，idna.ToASCII/ToUnicode 会返回错误
	_, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "xn---", Action: "to_punycode"})
	assert.Error(t, err)
}

func TestController_NormalizeDomainName_ToUnicodeError(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.NormalizeDomainName(NormalizeInput{Domain: "xn---", Action: "to_unicode"})
	assert.Error(t, err)
}

// ============================================================
// Controller 网络型查询函数：仅覆盖参数校验错误分支（happy-path 需真实网络）
// ============================================================

func TestController_ExecuteWhoisQueryFull_EmptyDomain(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteWhoisQueryFull(context.Background(), WhoisQueryInput{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}

func TestController_ExecuteIPWhoisQuery_EmptyIP(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteIPWhoisQuery(context.Background(), IPQueryInput{IP: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IP地址不能为空")
}

func TestController_ExecuteASNQuery_NonPositive(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteASNQuery(context.Background(), ASNQueryInput{ASN: 0})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ASN必须为正整数")
}

func TestController_ExecuteASNQuery_Negative(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteASNQuery(context.Background(), ASNQueryInput{ASN: -5})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ASN必须为正整数")
}

func TestController_ExecuteRDAPQuery_EmptyType(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteRDAPQuery(context.Background(), RDAPQueryInput{Type: "", Target: "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type和target不能为空")
}

func TestController_ExecuteRDAPQuery_EmptyTarget(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteRDAPQuery(context.Background(), RDAPQueryInput{Type: "domain", Target: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "type和target不能为空")
}

func TestController_ExecuteRDAPQuery_UnsupportedType(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.ExecuteRDAPQuery(context.Background(), RDAPQueryInput{Type: "bogus", Target: "x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的type")
}

func TestController_CheckAvailability_EmptyDomain(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.CheckAvailability(context.Background(), AvailabilityInput{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}

func TestController_CompareWhoisInfo_EmptyDomains(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.CompareWhoisInfo(context.Background(), WhoisCompareInput{Domain1: "", Domain2: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "两个域名都不能为空")
}

func TestController_AssessWhoisQuality_EmptyDomain(t *testing.T) {
	ctrl := NewController()
	_, err := ctrl.AssessWhoisQuality(context.Background(), QualityInput{Domain: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "域名不能为空")
}
