package middleware

import "testing"

func BenchmarkRateLimiter_Allow(b *testing.B) {
	rl := NewRateLimiter(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow("bench-key")
	}
}

func BenchmarkRateLimiter_MultiKey(b *testing.B) {
	rl := NewRateLimiter(100, 10)
	keys := []string{"user:1", "user:2", "user:3", "ip:1.2.3.4", "ip:5.6.7.8"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(keys[i%len(keys)])
	}
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := newBucket(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket.allow()
	}
}
