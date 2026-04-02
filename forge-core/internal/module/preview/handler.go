package preview

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

// ListPreviews returns all active preview environments for a project.
func (h *Handler) ListPreviews(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	envs, err := h.svc.ListPreviews(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, PreviewListResponse{Previews: envs})
}

// GetPreviewByTask returns the preview environment for a specific task.
func (h *Handler) GetPreviewByTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}
	env, err := h.svc.GetPreviewByTaskID(c.Request.Context(), taskID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if env == nil {
		response.Fail(c, http.StatusNotFound, "preview not found")
		return
	}
	response.OK(c, env)
}

type createPreviewRequest struct {
	BranchName string `json:"branchName"`
	PRNumber   int    `json:"prNumber"`
}

// CreatePreview manually creates a mock preview environment for a task.
func (h *Handler) CreatePreview(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var req createPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body — branch/pr can be optional for manual creation
		req = createPreviewRequest{}
	}

	// Extract tenant_id from JWT context (set by auth middleware)
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)
	if tid == 0 {
		tid = 1 // fallback for dev
	}

	env, err := h.svc.CreatePreview(c.Request.Context(), tid, projectID, taskID, req.BranchName, req.PRNumber)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, env)
}

// DestroyPreview marks a preview environment as destroyed.
func (h *Handler) DestroyPreview(c *gin.Context) {
	previewID, err := strconv.ParseInt(c.Param("previewId"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid preview id")
		return
	}
	if err := h.svc.DestroyPreview(c.Request.Context(), previewID); err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, gin.H{"destroyed": true})
}
