package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// Timeout creates middleware that cancels the request context after the given duration.
// SSE and streaming endpoints are excluded by checking for text/event-stream Accept header.
func Timeout(d time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip timeout for SSE/streaming endpoints
		if c.GetHeader("Accept") == "text/event-stream" {
			c.Next()
			return
		}

		// Skip for long-running paths
		path := c.FullPath()
		if path == "/api/stream/tasks/:taskId" {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), d)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// If context timed out and response hasn't been written
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			response.Fail(c, http.StatusGatewayTimeout, "request timeout")
			c.Abort()
		}
	}
}
