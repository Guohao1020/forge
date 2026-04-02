package pipeline

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

func (h *Handler) ListEnvironments(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	envs, err := h.svc.ListEnvironments(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, EnvironmentListResponse{Environments: envs})
}

func (h *Handler) GetEnvironment(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("envId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid environment id")
		return
	}
	env, err := h.svc.GetEnvironment(c.Request.Context(), envID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, env)
}

// --- Deploy Records ---

func (h *Handler) ListDeployRecords(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("envId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid environment id")
		return
	}
	records, err := h.svc.ListDeployRecords(c.Request.Context(), envID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, DeployRecordListResponse{Records: records})
}

func (h *Handler) TriggerDeploy(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	envID, err := strconv.ParseInt(c.Param("envId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid environment id")
		return
	}

	var req TriggerDeployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "version is required")
		return
	}

	// Extract user context from JWT
	tenantID, _ := c.Get("tenant_id")
	userID, _ := c.Get("user_id")
	tid, _ := tenantID.(int64)
	uid, _ := userID.(int64)

	record, err := h.svc.TriggerDeploy(c.Request.Context(), tid, projectID, envID, uid, req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, record)
}
