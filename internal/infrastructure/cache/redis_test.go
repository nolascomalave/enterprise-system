package cache_test

import (
	"context"
	"testing"

	"enterprise-system/internal/infrastructure/cache"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestNewRedisClientConnectsToMiniredis(t *testing.T) {
	server := miniredis.RunT(t)
	url := "redis://" + server.Addr() + "/0"

	client, err := cache.NewRedisClient(context.Background(), url)
	require.NoError(t, err)
	require.NotNil(t, client)
	_ = client.Close()
}

func TestNewRedisClientRejectsInvalidURL(t *testing.T) {
	_, err := cache.NewRedisClient(context.Background(), "not-a-redis-url")
	require.Error(t, err)
}
