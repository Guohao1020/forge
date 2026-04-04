package cost

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

// GET /api/admin/costs — tenant-wide monthly cost summary
func (h *Handler) GetMonthlyCosts(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	summary, err := h.svc.GetMonthlySummary(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, summary)
}

// GET /api/projects/:id/costs — project-level monthly cost summary
func (h *Handler) GetProjectCosts(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	summary, err := h.svc.GetProjectSummary(c.Request.Context(), tenantID, projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, summary)
}

// GET /api/admin/budget — current budget status
func (h *Handler) GetBudgetStatus(c *gin.Context) {
	tid, _ := c.Get("tenant_id")
	tenantID, _ := tid.(int64)

	status, err := h.svc.GetBudgetStatus(c.Request.Context(), tenantID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, status)
}
