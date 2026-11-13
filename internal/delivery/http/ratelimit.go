package http

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type rateEntry struct {
	count int
	reset time.Time
}

// rateLimitScript performs INCR and the first-window EXPIRE atomically, so a
// connection drop can never leave a counter without a TTL.
var rateLimitScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
`)

// RateLimiter is a fixed-window limiter. When a Redis client is configured it
// uses INCR + EXPIRE so multiple API instances share the same counters. If
// Redis is unavailable at runtime it degrades to the in-memory implementation.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
	client  *redis.Client
	prefix  string
	limit   int
	window  time.Duration
	now     func() time.Time
}

// NewRateLimiter creates an in-memory fixed-window limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

// NewRedisRateLimiter creates a Redis-backed fixed-window limiter. prefix
// isolates counters between different limiters (login, refresh, etc.).
func NewRedisRateLimiter(client *redis.Client, prefix string, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string]*rateEntry),
		client:  client,
		prefix:  prefix,
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

func (l *RateLimiter) Allow(key string) bool {
	if l == nil || l.limit <= 0 {
		return true
	}
	if l.client != nil {
		allowed, err := l.allowRedis(context.Background(), key)
		if err == nil {
			return allowed
		}
		slog.Warn("rate limiter: redis unavailable; using in-memory fallback", "error", err)
	}
	return l.allowMemory(key)
}

func (l *RateLimiter) allowRedis(ctx context.Context, key string) (bool, error) {
	redisKey := l.redisKey(key)
	ttlSeconds := int64(l.window / time.Second)
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}
	count, err := rateLimitScript.Run(ctx, l.client, []string{redisKey}, ttlSeconds).Int64()
	if err != nil {
		return false, err
	}
	return count <= int64(l.limit), nil
}

func (l *RateLimiter) allowMemory(key string) bool {
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
	if l.client != nil {
		ttl, err := l.client.TTL(context.Background(), l.redisKey(key)).Result()
		if err == nil {
			if ttl == -2 { // key does not exist
				return 0
			}
			if ttl < 0 { // key exists without expiry
				return l.window
			}
			return ttl
		}
		slog.Warn("rate limiter: redis ttl failed; using in-memory fallback", "error", err)
	}
	return l.retryAfterMemory(key)
}

func (l *RateLimiter) retryAfterMemory(key string) time.Duration {
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

func (l *RateLimiter) redisKey(key string) string {
	if l.prefix == "" {
		return "ratelimit:" + key
	}
	return "ratelimit:" + l.prefix + ":" + key
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
