package task

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// ConversationCreator saves initial messages when tasks are created.
type ConversationCreator interface {
	CreateInitialMessage(ctx context.Context, taskID int64, content string) error
}

type Handler struct {
	service     *Service
	convCreator ConversationCreator
}

func NewHandler(service *Service, cc ConversationCreator) *Handler {
	return &Handler{service: service, convCreator: cc}
}

// POST /api/projects/:id/tasks
func (h *Handler) CreateTask(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请输入需求描述")
		return
	}

	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	result, err := h.service.CreateTask(c.Request.Context(),
		tenantID.(int64), projectID, userID.(int64), &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "创建任务失败")
		return
	}
	// Save requirement as the first conversation message
	if h.convCreator != nil {
		if err := h.convCreator.CreateInitialMessage(c.Request.Context(), result.Task.ID, req.Requirement); err != nil {
			slog.Warn("failed to save initial conversation message", "task_id", result.Task.ID, "error", err)
		}
	}

	response.OK(c, result)
}

// GET /api/projects/:id/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	result, err := h.service.ListTasks(c.Request.Context(), projectID, status, page, pageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取任务列表失败")
		return
	}
	response.OK(c, result)
}

// GET /api/projects/:id/tasks/:taskId/nodes
func (h *Handler) ListTaskNodes(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	result, err := h.service.ListTaskNodes(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取任务节点失败")
		return
	}
	response.OK(c, result)
}

// POST /api/projects/:id/tasks/:taskId/cancel
func (h *Handler) CancelTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	if err := h.service.CancelTask(c.Request.Context(), taskID); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "任务已取消"})
}

// GET /api/projects/:id/tasks/:taskId
func (h *Handler) GetTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	result, err := h.service.GetTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取任务详情失败")
		return
	}
	response.OK(c, result)
}
