package http

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T, addr string) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		MaxRetries: -1,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedisRateLimiterBlocksAfterFiveAttempts(t *testing.T) {
	server := miniredis.RunT(t)
	client := newTestRedisClient(t, server.Addr())
	limiter := NewRedisRateLimiter(client, "login", 5, time.Minute)
	key := "192.0.2.1:/api/v1/auth/login"

	for i := 0; i < 5; i++ {
		require.True(t, limiter.Allow(key), "attempt %d should be allowed", i+1)
	}
	assert.False(t, limiter.Allow(key), "sixth attempt must be blocked")
	assert.Greater(t, limiter.RetryAfter(key), time.Duration(0))
}

func TestRedisRateLimiterIsSharedAcrossInstances(t *testing.T) {
	server := miniredis.RunT(t)
	client := newTestRedisClient(t, server.Addr())

	instanceA := NewRedisRateLimiter(client, "login", 5, time.Minute)
	instanceB := NewRedisRateLimiter(client, "login", 5, time.Minute)
	key := "192.0.2.1:/api/v1/auth/login"

	for i := 0; i < 5; i++ {
		require.True(t, instanceA.Allow(key))
	}
	assert.False(t, instanceB.Allow(key), "the second API instance must see the shared counter")
}

func TestRedisRateLimiterWindowResets(t *testing.T) {
	server := miniredis.RunT(t)
	client := newTestRedisClient(t, server.Addr())
	limiter := NewRedisRateLimiter(client, "login", 2, time.Minute)
	key := "192.0.2.1:/api/v1/auth/login"

	require.True(t, limiter.Allow(key))
	require.True(t, limiter.Allow(key))
	require.False(t, limiter.Allow(key))

	server.FastForward(2 * time.Minute)
	assert.True(t, limiter.Allow(key), "the counter must reset after the window expires")
}

func TestRedisRateLimiterFallsBackToMemoryWhenRedisFails(t *testing.T) {
	server := miniredis.RunT(t)
	client := newTestRedisClient(t, server.Addr())
	limiter := NewRedisRateLimiter(client, "login", 5, time.Minute)
	key := "192.0.2.1:/api/v1/auth/login"

	server.Close()

	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(key), "memory fallback attempt %d should be allowed", i+1)
	}
	assert.False(t, limiter.Allow(key), "memory fallback must still enforce the limit")
}

func TestRedisRateLimiterSetsTTLAtomicallyOnFirstRequest(t *testing.T) {
	server := miniredis.RunT(t)
	client := newTestRedisClient(t, server.Addr())
	limiter := NewRedisRateLimiter(client, "login", 5, time.Minute)
	key := "192.0.2.1:/api/v1/auth/login"
	redisKey := "ratelimit:login:" + key

	require.True(t, limiter.Allow(key))
	ttl := server.TTL(redisKey)
	require.Greater(t, ttl, time.Duration(0), "the first request must set a TTL atomically")

	// The second request must not reset the window TTL.
	server.FastForward(30 * time.Second)
	require.True(t, limiter.Allow(key))
	ttl = server.TTL(redisKey)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, 30*time.Second, "the TTL must not be extended by later requests")
}
