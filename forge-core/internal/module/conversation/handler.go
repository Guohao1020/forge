package conversation

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

// POST /api/projects/:id/tasks/:taskId/messages
func (h *Handler) SendMessage(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的项目ID")
		return
	}

	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请输入消息内容")
		return
	}

	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")

	result, err := h.service.SendMessage(c.Request.Context(),
		projectID, taskID, tenantID.(int64), userID.(int64), req.Content)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "发送消息失败")
		return
	}
	response.OK(c, result)
}

// GET /api/projects/:id/tasks/:taskId/messages
func (h *Handler) GetHistory(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	convs, err := h.service.GetHistory(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取对话历史失败")
		return
	}
	response.OK(c, &ConversationListResponse{Messages: convs})
}

// POST /api/projects/:id/tasks/:taskId/confirm
func (h *Handler) ConfirmPlan(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "无效的任务ID")
		return
	}

	tenantID, _ := c.Get("tenant_id")

	if err := h.service.ConfirmPlan(c.Request.Context(), taskID, tenantID.(int64)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "确认方案失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "方案已确认，生成流程已启动"})
}
