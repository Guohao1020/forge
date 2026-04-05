package middleware

import (
	"sync"
	"testing"
)

func TestMetrics_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 50
	const iterations = 100

	wg.Add(goroutines * 3) // 3 types of operations

	// Concurrent AI call recording
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			models := []string{"claude", "gpt-4", "qwen"}
			for j := 0; j < iterations; j++ {
				RecordAICall(models[j%3], int64(j*100), float64(j*10), j%5 == 0)
			}
		}(i)
	}

	// Concurrent task events
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			events := []string{"created", "completed", "failed"}
			for j := 0; j < iterations; j++ {
				RecordTaskEvent(events[j%3])
			}
		}()
	}

	// Concurrent metrics reading
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				snap := GetMetrics()
				_ = snap.TotalRequests
				_ = snap.AI.TotalCalls
				_ = snap.Tasks.Total
				_ = PrometheusFormat()
			}
		}()
	}

	wg.Wait()

	// Verify no data corruption
	snap := GetMetrics()
	if snap.AI.TotalCalls < 0 {
		t.Error("AI calls should not be negative")
	}
	if snap.Tasks.Total < 0 {
		t.Error("task total should not be negative")
	}
}

func TestSSEMetrics_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	const goroutines = 100

	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			SSEConnect()
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			SSEDisconnect()
		}()
	}

	wg.Wait()
	// Should not panic or deadlock
}
