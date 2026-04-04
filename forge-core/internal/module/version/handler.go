package version

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

func (h *Handler) Create(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)
	userID, _ := c.Get("user_id")
	uid, _ := userID.(int64)

	v, err := h.svc.Create(c.Request.Context(), tid, projectID, uid, &req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, v)
}

func (h *Handler) List(c *gin.Context) {
	projectID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid project id")
		return
	}
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)

	versions, err := h.svc.List(c.Request.Context(), tid, projectID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, VersionListResponse{Versions: versions})
}

func (h *Handler) Get(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid version id")
		return
	}
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)

	detail, err := h.svc.Get(c.Request.Context(), tid, versionID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, detail)
}

func (h *Handler) Update(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid version id")
		return
	}
	var req UpdateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)

	if err := h.svc.Update(c.Request.Context(), tid, versionID, &req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"status": "updated"})
}

func (h *Handler) Release(c *gin.Context) {
	versionID, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid version id")
		return
	}
	tenantID, _ := c.Get("tenant_id")
	tid, _ := tenantID.(int64)

	v, err := h.svc.Release(c.Request.Context(), tid, versionID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, v)
}
