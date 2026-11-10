package http

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateEntry struct {
	count int
	reset time.Time
}

// RateLimiter is an in-memory fixed-window limiter. For a multi-instance
// deployment, replace it with a Redis-backed limiter.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
	limit   int
	window  time.Duration
	now     func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

func (l *RateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || !now.Before(entry.reset) {
		l.entries[key] = &rateEntry{count: 1, reset: now.Add(l.window)}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}

func (l *RateLimiter) RetryAfter(key string) time.Duration {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok {
		return 0
	}
	d := entry.reset.Sub(l.now())
	if d < 0 {
		return 0
	}
	return d
}

func RateLimit(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.Request.URL.Path
		if !limiter.Allow(key) {
			retry := limiter.RetryAfter(key)
			c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, APIError{
				Error: "too many requests",
				Code:  "rate_limited",
			})
			return
		}
		c.Next()
	}
}
