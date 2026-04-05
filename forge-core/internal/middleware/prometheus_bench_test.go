package middleware

import "testing"

func BenchmarkPrometheusFormat(b *testing.B) {
	// Populate some metrics
	for i := 0; i < 100; i++ {
		RecordAICall("qwen3", 500, 100.0, false)
		RecordTaskEvent("created")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PrometheusFormat()
	}
}

func BenchmarkGetMetrics(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetMetrics()
	}
}
