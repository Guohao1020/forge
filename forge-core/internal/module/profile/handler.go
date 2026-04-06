package profile

import (
	"encoding/json"
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

func (h *Handler) ListProfiles(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	profiles, err := h.svc.ListProfiles(c.Request.Context(), projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, ProfileListResponse{Profiles: profiles})
}

func (h *Handler) GetProfile(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	key := c.Param("key")
	entry, err := h.svc.GetProfile(c.Request.Context(), projectID, key)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, entry)
}

func (h *Handler) TriggerScan(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}

	var req ScanRequest
	_ = c.ShouldBindJSON(&req) // optional body with keys and branches

	userID, _ := c.Get("user_id")
	uid, _ := userID.(int64)

	workflowIDs, err := h.svc.TriggerScan(c.Request.Context(), projectID, uid, req.Keys, req.Branches)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to trigger scan: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"status":      "scan_started",
		"workflowIds": workflowIDs,
	})
}

// SaveProfile handles PUT /api/projects/:id/profiles/:key — called by Python ai-worker to save scan results.
func (h *Handler) SaveProfile(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	key := c.Param("key")
	if key == "" {
		response.Fail(c, http.StatusBadRequest, "profile key is required")
		return
	}

	var body json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	entry, err := h.svc.SaveProfile(c.Request.Context(), projectID, key, body)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "failed to save profile: "+err.Error())
		return
	}
	response.OK(c, entry)
}
