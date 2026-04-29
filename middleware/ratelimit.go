package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"apig0/config"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
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
		stale := b.lastRefill.Before(cutoff)
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
		overrideRule, hasTokenOverride := c.Get("api_token_rate_limit_rule")
		tokenID := strings.TrimSpace(c.GetString("api_token_id"))
		tokenRule, _ := overrideRule.(config.RateLimitRule)
		if key == "" {
			key = c.ClientIP()
		}

		settings := config.GetRateLimits()
		rule := settings.Default
		if hasTokenOverride && tokenRule.RequestsPerMinute > 0 {
			rule = config.NormalizeRateLimitRule(tokenRule)
			if tokenID != "" {
				key = "token:" + tokenID
			}
		} else if r, ok := settings.Users[key]; ok {
			rule = r
		}

		if rule.RequestsPerMinute <= 0 {
			c.Next()
			return
		}

		allowed, retryAfter := globalRateLimiter.allow(key, rule)
		c.Header("X-RateLimit-Limit", strconv.Itoa(rule.RequestsPerMinute))
		c.Header("X-RateLimit-Window", "60")
		c.Header("X-RateLimit-Burst", strconv.Itoa(rule.Burst))
		if !allowed {
			if retryAfter < time.Second {
				retryAfter = time.Second
			}
			retrySeconds := int(retryAfter.Seconds())
			c.Header("Retry-After", strconv.Itoa(retrySeconds))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":               "rate limit exceeded",
				"limit_type":          "token_bucket",
				"sustained_limit_rpm": rule.RequestsPerMinute,
				"burst_allowance":     rule.Burst,
				"retry_after_seconds": retrySeconds,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (rl *RateLimiter) allow(key string, rule config.RateLimitRule) (bool, time.Duration) {
	rule = config.NormalizeRateLimitRule(rule)
	if rule.RequestsPerMinute <= 0 {
		return true, 0
	}
	burst := rule.Burst
	if burst <= 0 {
		burst = 1
	}

	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(burst), lastRefill: time.Now()}
		rl.buckets[key] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	rate := float64(rule.RequestsPerMinute) / 60.0
	b.tokens += elapsed * rate
	if b.tokens > float64(burst) {
		b.tokens = float64(burst)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		secondsUntilNextToken := (1 - b.tokens) / rate
		return false, time.Duration(secondsUntilNextToken * float64(time.Second))
	}
	b.tokens--
	return true, 0
}
