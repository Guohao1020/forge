package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/module/auth"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

func JWTAuth(authService *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Fail(c, http.StatusUnauthorized, "请先登录")
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "登录已过期，请重新登录")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("username", claims.Username)
		c.Set("token_jti", claims.ID)
		c.Next()
	}
}
