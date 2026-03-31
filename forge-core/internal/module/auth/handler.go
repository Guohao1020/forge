package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// POST /api/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}

	resp, err := h.service.Login(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.OK(c, resp)
}

// POST /api/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	jti, exists := c.Get("token_jti")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	if err := h.service.Logout(c.Request.Context(), jti.(string)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "登出失败")
		return
	}

	response.OK(c, nil)
}

// GET /api/auth/me
func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	user, err := h.service.GetCurrentUser(c.Request.Context(), userID.(int64))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "获取用户信息失败")
		return
	}

	response.OK(c, user)
}

// GET /api/auth/github/authorize
func (h *Handler) GitHubAuthorize(c *gin.Context) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		response.Fail(c, http.StatusInternalServerError, "生成安全令牌失败")
		return
	}
	state := hex.EncodeToString(b)
	authorizeURL := h.service.GetGitHubAuthorizeURL(state)
	response.OK(c, GitHubAuthorizeResponse{AuthorizeURL: authorizeURL})
}

// GET /api/auth/github/callback?code=xxx
func (h *Handler) GitHubCallback(c *gin.Context) {
	var req GitHubCallbackRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "缺少 code 参数")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}

	result, err := h.service.HandleGitHubCallback(c.Request.Context(), userID.(int64), req.Code)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "GitHub 授权失败: "+err.Error())
		return
	}
	response.OK(c, result)
}

// GET /api/auth/github/status
func (h *Handler) GitHubStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	connected := h.service.HasGitHubConnection(c.Request.Context(), userID.(int64))
	response.OK(c, gin.H{"connected": connected})
}

// DELETE /api/auth/github/disconnect
func (h *Handler) GitHubDisconnect(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	if err := h.service.DisconnectGitHub(c.Request.Context(), userID.(int64)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "断开 GitHub 失败")
		return
	}
	response.OK(c, nil)
}

// GET /api/github/repos
func (h *Handler) ListGitHubRepos(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Fail(c, http.StatusUnauthorized, "未登录")
		return
	}
	repos, err := h.service.ListGitHubRepos(c.Request.Context(), userID.(int64))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "获取仓库列表失败: "+err.Error())
		return
	}
	response.OK(c, repos)
}
