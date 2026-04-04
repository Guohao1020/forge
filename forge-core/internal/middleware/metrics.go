package middleware

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Metrics collects basic HTTP request metrics.
// Thread-safe for concurrent access.
type Metrics struct {
	mu             sync.RWMutex
	totalRequests  int64
	totalErrors    int64 // 5xx responses
	requestsByPath map[string]int64
	latencySum     float64 // total milliseconds
	latencyCount   int64
}

// Global metrics instance
var httpMetrics = &Metrics{
	requestsByPath: make(map[string]int64),
}

// MetricsMiddleware records request count, errors, and latency.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := float64(time.Since(start).Milliseconds())
		status := c.Writer.Status()

		httpMetrics.mu.Lock()
		httpMetrics.totalRequests++
		httpMetrics.latencySum += latency
		httpMetrics.latencyCount++
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		httpMetrics.requestsByPath[path]++
		if status >= 500 {
			httpMetrics.totalErrors++
		}
		httpMetrics.mu.Unlock()
	}
}

// MetricsSnapshot returns current metrics values.
type MetricsSnapshot struct {
	TotalRequests  int64              `json:"totalRequests"`
	TotalErrors    int64              `json:"totalErrors"`
	AvgLatencyMs   float64            `json:"avgLatencyMs"`
	RequestsByPath map[string]int64   `json:"requestsByPath"`
	Uptime         string             `json:"uptime"`
}

var startTime = time.Now()

// GetMetrics returns a snapshot of current metrics.
func GetMetrics() MetricsSnapshot {
	httpMetrics.mu.RLock()
	defer httpMetrics.mu.RUnlock()

	avgLatency := float64(0)
	if httpMetrics.latencyCount > 0 {
		avgLatency = httpMetrics.latencySum / float64(httpMetrics.latencyCount)
	}

	pathsCopy := make(map[string]int64, len(httpMetrics.requestsByPath))
	for k, v := range httpMetrics.requestsByPath {
		pathsCopy[k] = v
	}

	return MetricsSnapshot{
		TotalRequests:  httpMetrics.totalRequests,
		TotalErrors:    httpMetrics.totalErrors,
		AvgLatencyMs:   avgLatency,
		RequestsByPath: pathsCopy,
		Uptime:         time.Since(startTime).Truncate(time.Second).String(),
	}
}

// PrometheusFormat returns metrics in Prometheus text exposition format.
func PrometheusFormat() string {
	httpMetrics.mu.RLock()
	defer httpMetrics.mu.RUnlock()

	avgLatency := float64(0)
	if httpMetrics.latencyCount > 0 {
		avgLatency = httpMetrics.latencySum / float64(httpMetrics.latencyCount)
	}

	out := "# HELP forge_http_requests_total Total HTTP requests\n"
	out += "# TYPE forge_http_requests_total counter\n"
	out += "forge_http_requests_total " + strconv.FormatInt(httpMetrics.totalRequests, 10) + "\n"
	out += "# HELP forge_http_errors_total Total HTTP 5xx errors\n"
	out += "# TYPE forge_http_errors_total counter\n"
	out += "forge_http_errors_total " + strconv.FormatInt(httpMetrics.totalErrors, 10) + "\n"
	out += "# HELP forge_http_latency_avg_ms Average HTTP latency in milliseconds\n"
	out += "# TYPE forge_http_latency_avg_ms gauge\n"
	out += "forge_http_latency_avg_ms " + strconv.FormatFloat(avgLatency, 'f', 2, 64) + "\n"
	out += "# HELP forge_uptime_seconds Server uptime in seconds\n"
	out += "# TYPE forge_uptime_seconds gauge\n"
	out += "forge_uptime_seconds " + strconv.FormatFloat(time.Since(startTime).Seconds(), 'f', 0, 64) + "\n"

	return out
}
