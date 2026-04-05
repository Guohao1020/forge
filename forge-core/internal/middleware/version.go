package middleware

import (
	"github.com/gin-gonic/gin"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// VersionHeader adds X-Forge-Version header to all responses.
func VersionHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Forge-Version", Version)
		c.Next()
	}
}
