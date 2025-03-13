package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Server 是MCP服务器，提供HTTP API处理MCP协议
type Server struct {
	controller *Controller
	logger     *logrus.Logger
}

// NewServer 创建一个新的MCP服务器
func NewServer(logger *logrus.Logger) *Server {
	return &Server{
		controller: NewController(),
		logger:     logger,
	}
}

// RegisterRoutes 注册MCP协议的路由
func (s *Server) RegisterRoutes(router *mux.Router) {
	// 主要MCP端点
	router.HandleFunc("/mcp/request_planning", s.handleRequestPlanning).Methods("POST")
	router.HandleFunc("/mcp/get_next_task", s.handleGetNextTask).Methods("POST")
	router.HandleFunc("/mcp/mark_task_done", s.handleMarkTaskDone).Methods("POST")
	router.HandleFunc("/mcp/approve_task_completion", s.handleApproveTaskCompletion).Methods("POST")
	router.HandleFunc("/mcp/approve_request_completion", s.handleApproveRequestCompletion).Methods("POST")

	// 辅助端点
	router.HandleFunc("/mcp/open_task_details", s.handleOpenTaskDetails).Methods("POST")
	router.HandleFunc("/mcp/list_requests", s.handleListRequests).Methods("GET")
	router.HandleFunc("/mcp/add_tasks_to_request", s.handleAddTasksToRequest).Methods("POST")
	router.HandleFunc("/mcp/update_task", s.handleUpdateTask).Methods("POST")
	router.HandleFunc("/mcp/delete_task", s.handleDeleteTask).Methods("POST")
}

// 响应包装器
func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Errorf("无法编码响应: %v", err)
	}
}

// 错误响应
func (s *Server) respondError(w http.ResponseWriter, statusCode int, message string) {
	s.respondJSON(w, statusCode, map[string]string{"error": message})
}

// MCP请求规划
func (s *Server) handleRequestPlanning(w http.ResponseWriter, r *http.Request) {
	var input RequestPlanningInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.PlanRequest(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP获取下一个任务
func (s *Server) handleGetNextTask(w http.ResponseWriter, r *http.Request) {
	var input GetNextTaskInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.GetNextTask(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP标记任务完成
func (s *Server) handleMarkTaskDone(w http.ResponseWriter, r *http.Request) {
	var input MarkTaskDoneInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.MarkTaskDone(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP批准任务完成
func (s *Server) handleApproveTaskCompletion(w http.ResponseWriter, r *http.Request) {
	var input ApproveTaskInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.ApproveTaskCompletion(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP批准请求完成
func (s *Server) handleApproveRequestCompletion(w http.ResponseWriter, r *http.Request) {
	var input ApproveRequestInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.ApproveRequestCompletion(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP查看任务详情
func (s *Server) handleOpenTaskDetails(w http.ResponseWriter, r *http.Request) {
	var input TaskDetailsInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.GetTaskDetails(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP列出所有请求
func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	output, err := s.controller.ListRequests()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP向请求添加任务
func (s *Server) handleAddTasksToRequest(w http.ResponseWriter, r *http.Request) {
	var input AddTasksInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.AddTasksToRequest(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP更新任务
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	var input UpdateTaskInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.UpdateTask(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}

// MCP删除任务
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	var input DeleteTaskInput

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.respondError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	output, err := s.controller.DeleteTask(input)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, output)
}
