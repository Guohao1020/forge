package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shulex/forge/forge-core/internal/pkg/response"
)

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func newBucket(maxTokens, refillRate float64) *tokenBucket {
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimiter stores per-key buckets with automatic cleanup.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	max     float64
	rate    float64
}

// NewRateLimiter creates a rate limiter with the given max burst and refill rate.
// maxBurst: maximum tokens (requests) allowed in a burst.
// refillPerSecond: tokens refilled per second.
func NewRateLimiter(maxBurst, refillPerSecond float64) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		max:     maxBurst,
		rate:    refillPerSecond,
	}
	// Periodic cleanup of stale buckets
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getBucket(key string) *tokenBucket {
	rl.mu.RLock()
	b, ok := rl.buckets[key]
	rl.mu.RUnlock()
	if ok {
		return b
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	// Double-check
	if b, ok := rl.buckets[key]; ok {
		return b
	}
	b = newBucket(rl.max, rl.rate)
	rl.buckets[key] = b
	return b
}

func (rl *RateLimiter) Allow(key string) bool {
	return rl.getBucket(key).allow()
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.buckets {
			b.mu.Lock()
			if now.Sub(b.lastRefill) > 10*time.Minute {
				delete(rl.buckets, key)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware creates a Gin middleware that rate-limits by user ID or IP.
// Default: 60 requests/burst, 10 requests/second refill.
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := NewRateLimiter(60, 10)

	return func(c *gin.Context) {
		// Use user ID if authenticated, otherwise IP
		key := c.ClientIP()
		if uid, exists := c.Get("user_id"); exists {
			if userID, ok := uid.(int64); ok && userID > 0 {
				key = "user:" + c.GetString("username")
			}
		}

		if !limiter.Allow(key) {
			response.Fail(c, http.StatusTooManyRequests, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}
