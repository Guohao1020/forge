package project

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

func userCtx(c *gin.Context) (userID, tenantID int64) {
	uid, _ := c.Get("user_id")
	tid, _ := c.Get("tenant_id")
	userID, _ = uid.(int64)
	tenantID, _ = tid.(int64)
	return
}

func (h *Handler) Create(c *gin.Context) {
	userID, tenantID := userCtx(c)
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.svc.Create(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, p)
}

func (h *Handler) List(c *gin.Context) {
	userID, tenantID := userCtx(c)
	var q ListProjectsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.svc.List(c.Request.Context(), tenantID, userID, &q)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetByID(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	p, err := h.svc.GetByID(c.Request.Context(), id, tenantID, userID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, p)
}

func (h *Handler) Update(c *gin.Context) {
	_, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	p, err := h.svc.Update(c.Request.Context(), id, tenantID, &req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, p)
}

func (h *Handler) Archive(c *gin.Context) {
	_, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Archive(c.Request.Context(), id, tenantID); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *Handler) Star(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Star(c.Request.Context(), id, tenantID, userID); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *Handler) Import(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请选择至少一个仓库导入")
		return
	}

	userID, tenantID := userCtx(c)
	result, err := h.svc.ImportFromGitHub(c.Request.Context(), tenantID, userID, &req)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "导入失败: "+err.Error())
		return
	}
	response.OK(c, result)
}

func (h *Handler) Unstar(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Unstar(c.Request.Context(), id, tenantID, userID); err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, nil)
}
