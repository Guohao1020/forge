package entropy

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

// GET /api/projects/:id/entropy/latest
func (h *Handler) GetLatestScan(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	scan, err := h.service.GetLatestScan(c.Request.Context(), projectID)
	if err != nil {
		// No scans yet — return empty
		response.OK(c, gin.H{"scan": nil, "message": "no scans yet"})
		return
	}
	response.OK(c, gin.H{"scan": scan})
}

// GET /api/projects/:id/entropy/scans
func (h *Handler) ListScans(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	scans, err := h.service.ListScans(c.Request.Context(), projectID, limit)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"scans": scans})
}

// GET /api/projects/:id/entropy/trends
func (h *Handler) GetTrends(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	trends, err := h.service.GetQualityTrends(c.Request.Context(), projectID, days)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"trends": trends})
}

// GET /api/projects/:id/entropy/config
func (h *Handler) GetConfig(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	cfg, err := h.service.GetConfig(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"config": cfg})
}

// PUT /api/projects/:id/entropy/config
func (h *Handler) UpdateConfig(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.service.UpdateConfig(c.Request.Context(), projectID, tenantID, &req); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"status": "config_updated"})
}

// POST /api/projects/:id/entropy/scan
func (h *Handler) TriggerScan(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	workflowID, err := h.service.TriggerScan(c.Request.Context(), projectID, tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"workflow_id": workflowID, "status": "scan_started"})
}
