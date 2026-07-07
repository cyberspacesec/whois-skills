package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// newTestServer 创建一个用于测试的 MCP 服务器实例
func newTestServer() *Server {
	return NewServer(logrus.New())
}

// mustPlanRequest 通过 controller 创建一个请求并返回 requestID 与 task id 列表
func mustPlanRequest(t *testing.T, s *Server, tasks []TaskInput) (string, []string) {
	t.Helper()
	out, err := s.controller.PlanRequest(RequestPlanningInput{
		OriginalRequest: "test request",
		Tasks:           tasks,
	})
	assert.NoError(t, err)
	ids := make([]string, 0, len(out.Tasks))
	for _, tk := range out.Tasks {
		ids = append(ids, tk["id"])
	}
	return out.RequestID, ids
}

// doJSON 对给定 handlerFunc 发起一次 POST 请求，body 为 marshal 后的 v，
// 返回响应记录器。body 为 nil 时发送空 body。
func doJSON(handler http.HandlerFunc, v interface{}) *httptest.ResponseRecorder {
	var body *bytes.Buffer
	if v != nil {
		b, _ := json.Marshal(v)
		body = bytes.NewBuffer(b)
	} else {
		body = bytes.NewBufferString("")
	}
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// doInvalidJSON 发送非法 JSON body 触发 decode 错误分支
func doInvalidJSON(handler http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not valid json"))
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// decodeBody 将响应体解码到 map 中，用于断言
func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&m)
	assert.NoError(t, err)
	return m
}

// ============================================================
// NewServer / RegisterRoutes / respondJSON / respondError
// ============================================================

func TestNewServer(t *testing.T) {
	s := newTestServer()
	assert.NotNil(t, s)
	assert.NotNil(t, s.controller)
	assert.NotNil(t, s.logger)
}

func TestServer_RegisterRoutes(t *testing.T) {
	s := newTestServer()
	router := mux.NewRouter()
	s.RegisterRoutes(router)

	// 用 mux 路由器逐个端点做一次合法请求，确保路由已注册且可调用
	cases := []struct {
		method string
		path   string
		body   interface{}
	}{
		{http.MethodPost, "/mcp/request_planning", RequestPlanningInput{
			OriginalRequest: "r", Tasks: []TaskInput{{Title: "t"}},
		}},
		{http.MethodPost, "/mcp/get_next_task", GetNextTaskInput{RequestID: "nonexistent"}},
		{http.MethodPost, "/mcp/mark_task_done", MarkTaskDoneInput{RequestID: "x", TaskID: "y"}},
		{http.MethodPost, "/mcp/approve_task_completion", ApproveTaskInput{RequestID: "x", TaskID: "y"}},
		{http.MethodPost, "/mcp/approve_request_completion", ApproveRequestInput{RequestID: "x"}},
		{http.MethodPost, "/mcp/open_task_details", TaskDetailsInput{TaskID: "x"}},
		{http.MethodGet, "/mcp/list_requests", nil},
		{http.MethodPost, "/mcp/add_tasks_to_request", AddTasksInput{RequestID: "x"}},
		{http.MethodPost, "/mcp/update_task", UpdateTaskInput{RequestID: "x", TaskID: "y"}},
		{http.MethodPost, "/mcp/delete_task", DeleteTaskInput{RequestID: "x", TaskID: "y"}},
	}

	for _, c := range cases {
		var req *http.Request
		if c.body != nil {
			b, _ := json.Marshal(c.body)
			req = httptest.NewRequest(c.method, c.path, bytes.NewBuffer(b))
		} else {
			req = httptest.NewRequest(c.method, c.path, nil)
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		// 所有请求都应返回 JSON 响应（成功或错误），状态码 200 或 500
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"),
			"route %s should return json content-type", c.path)
	}
}

func TestServer_RespondJSON_EncodeError(t *testing.T) {
	// 传入不可编码类型（func）触发 json.Encoder.Encode 错误分支
	s := newTestServer()
	rr := httptest.NewRecorder()
	// 不应 panic，仅记录错误日志
	assert.NotPanics(t, func() {
		s.respondJSON(rr, http.StatusOK, func() {})
	})
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestServer_RespondError(t *testing.T) {
	s := newTestServer()
	rr := httptest.NewRecorder()
	s.respondError(rr, http.StatusBadRequest, "bad input")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	m := decodeBody(t, rr)
	assert.Equal(t, "bad input", m["error"])
}

// ============================================================
// handleRequestPlanning
// ============================================================

func TestServer_HandleRequestPlanning_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleRequestPlanning)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "无效的请求格式", decodeBody(t, rr)["error"])
}

func TestServer_HandleRequestPlanning_ControllerError(t *testing.T) {
	s := newTestServer()
	// 空原始请求 -> controller 返回错误 -> 500
	rr := doJSON(s.handleRequestPlanning, RequestPlanningInput{
		OriginalRequest: "",
		Tasks:           []TaskInput{{Title: "t"}},
	})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["error"], "原始请求不能为空")
}

func TestServer_HandleRequestPlanning_Success(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleRequestPlanning, RequestPlanningInput{
		OriginalRequest: "查询 example.com",
		Tasks: []TaskInput{
			{Title: "step1", Description: "d1"},
			{Title: "step2", Description: "d2"},
		},
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.NotEmpty(t, m["requestId"])
	assert.Len(t, m["tasks"], 2)
}

// ============================================================
// handleGetNextTask
// ============================================================

func TestServer_HandleGetNextTask_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleGetNextTask)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleGetNextTask_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleGetNextTask, GetNextTaskInput{RequestID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["error"], "请求ID不能为空")
}

func TestServer_HandleGetNextTask_Success(t *testing.T) {
	s := newTestServer()
	rid, _ := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := doJSON(s.handleGetNextTask, GetNextTaskInput{RequestID: rid})
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.Equal(t, false, m["all_tasks_done"])
}

// ============================================================
// handleMarkTaskDone
// ============================================================

func TestServer_HandleMarkTaskDone_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleMarkTaskDone)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleMarkTaskDone_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleMarkTaskDone, MarkTaskDoneInput{RequestID: "", TaskID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleMarkTaskDone_Success(t *testing.T) {
	s := newTestServer()
	rid, ids := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := doJSON(s.handleMarkTaskDone, MarkTaskDoneInput{
		RequestID: rid, TaskID: ids[0], CompletedDetails: "done",
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["message"], "已标记为完成")
}

// ============================================================
// handleApproveTaskCompletion
// ============================================================

func TestServer_HandleApproveTaskCompletion_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleApproveTaskCompletion)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleApproveTaskCompletion_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleApproveTaskCompletion, ApproveTaskInput{RequestID: "", TaskID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleApproveTaskCompletion_Success(t *testing.T) {
	s := newTestServer()
	rid, ids := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	s.controller.MarkTaskDone(MarkTaskDoneInput{RequestID: rid, TaskID: ids[0]})
	rr := doJSON(s.handleApproveTaskCompletion, ApproveTaskInput{
		RequestID: rid, TaskID: ids[0],
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["message"], "已批准完成")
}

// ============================================================
// handleApproveRequestCompletion
// ============================================================

func TestServer_HandleApproveRequestCompletion_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleApproveRequestCompletion)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleApproveRequestCompletion_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleApproveRequestCompletion, ApproveRequestInput{RequestID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleApproveRequestCompletion_Success(t *testing.T) {
	s := newTestServer()
	rid, ids := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	s.controller.MarkTaskDone(MarkTaskDoneInput{RequestID: rid, TaskID: ids[0]})
	s.controller.ApproveTaskCompletion(ApproveTaskInput{RequestID: rid, TaskID: ids[0]})
	rr := doJSON(s.handleApproveRequestCompletion, ApproveRequestInput{RequestID: rid})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["message"], "已完成并批准")
}

// ============================================================
// handleOpenTaskDetails
// ============================================================

func TestServer_HandleOpenTaskDetails_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleOpenTaskDetails)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleOpenTaskDetails_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleOpenTaskDetails, TaskDetailsInput{TaskID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleOpenTaskDetails_Success(t *testing.T) {
	s := newTestServer()
	_, ids := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := doJSON(s.handleOpenTaskDetails, TaskDetailsInput{TaskID: ids[0]})
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.NotNil(t, m["task"])
}

// ============================================================
// handleListRequests
// ============================================================

func TestServer_HandleListRequests_Empty(t *testing.T) {
	s := newTestServer()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp/list_requests", nil)
	s.handleListRequests(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.Len(t, m["requests"], 0)
	assert.Contains(t, m["message"], "0")
}

func TestServer_HandleListRequests_WithRequests(t *testing.T) {
	s := newTestServer()
	mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp/list_requests", nil)
	s.handleListRequests(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.Len(t, m["requests"], 1)
}

// ============================================================
// handleAddTasksToRequest
// ============================================================

func TestServer_HandleAddTasksToRequest_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleAddTasksToRequest)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleAddTasksToRequest_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleAddTasksToRequest, AddTasksInput{RequestID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleAddTasksToRequest_Success(t *testing.T) {
	s := newTestServer()
	rid, _ := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := doJSON(s.handleAddTasksToRequest, AddTasksInput{
		RequestID: rid,
		Tasks:     []TaskInput{{Title: "new1"}, {Title: "new2"}},
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	assert.Len(t, m["tasks"], 2)
}

// ============================================================
// handleUpdateTask
// ============================================================

func TestServer_HandleUpdateTask_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleUpdateTask)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleUpdateTask_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleUpdateTask, UpdateTaskInput{RequestID: "", TaskID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleUpdateTask_Success(t *testing.T) {
	s := newTestServer()
	rid, ids := mustPlanRequest(t, s, []TaskInput{{Title: "t1"}})
	rr := doJSON(s.handleUpdateTask, UpdateTaskInput{
		RequestID: rid, TaskID: ids[0], Title: "updated", Description: "desc",
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "任务信息已更新", decodeBody(t, rr)["message"])
}

// ============================================================
// handleDeleteTask
// ============================================================

func TestServer_HandleDeleteTask_InvalidJSON(t *testing.T) {
	s := newTestServer()
	rr := doInvalidJSON(s.handleDeleteTask)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestServer_HandleDeleteTask_ControllerError(t *testing.T) {
	s := newTestServer()
	rr := doJSON(s.handleDeleteTask, DeleteTaskInput{RequestID: "", TaskID: ""})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestServer_HandleDeleteTask_Success(t *testing.T) {
	s := newTestServer()
	rid, ids := mustPlanRequest(t, s, []TaskInput{{Title: "keep"}, {Title: "del"}})
	rr := doJSON(s.handleDeleteTask, DeleteTaskInput{RequestID: rid, TaskID: ids[1]})
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, decodeBody(t, rr)["message"], "已删除")
}

// ============================================================
// HandleXxx() http.HandlerFunc 包装器
// 覆盖每个包装器，确保其内部正确转发到对应的私有 handler
// ============================================================

func TestServer_HandleWrappers(t *testing.T) {
	s := newTestServer()

	// 每个 wrapper 返回非 nil
	wrappers := []http.HandlerFunc{
		s.HandleRequestPlanning(),
		s.HandleGetNextTask(),
		s.HandleMarkTaskDone(),
		s.HandleApproveTaskCompletion(),
		s.HandleApproveRequestCompletion(),
		s.HandleOpenTaskDetails(),
		s.HandleListRequests(),
		s.HandleAddTasksToRequest(),
		s.HandleUpdateTask(),
		s.HandleDeleteTask(),
	}
	for _, w := range wrappers {
		assert.NotNil(t, w)
	}

	// 对每个 wrapper 实际发起一次请求，确保闭包内的转发语句被执行
	// 1) RequestPlanning: 合法 -> 200
	rr := doJSON(s.HandleRequestPlanning(), RequestPlanningInput{
		OriginalRequest: "r", Tasks: []TaskInput{{Title: "t1"}, {Title: "t2"}},
	})
	assert.Equal(t, http.StatusOK, rr.Code)
	m := decodeBody(t, rr)
	rid := m["requestId"].(string)
	tids := make([]string, 0, 2)
	for _, tk := range m["tasks"].([]interface{}) {
		tids = append(tids, tk.(map[string]interface{})["id"].(string))
	}

	// 2) GetNextTask: 合法 -> 200
	rr = doJSON(s.HandleGetNextTask(), GetNextTaskInput{RequestID: rid})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 3) MarkTaskDone: 合法 -> 200
	rr = doJSON(s.HandleMarkTaskDone(), MarkTaskDoneInput{RequestID: rid, TaskID: tids[0]})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 4) ApproveTaskCompletion: 合法(先标记完成) -> 200
	rr = doJSON(s.HandleApproveTaskCompletion(), ApproveTaskInput{RequestID: rid, TaskID: tids[0]})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 5) ApproveRequestCompletion: 错误(任务2未批准) -> 500，但仍走闭包
	rr = doJSON(s.HandleApproveRequestCompletion(), ApproveRequestInput{RequestID: rid})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	// 6) OpenTaskDetails: 合法 -> 200
	rr = doJSON(s.HandleOpenTaskDetails(), TaskDetailsInput{TaskID: tids[0]})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 7) ListRequests: 合法 -> 200
	rr = httptest.NewRecorder()
	s.HandleListRequests()(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	// 8) AddTasksToRequest: 合法 -> 200
	rr = doJSON(s.HandleAddTasksToRequest(), AddTasksInput{
		RequestID: rid, Tasks: []TaskInput{{Title: "new"}},
	})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 9) UpdateTask: 合法(更新尚未完成的任务 tids[1]) -> 200
	rr = doJSON(s.HandleUpdateTask(), UpdateTaskInput{
		RequestID: rid, TaskID: tids[1], Title: "updated",
	})
	assert.Equal(t, http.StatusOK, rr.Code)

	// 10) DeleteTask: 合法(删除尚未完成的任务 tids[1]) -> 200
	rr = doJSON(s.HandleDeleteTask(), DeleteTaskInput{RequestID: rid, TaskID: tids[1]})
	assert.Equal(t, http.StatusOK, rr.Code)
}
