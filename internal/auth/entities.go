package auth

import (
	"context"
	"errors"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type Credentials struct {
	ID                      uuid.UUID
	TenantID                uuid.UUID
	PartyID                 uuid.UUID
	PartyContactMechanismID uuid.UUID
	PasswordHash            string
	TOTPSecret              string
	TOTPEnabled             bool
	BackupCodesHash         []string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	DeletedAt               *time.Time
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
	FindByPartyContactID(ctx context.Context, tenantID, partyContactMechanismID uuid.UUID) (*Credentials, error)
	GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*Credentials, error)
	// UpdatePasswordIfCurrent compares-and-swaps the password hash. It returns
	// ErrConcurrentModification when the stored hash no longer matches
	// currentHash (another change/reset won the race).
	UpdatePasswordIfCurrent(ctx context.Context, tenantID, partyID uuid.UUID, currentHash, newHash string, at time.Time) error
	// ConsumeBackupCodeIfUnchanged replaces the backup codes only if the row was
	// not modified since expectedUpdatedAt. It returns ErrConcurrentModification
	// when another request consumed a code first.
	ConsumeBackupCodeIfUnchanged(ctx context.Context, tenantID, partyID uuid.UUID, expectedUpdatedAt time.Time, remaining []string, at time.Time) error
}

// ContactRepository resolves the global email master record and its active
// tenant-scoped party association before credentials are loaded.
type ContactRepository interface {
	FindContactMechanismByTypeValue(ctx context.Context, contactType domain.ContactType, value string) (*domain.ContactMechanism, error)
	FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*domain.PartyContactMechanism, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (*Session, error)
	// RevokeIfActive revokes the session only while it is still active, not
	// expired and its token hash matches. It returns ErrSessionRevoked when no
	// row is affected (concurrent rotation/revocation or expiry).
	RevokeIfActive(ctx context.Context, tenantID, sessionID uuid.UUID, tokenHash []byte, at time.Time) error
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
