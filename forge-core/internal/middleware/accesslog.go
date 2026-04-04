package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog logs each HTTP request in structured JSON format for Loki ingestion.
// Logs: method, path, status, latency, client IP, user ID (if authenticated).
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.String("ip", c.ClientIP()),
			slog.Int("size", c.Writer.Size()),
		}

		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}

		if uid, exists := c.Get("user_id"); exists {
			if userID, ok := uid.(int64); ok && userID > 0 {
				attrs = append(attrs, slog.Int64("user_id", userID))
			}
		}

		if reqID := c.Writer.Header().Get("X-Request-ID"); reqID != "" {
			attrs = append(attrs, slog.String("request_id", reqID))
		}

		// Log at appropriate level based on status
		args := make([]any, len(attrs))
		for i, a := range attrs {
			args[i] = a
		}

		switch {
		case status >= 500:
			slog.Error("http request", args...)
		case status >= 400:
			slog.Warn("http request", args...)
		default:
			slog.Info("http request", args...)
		}
	}
}
