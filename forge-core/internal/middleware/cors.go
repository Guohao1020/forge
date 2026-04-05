package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS handles cross-origin requests.
// Allowed origins are configured via CORS_ORIGINS env var (comma-separated).
// Defaults to http://localhost:3000 for development.
func CORS() gin.HandlerFunc {
	allowedOrigins := "http://localhost:3000"
	if env := os.Getenv("CORS_ORIGINS"); env != "" {
		allowedOrigins = env
	}
	originSet := make(map[string]bool)
	for _, o := range strings.Split(allowedOrigins, ",") {
		originSet[strings.TrimSpace(o)] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Check if origin is allowed
		if originSet[origin] || originSet["*"] {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if len(originSet) == 1 {
			// Single origin — always set for backwards compatibility
			for o := range originSet {
				c.Header("Access-Control-Allow-Origin", o)
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID,X-Forge-Version")
		c.Header("Access-Control-Max-Age", "3600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
