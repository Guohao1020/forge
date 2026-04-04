package middleware

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Metrics collects HTTP request metrics with Prometheus-compatible labels.
type Metrics struct {
	mu             sync.RWMutex
	totalRequests  int64
	totalErrors    int64 // 5xx responses
	requestsByPath map[string]int64
	errorsByPath   map[string]int64
	latencySum     float64 // total milliseconds
	latencyCount   int64
	latencyByPath  map[string]float64
	latencyCountBy map[string]int64
}

// AI metrics — populated by task/cost modules via RecordAICall.
type AIMetrics struct {
	mu            sync.RWMutex
	totalCalls    int64
	totalTokens   int64
	totalFallback int64
	latencySum    float64
	latencyCount  int64
	callsByModel  map[string]int64
	tokensByModel map[string]int64
}

// Task metrics — populated by task module via RecordTaskEvent.
type TaskMetrics struct {
	mu          sync.RWMutex
	total       int64
	completed   int64
	failed      int64
	inProgress  int64
	byStatus    map[string]int64
	stageDurAvg map[string]float64 // avg ms per pipeline stage
}

// SSE metrics
type SSEMetrics struct {
	mu     sync.RWMutex
	active int64
}

// Global instances
var (
	httpMetrics = &Metrics{
		requestsByPath: make(map[string]int64),
		errorsByPath:   make(map[string]int64),
		latencyByPath:  make(map[string]float64),
		latencyCountBy: make(map[string]int64),
	}
	aiMetrics = &AIMetrics{
		callsByModel:  make(map[string]int64),
		tokensByModel: make(map[string]int64),
	}
	taskMetrics = &TaskMetrics{
		byStatus:    make(map[string]int64),
		stageDurAvg: make(map[string]float64),
	}
	sseMetrics = &SSEMetrics{}
	startTime  = time.Now()
)

// MetricsMiddleware records request count, errors, and latency per path.
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
		httpMetrics.latencyByPath[path] += latency
		httpMetrics.latencyCountBy[path]++
		if status >= 500 {
			httpMetrics.totalErrors++
			httpMetrics.errorsByPath[path]++
		}
		httpMetrics.mu.Unlock()
	}
}

// RecordAICall records a single AI/LLM call for metrics.
func RecordAICall(model string, tokens int64, latencyMs float64, isFallback bool) {
	aiMetrics.mu.Lock()
	aiMetrics.totalCalls++
	aiMetrics.totalTokens += tokens
	aiMetrics.latencySum += latencyMs
	aiMetrics.latencyCount++
	aiMetrics.callsByModel[model]++
	aiMetrics.tokensByModel[model] += tokens
	if isFallback {
		aiMetrics.totalFallback++
	}
	aiMetrics.mu.Unlock()
}

// RecordTaskEvent updates task metrics counters.
func RecordTaskEvent(event string) {
	taskMetrics.mu.Lock()
	switch event {
	case "created":
		taskMetrics.total++
		taskMetrics.inProgress++
		taskMetrics.byStatus["in_progress"]++
	case "completed":
		taskMetrics.completed++
		if taskMetrics.inProgress > 0 {
			taskMetrics.inProgress--
		}
		taskMetrics.byStatus["completed"]++
		if taskMetrics.byStatus["in_progress"] > 0 {
			taskMetrics.byStatus["in_progress"]--
		}
	case "failed":
		taskMetrics.failed++
		if taskMetrics.inProgress > 0 {
			taskMetrics.inProgress--
		}
		taskMetrics.byStatus["failed"]++
		if taskMetrics.byStatus["in_progress"] > 0 {
			taskMetrics.byStatus["in_progress"]--
		}
	}
	taskMetrics.mu.Unlock()
}

// RecordStageDuration records avg pipeline stage time.
func RecordStageDuration(stage string, ms float64) {
	taskMetrics.mu.Lock()
	taskMetrics.stageDurAvg[stage] = ms
	taskMetrics.mu.Unlock()
}

// SSEConnect increments active SSE connections.
func SSEConnect() {
	sseMetrics.mu.Lock()
	sseMetrics.active++
	sseMetrics.mu.Unlock()
}

// SSEDisconnect decrements active SSE connections.
func SSEDisconnect() {
	sseMetrics.mu.Lock()
	if sseMetrics.active > 0 {
		sseMetrics.active--
	}
	sseMetrics.mu.Unlock()
}

// MetricsSnapshot returns current metrics values (for JSON endpoint).
type MetricsSnapshot struct {
	TotalRequests  int64            `json:"totalRequests"`
	TotalErrors    int64            `json:"totalErrors"`
	AvgLatencyMs   float64          `json:"avgLatencyMs"`
	RequestsByPath map[string]int64 `json:"requestsByPath"`
	Uptime         string           `json:"uptime"`
	AI             AISnapshot       `json:"ai"`
	Tasks          TaskSnapshot     `json:"tasks"`
	SSEActive      int64            `json:"sseActive"`
}

type AISnapshot struct {
	TotalCalls    int64            `json:"totalCalls"`
	TotalTokens   int64            `json:"totalTokens"`
	TotalFallback int64            `json:"totalFallback"`
	AvgLatencyMs  float64          `json:"avgLatencyMs"`
	CallsByModel  map[string]int64 `json:"callsByModel"`
	TokensByModel map[string]int64 `json:"tokensByModel"`
}

type TaskSnapshot struct {
	Total      int64              `json:"total"`
	Completed  int64              `json:"completed"`
	Failed     int64              `json:"failed"`
	InProgress int64              `json:"inProgress"`
	ByStatus   map[string]int64   `json:"byStatus"`
	StageAvgMs map[string]float64 `json:"stageAvgMs"`
}

// GetMetrics returns a full snapshot.
func GetMetrics() MetricsSnapshot {
	httpMetrics.mu.RLock()
	avgLatency := float64(0)
	if httpMetrics.latencyCount > 0 {
		avgLatency = httpMetrics.latencySum / float64(httpMetrics.latencyCount)
	}
	pathsCopy := make(map[string]int64, len(httpMetrics.requestsByPath))
	for k, v := range httpMetrics.requestsByPath {
		pathsCopy[k] = v
	}
	httpMetrics.mu.RUnlock()

	aiMetrics.mu.RLock()
	aiAvg := float64(0)
	if aiMetrics.latencyCount > 0 {
		aiAvg = aiMetrics.latencySum / float64(aiMetrics.latencyCount)
	}
	aiCallsCopy := make(map[string]int64, len(aiMetrics.callsByModel))
	for k, v := range aiMetrics.callsByModel {
		aiCallsCopy[k] = v
	}
	aiTokensCopy := make(map[string]int64, len(aiMetrics.tokensByModel))
	for k, v := range aiMetrics.tokensByModel {
		aiTokensCopy[k] = v
	}
	aiMetrics.mu.RUnlock()

	taskMetrics.mu.RLock()
	statusCopy := make(map[string]int64, len(taskMetrics.byStatus))
	for k, v := range taskMetrics.byStatus {
		statusCopy[k] = v
	}
	stageCopy := make(map[string]float64, len(taskMetrics.stageDurAvg))
	for k, v := range taskMetrics.stageDurAvg {
		stageCopy[k] = v
	}
	taskMetrics.mu.RUnlock()

	sseMetrics.mu.RLock()
	sseActive := sseMetrics.active
	sseMetrics.mu.RUnlock()

	return MetricsSnapshot{
		TotalRequests:  httpMetrics.totalRequests,
		TotalErrors:    httpMetrics.totalErrors,
		AvgLatencyMs:   avgLatency,
		RequestsByPath: pathsCopy,
		Uptime:         time.Since(startTime).Truncate(time.Second).String(),
		AI: AISnapshot{
			TotalCalls:    aiMetrics.totalCalls,
			TotalTokens:   aiMetrics.totalTokens,
			TotalFallback: aiMetrics.totalFallback,
			AvgLatencyMs:  aiAvg,
			CallsByModel:  aiCallsCopy,
			TokensByModel: aiTokensCopy,
		},
		Tasks: TaskSnapshot{
			Total:      taskMetrics.total,
			Completed:  taskMetrics.completed,
			Failed:     taskMetrics.failed,
			InProgress: taskMetrics.inProgress,
			ByStatus:   statusCopy,
			StageAvgMs: stageCopy,
		},
		SSEActive: sseActive,
	}
}

// PrometheusFormat returns all metrics in Prometheus text exposition format.
func PrometheusFormat() string {
	var b strings.Builder

	// --- HTTP Metrics ---
	httpMetrics.mu.RLock()
	avgLatency := float64(0)
	if httpMetrics.latencyCount > 0 {
		avgLatency = httpMetrics.latencySum / float64(httpMetrics.latencyCount)
	}

	b.WriteString("# HELP forge_http_requests_total Total HTTP requests\n")
	b.WriteString("# TYPE forge_http_requests_total counter\n")
	b.WriteString("forge_http_requests_total ")
	b.WriteString(strconv.FormatInt(httpMetrics.totalRequests, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_http_errors_total Total HTTP 5xx errors\n")
	b.WriteString("# TYPE forge_http_errors_total counter\n")
	b.WriteString("forge_http_errors_total ")
	b.WriteString(strconv.FormatInt(httpMetrics.totalErrors, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_http_latency_avg_ms Average HTTP latency in milliseconds\n")
	b.WriteString("# TYPE forge_http_latency_avg_ms gauge\n")
	b.WriteString("forge_http_latency_avg_ms ")
	b.WriteString(strconv.FormatFloat(avgLatency, 'f', 2, 64))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_http_requests_by_path HTTP requests by path\n")
	b.WriteString("# TYPE forge_http_requests_by_path counter\n")
	for path, count := range httpMetrics.requestsByPath {
		fmt.Fprintf(&b, "forge_http_requests_by_path{path=%q} %d\n", path, count)
	}

	b.WriteString("# HELP forge_http_errors_by_path HTTP 5xx errors by path\n")
	b.WriteString("# TYPE forge_http_errors_by_path counter\n")
	for path, count := range httpMetrics.errorsByPath {
		fmt.Fprintf(&b, "forge_http_errors_by_path{path=%q} %d\n", path, count)
	}
	httpMetrics.mu.RUnlock()

	// --- Uptime ---
	b.WriteString("# HELP forge_uptime_seconds Server uptime in seconds\n")
	b.WriteString("# TYPE forge_uptime_seconds gauge\n")
	b.WriteString("forge_uptime_seconds ")
	b.WriteString(strconv.FormatFloat(time.Since(startTime).Seconds(), 'f', 0, 64))
	b.WriteByte('\n')

	// --- AI Metrics ---
	aiMetrics.mu.RLock()
	aiAvg := float64(0)
	if aiMetrics.latencyCount > 0 {
		aiAvg = aiMetrics.latencySum / float64(aiMetrics.latencyCount)
	}

	b.WriteString("# HELP forge_ai_calls_total Total AI/LLM calls\n")
	b.WriteString("# TYPE forge_ai_calls_total counter\n")
	b.WriteString("forge_ai_calls_total ")
	b.WriteString(strconv.FormatInt(aiMetrics.totalCalls, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_ai_tokens_total Total AI tokens consumed\n")
	b.WriteString("# TYPE forge_ai_tokens_total counter\n")
	b.WriteString("forge_ai_tokens_total ")
	b.WriteString(strconv.FormatInt(aiMetrics.totalTokens, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_ai_fallback_total Total AI fallback events\n")
	b.WriteString("# TYPE forge_ai_fallback_total counter\n")
	b.WriteString("forge_ai_fallback_total ")
	b.WriteString(strconv.FormatInt(aiMetrics.totalFallback, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_ai_latency_avg_ms Average AI call latency in milliseconds\n")
	b.WriteString("# TYPE forge_ai_latency_avg_ms gauge\n")
	b.WriteString("forge_ai_latency_avg_ms ")
	b.WriteString(strconv.FormatFloat(aiAvg, 'f', 2, 64))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_ai_calls_by_model AI calls by model\n")
	b.WriteString("# TYPE forge_ai_calls_by_model counter\n")
	for model, count := range aiMetrics.callsByModel {
		fmt.Fprintf(&b, "forge_ai_calls_by_model{model=%q} %d\n", model, count)
	}

	b.WriteString("# HELP forge_ai_tokens_by_model Tokens consumed by model\n")
	b.WriteString("# TYPE forge_ai_tokens_by_model counter\n")
	for model, tokens := range aiMetrics.tokensByModel {
		fmt.Fprintf(&b, "forge_ai_tokens_by_model{model=%q} %d\n", model, tokens)
	}
	aiMetrics.mu.RUnlock()

	// --- Task Metrics ---
	taskMetrics.mu.RLock()
	b.WriteString("# HELP forge_tasks_total Total tasks created\n")
	b.WriteString("# TYPE forge_tasks_total counter\n")
	b.WriteString("forge_tasks_total ")
	b.WriteString(strconv.FormatInt(taskMetrics.total, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_tasks_completed_total Total tasks completed\n")
	b.WriteString("# TYPE forge_tasks_completed_total counter\n")
	b.WriteString("forge_tasks_completed_total ")
	b.WriteString(strconv.FormatInt(taskMetrics.completed, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_tasks_failed_total Total tasks failed\n")
	b.WriteString("# TYPE forge_tasks_failed_total counter\n")
	b.WriteString("forge_tasks_failed_total ")
	b.WriteString(strconv.FormatInt(taskMetrics.failed, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_tasks_in_progress Current tasks in progress\n")
	b.WriteString("# TYPE forge_tasks_in_progress gauge\n")
	b.WriteString("forge_tasks_in_progress ")
	b.WriteString(strconv.FormatInt(taskMetrics.inProgress, 10))
	b.WriteByte('\n')

	b.WriteString("# HELP forge_tasks_by_status Tasks by status\n")
	b.WriteString("# TYPE forge_tasks_by_status gauge\n")
	for status, count := range taskMetrics.byStatus {
		fmt.Fprintf(&b, "forge_tasks_by_status{status=%q} %d\n", status, count)
	}

	b.WriteString("# HELP forge_pipeline_stage_duration_avg_ms Average pipeline stage duration\n")
	b.WriteString("# TYPE forge_pipeline_stage_duration_avg_ms gauge\n")
	for stage, ms := range taskMetrics.stageDurAvg {
		fmt.Fprintf(&b, "forge_pipeline_stage_duration_avg_ms{stage=%q} %.2f\n", stage, ms)
	}
	taskMetrics.mu.RUnlock()

	// --- SSE Metrics ---
	sseMetrics.mu.RLock()
	b.WriteString("# HELP forge_sse_connections_active Active SSE connections\n")
	b.WriteString("# TYPE forge_sse_connections_active gauge\n")
	b.WriteString("forge_sse_connections_active ")
	b.WriteString(strconv.FormatInt(sseMetrics.active, 10))
	b.WriteByte('\n')
	sseMetrics.mu.RUnlock()

	return b.String()
}
