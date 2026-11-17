package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newBlocklistTestClient(t *testing.T, addr string) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestSessionBlocklistIsSharedAcrossInstances(t *testing.T) {
	server := miniredis.RunT(t)
	client := newBlocklistTestClient(t, server.Addr())
	ttl := time.Minute
	sessionID := uuid.New()

	instanceA := NewSessionBlocklist(client)
	instanceB := NewSessionBlocklist(client)

	require.NoError(t, instanceA.Block(context.Background(), sessionID, ttl))
	blocked, err := instanceB.IsBlocked(context.Background(), sessionID)
	require.NoError(t, err)
	require.True(t, blocked)

	server.FastForward(ttl + time.Second)
	blocked, err = instanceB.IsBlocked(context.Background(), sessionID)
	require.NoError(t, err)
	require.False(t, blocked)
}

func TestSessionBlocklistFallsBackToMemoryWhenRedisFails(t *testing.T) {
	server := miniredis.RunT(t)
	client := newBlocklistTestClient(t, server.Addr())
	blocklist := NewSessionBlocklist(client)
	sessionID := uuid.New()

	server.Close()

	require.NoError(t, blocklist.Block(context.Background(), sessionID, time.Minute))
	blocked, err := blocklist.IsBlocked(context.Background(), sessionID)
	require.NoError(t, err)
	require.True(t, blocked)
}

func TestSessionBlocklistWorksWithoutRedis(t *testing.T) {
	blocklist := NewSessionBlocklist(nil)
	sessionID := uuid.New()

	require.NoError(t, blocklist.Block(context.Background(), sessionID, time.Minute))
	blocked, err := blocklist.IsBlocked(context.Background(), sessionID)
	require.NoError(t, err)
	require.True(t, blocked)
}
