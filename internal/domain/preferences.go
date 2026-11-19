package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPreferencesNotFound = errors.New("user app preferences not found")
	ErrInvalidPreferences  = errors.New("invalid user app preferences")
)

// UserPreferences is the per-user, per-client-application UI configuration.
// UserID in the repository contract is the authenticated party ID from the JWT;
// the repository resolves it to user_credentials.id.
type UserPreferences struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	UserID              uuid.UUID // authenticated party ID (JWT user_id)
	UserCredentialsID   uuid.UUID
	ClientApplicationID uuid.UUID
	Preferences         map[string]any
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

type UserPreferencesRepository interface {
	Get(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*UserPreferences, error)
	Upsert(ctx context.Context, prefs *UserPreferences) error
}
