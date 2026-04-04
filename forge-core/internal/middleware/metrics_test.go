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
}

func TestPrometheusFormat(t *testing.T) {
	output := PrometheusFormat()
	if !strings.Contains(output, "forge_http_requests_total") {
		t.Error("missing forge_http_requests_total")
	}
	if !strings.Contains(output, "forge_http_errors_total") {
		t.Error("missing forge_http_errors_total")
	}
	if !strings.Contains(output, "forge_http_latency_avg_ms") {
		t.Error("missing forge_http_latency_avg_ms")
	}
	if !strings.Contains(output, "forge_uptime_seconds") {
		t.Error("missing forge_uptime_seconds")
	}
	if !strings.Contains(output, "# TYPE") {
		t.Error("missing TYPE annotations")
	}
}
