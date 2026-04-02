package testresult

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListTestResults(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}
	results, err := h.svc.ListByTask(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, TestResultListResponse{Results: results})
}

func (h *Handler) CreateTestResult(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var req CreateTestResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	req.TaskID = taskID

	result, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}
