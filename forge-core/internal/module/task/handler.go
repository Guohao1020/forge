package task

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
