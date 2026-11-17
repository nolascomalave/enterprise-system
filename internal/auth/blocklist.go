package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionBlocklist records revoked sessions until their access tokens expire.
type SessionBlocklist interface {
	Block(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error
	IsBlocked(ctx context.Context, sessionID uuid.UUID) (bool, error)
}
