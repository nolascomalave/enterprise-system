package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Credentials struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PartyID         uuid.UUID
	Email           string
	PasswordHash    string
	TOTPSecret      string
	TOTPEnabled     bool
	BackupCodesHash []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type Session struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	UserID     uuid.UUID
	TokenHash  []byte
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

type CredentialRepository interface {
	FindByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*Credentials, error)
	GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*Credentials, error)
	Update(ctx context.Context, credentials *Credentials) error
}

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (*Session, error)
	Revoke(ctx context.Context, tenantID, sessionID uuid.UUID, at time.Time) error
	RevokeAll(ctx context.Context, tenantID, userID uuid.UUID, at time.Time) error
	RevokeAllExcept(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error
}

type PasswordResetToken struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	PartyID   uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *PasswordResetToken) error
	InvalidateActive(ctx context.Context, tenantID, partyID uuid.UUID, at time.Time) error
	Consume(ctx context.Context, tenantID uuid.UUID, tokenHash []byte, now time.Time) (*PasswordResetToken, error)
}

type EmailSender interface {
	SendPasswordReset(ctx context.Context, to, token string) error
}
