package auth

import (
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
