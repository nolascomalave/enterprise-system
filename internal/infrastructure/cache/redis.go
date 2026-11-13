package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const DefaultRedisURL = "redis://localhost:6379/0"

// NewRedisClient parses redisURL, connects and verifies the connection with PING.
// An empty URL falls back to DefaultRedisURL.
func NewRedisClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		redisURL = DefaultRedisURL
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(options)
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}
