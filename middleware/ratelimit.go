package middleware

import (
	"net/http"
	"sync"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens    float64
	lastRefil time.Time
	mu        sync.Mutex
}

type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
}

var globalRateLimiter = &RateLimiter{buckets: make(map[string]*bucket)}

func init() {
	// Sweep stale rate-limit buckets every 5 minutes.
	// A bucket is stale if it hasn't been refilled in over 10 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			globalRateLimiter.sweep()
		}
	}()
}

func (rl *RateLimiter) sweep() {
	cutoff := time.Now().Add(-10 * time.Minute)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, b := range rl.buckets {
		b.mu.Lock()
		stale := b.lastRefil.Before(cutoff)
		b.mu.Unlock()
		if stale {
			delete(rl.buckets, key)
		}
	}
}

func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, _ := c.Get("session_user")
		key, _ := user.(string)
		if key == "" {
			key = c.ClientIP()
		}

		settings := config.GetRateLimits()
		rule := settings.Default
		if r, ok := settings.Users[key]; ok {
			rule = r
		}

		if rule.RequestsPerMinute <= 0 {
			c.Next()
			return
		}

		if !globalRateLimiter.allow(key, rule) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(key string, rule config.RateLimitRule) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		burst := rule.Burst
	if burst <= 0 {
		burst = 1
	}
	b = &bucket{tokens: float64(burst), lastRefil: time.Now()}
		rl.buckets[key] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefil).Seconds()
	rate := float64(rule.RequestsPerMinute) / 60.0
	b.tokens += elapsed * rate
	if b.tokens > float64(rule.Burst) {
		b.tokens = float64(rule.Burst)
	}
	b.lastRefil = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
