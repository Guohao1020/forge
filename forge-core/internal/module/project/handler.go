package project

import (
	"context"
	"log/slog"
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

// DetectTechStack triggers tech stack re-detection for a project.
func (h *Handler) DetectTechStack(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	go func() {
		bgCtx := context.Background()
		if err := h.svc.DetectTechStack(bgCtx, id, tenantID, userID); err != nil {
			slog.Warn("manual tech stack detection failed", "project_id", id, "error", err)
		}
	}()
	response.OK(c, gin.H{"status": "detection_started"})
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

// GetStats returns task/version/quality statistics for the project.
// GET /api/projects/:id/stats
func (h *Handler) GetStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	stats, err := h.svc.GetProjectStats(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, stats)
}

// SyncToRemote creates a GitHub repo for an existing project that has no remote.
// POST /api/projects/:id/sync
func (h *Handler) SyncToRemote(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Private bool `json:"private"`
	}
	_ = c.ShouldBindJSON(&req)

	p, err := h.svc.SyncProjectToRemote(c.Request.Context(), id, tenantID, userID, req.Private)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, p)
}

// ---------------------------------------------------------------------------
// Code browsing handlers
// ---------------------------------------------------------------------------

// GetCodeTree returns the repository file tree.
// GET /api/projects/:id/code/tree?ref=main
func (h *Handler) GetCodeTree(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	ref := c.DefaultQuery("ref", "")
	tree, err := h.svc.GetCodeTree(c.Request.Context(), id, tenantID, userID, ref)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文件树失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"files": tree, "ref": ref})
}

// GetCodeFile returns file content at a given path and ref.
// GET /api/projects/:id/code/file?path=src/main.go&ref=main
func (h *Handler) GetCodeFile(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	path := c.Query("path")
	ref := c.DefaultQuery("ref", "")
	if path == "" {
		response.Fail(c, http.StatusBadRequest, "path 参数必填")
		return
	}
	content, err := h.svc.GetCodeFile(c.Request.Context(), id, tenantID, userID, path, ref)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取文件内容失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"path": path, "content": content, "ref": ref})
}

// ListBranches returns all branches for the project's repo.
// GET /api/projects/:id/code/branches
func (h *Handler) ListBranches(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	branches, err := h.svc.ListBranches(c.Request.Context(), id, tenantID, userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取分支列表失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"branches": branches})
}

// ListPRs returns pull requests for the project's repo.
// GET /api/projects/:id/code/prs?state=open
func (h *Handler) ListPRs(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	state := c.DefaultQuery("state", "open")
	prs, err := h.svc.ListPRs(c.Request.Context(), id, tenantID, userID, state)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取 PR 列表失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"prs": prs})
}

// GetPRDetail returns changed files for a specific pull request.
// GET /api/projects/:id/code/prs/:prNumber
func (h *Handler) GetPRDetail(c *gin.Context) {
	userID, tenantID := userCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	prNumber, err := strconv.Atoi(c.Param("prNumber"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid PR number")
		return
	}
	files, err := h.svc.GetPRDetail(c.Request.Context(), id, tenantID, userID, prNumber)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取 PR 详情失败: "+err.Error())
		return
	}
	response.OK(c, gin.H{"files": files})
}
