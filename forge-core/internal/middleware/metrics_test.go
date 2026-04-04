package middleware

import (
	"strings"
	"testing"
)

func TestMetricsSnapshot(t *testing.T) {
	snap := GetMetrics()
	if snap.TotalRequests < 0 {
		t.Error("TotalRequests should be >= 0")
	}
	if snap.Uptime == "" {
		t.Error("Uptime should not be empty")
	}
	if snap.RequestsByPath == nil {
		t.Error("RequestsByPath should not be nil")
	}
	if snap.AI.CallsByModel == nil {
		t.Error("AI.CallsByModel should not be nil")
	}
	if snap.Tasks.ByStatus == nil {
		t.Error("Tasks.ByStatus should not be nil")
	}
}

func TestPrometheusFormat(t *testing.T) {
	output := PrometheusFormat()
	required := []string{
		"forge_http_requests_total",
		"forge_http_errors_total",
		"forge_http_latency_avg_ms",
		"forge_uptime_seconds",
		"forge_ai_calls_total",
		"forge_ai_tokens_total",
		"forge_ai_fallback_total",
		"forge_ai_latency_avg_ms",
		"forge_tasks_total",
		"forge_tasks_completed_total",
		"forge_tasks_failed_total",
		"forge_tasks_in_progress",
		"forge_sse_connections_active",
		"# TYPE",
	}
	for _, metric := range required {
		if !strings.Contains(output, metric) {
			t.Errorf("missing metric: %s", metric)
		}
	}
}

func TestRecordAICall(t *testing.T) {
	RecordAICall("claude-3-opus", 1500, 3200.5, false)
	RecordAICall("gpt-4", 800, 2100.0, true)

	snap := GetMetrics()
	if snap.AI.TotalCalls < 2 {
		t.Errorf("expected at least 2 AI calls, got %d", snap.AI.TotalCalls)
	}
	if snap.AI.TotalTokens < 2300 {
		t.Errorf("expected at least 2300 tokens, got %d", snap.AI.TotalTokens)
	}
	if snap.AI.TotalFallback < 1 {
		t.Errorf("expected at least 1 fallback, got %d", snap.AI.TotalFallback)
	}
	if snap.AI.CallsByModel["claude-3-opus"] < 1 {
		t.Error("missing claude-3-opus in CallsByModel")
	}
}

func TestRecordTaskEvent(t *testing.T) {
	RecordTaskEvent("created")
	RecordTaskEvent("created")
	RecordTaskEvent("completed")
	RecordTaskEvent("failed")

	snap := GetMetrics()
	if snap.Tasks.Total < 2 {
		t.Errorf("expected at least 2 tasks total, got %d", snap.Tasks.Total)
	}
	if snap.Tasks.Completed < 1 {
		t.Errorf("expected at least 1 completed, got %d", snap.Tasks.Completed)
	}
	if snap.Tasks.Failed < 1 {
		t.Errorf("expected at least 1 failed, got %d", snap.Tasks.Failed)
	}
}

func TestSSEMetrics(t *testing.T) {
	SSEConnect()
	SSEConnect()

	sseMetrics.mu.RLock()
	active := sseMetrics.active
	sseMetrics.mu.RUnlock()
	if active < 2 {
		t.Errorf("expected at least 2 active SSE, got %d", active)
	}

	SSEDisconnect()
	sseMetrics.mu.RLock()
	active = sseMetrics.active
	sseMetrics.mu.RUnlock()
	if active < 1 {
		t.Errorf("expected at least 1 active SSE after disconnect, got %d", active)
	}
}

func TestRecordStageDuration(t *testing.T) {
	RecordStageDuration("PLAN", 1500.0)
	RecordStageDuration("GENERATE", 8500.0)

	snap := GetMetrics()
	if snap.Tasks.StageAvgMs["PLAN"] != 1500.0 {
		t.Errorf("expected PLAN stage 1500ms, got %.2f", snap.Tasks.StageAvgMs["PLAN"])
	}
	if snap.Tasks.StageAvgMs["GENERATE"] != 8500.0 {
		t.Errorf("expected GENERATE stage 8500ms, got %.2f", snap.Tasks.StageAvgMs["GENERATE"])
	}

	// Verify it appears in Prometheus output
	output := PrometheusFormat()
	if !strings.Contains(output, "forge_pipeline_stage_duration_avg_ms") {
		t.Error("missing pipeline stage metrics in Prometheus output")
	}
}
