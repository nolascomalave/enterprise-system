package cache

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const sessionBlocklistPrefix = "blocklist:session:"

// SessionBlocklist stores revoked session IDs in Redis with an in-memory
// fallback. The fallback keeps the API available when Redis is down, at the
// cost of not sharing new revocations between instances during the outage.
type SessionBlocklist struct {
	client  *redis.Client
	mu      sync.Mutex
	entries map[uuid.UUID]time.Time
	now     func() time.Time
}

func NewSessionBlocklist(client *redis.Client) *SessionBlocklist {
	return &SessionBlocklist{
		client:  client,
		entries: make(map[uuid.UUID]time.Time),
		now:     time.Now,
	}
}

func (b *SessionBlocklist) Block(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error {
	if b == nil || sessionID == uuid.Nil || ttl <= 0 {
		return nil
	}
	expiresAt := b.now().UTC().Add(ttl)

	b.mu.Lock()
	b.entries[sessionID] = expiresAt
	b.mu.Unlock()

	if b.client == nil {
		return nil
	}
	if err := b.client.Set(ctx, b.key(sessionID), "1", ttl).Err(); err != nil {
		slog.Warn("session blocklist: redis write failed; using in-memory fallback", "error", err)
		return nil
	}
	return nil
}

func (b *SessionBlocklist) IsBlocked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	if b == nil || sessionID == uuid.Nil {
		return false, nil
	}

	b.mu.Lock()
	expiresAt, ok := b.entries[sessionID]
	if ok && !b.now().UTC().Before(expiresAt) {
		delete(b.entries, sessionID)
		ok = false
	}
	b.mu.Unlock()
	if ok {
		return true, nil
	}

	if b.client == nil {
		return false, nil
	}
	exists, err := b.client.Exists(ctx, b.key(sessionID)).Result()
	if err != nil {
		slog.Warn("session blocklist: redis read failed; using in-memory fallback", "error", err)
		return false, nil
	}
	return exists > 0, nil
}

func (b *SessionBlocklist) key(sessionID uuid.UUID) string {
	return fmt.Sprintf("%s%s", sessionBlocklistPrefix, sessionID)
}
