package middleware

import (
	"testing"
	"time"
)

func TestTokenBucketAllow(t *testing.T) {
	b := newBucket(5, 1)

	// Should allow 5 requests immediately (burst)
	for i := 0; i < 5; i++ {
		if !b.allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be rejected
	if b.allow() {
		t.Error("6th request should be rejected (burst exhausted)")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	b := newBucket(2, 100) // 100 tokens/sec = very fast refill

	// Exhaust tokens
	b.allow()
	b.allow()
	if b.allow() {
		t.Error("should be exhausted")
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond) // should refill ~5 tokens at 100/sec

	if !b.allow() {
		t.Error("should have refilled")
	}
}

func TestRateLimiterPerKey(t *testing.T) {
	rl := NewRateLimiter(3, 1)

	// Key A gets 3 requests
	for i := 0; i < 3; i++ {
		if !rl.Allow("a") {
			t.Errorf("key A request %d should be allowed", i+1)
		}
	}
	if rl.Allow("a") {
		t.Error("key A 4th request should be rejected")
	}

	// Key B should still have its own budget
	if !rl.Allow("b") {
		t.Error("key B first request should be allowed")
	}
}

func TestRateLimiterConcurrent(t *testing.T) {
	rl := NewRateLimiter(100, 10)

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			rl.Allow("concurrent-key")
			done <- true
		}()
	}

	for i := 0; i < 50; i++ {
		<-done
	}
	// If we get here without deadlock/panic, concurrent access is safe
}
