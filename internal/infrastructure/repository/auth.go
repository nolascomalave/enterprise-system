package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/session"
	"enterprise-system/ent/usercredentials"
	"enterprise-system/internal/auth"
	"github.com/google/uuid"
)

var (
	_ auth.CredentialRepository = (*CredentialRepository)(nil)
	_ auth.SessionRepository    = (*SessionRepository)(nil)
)

type CredentialRepository struct {
	client *ent.Client
}

func NewCredentialRepository(client *ent.Client) *CredentialRepository {
	return &CredentialRepository{client: client}
}

func (r *CredentialRepository) FindByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*auth.Credentials, error) {
	var result *auth.Credentials
	err := withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		row, err := tx.UserCredentials.Query().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.Email(email),
				usercredentials.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toAuthCredentials(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, auth.ErrNotFound
	}
	return result, err
}

func (r *CredentialRepository) GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*auth.Credentials, error) {
	var result *auth.Credentials
	err := withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		row, err := tx.UserCredentials.Query().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.PartyID(partyID),
				usercredentials.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toAuthCredentials(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, auth.ErrNotFound
	}
	return result, err
}

func (r *CredentialRepository) Update(ctx context.Context, credentials *auth.Credentials) error {
	if credentials == nil {
		return errors.New("repository: nil credentials")
	}
	return withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		update := tx.UserCredentials.UpdateOneID(credentials.ID).
			SetEmail(credentials.Email).
			SetPasswordHash(credentials.PasswordHash).
			SetTotpEnabled(credentials.TOTPEnabled).
			SetBackupCodesHash(credentials.BackupCodesHash).
			SetUpdatedAt(credentials.UpdatedAt)
		if credentials.TOTPSecret != "" {
			update.SetTotpSecret(credentials.TOTPSecret)
		} else {
			update.ClearTotpSecret()
		}
		if credentials.DeletedAt != nil {
			update.SetDeletedAt(*credentials.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return err
	})
}

type SessionRepository struct {
	client *ent.Client
}

func NewSessionRepository(client *ent.Client) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) Create(ctx context.Context, s *auth.Session) error {
	if s == nil {
		return errors.New("repository: nil session")
	}
	return withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		_, err := tx.Session.Create().
			SetID(s.ID).
			SetTenantID(s.TenantID).
			SetUserID(s.UserID).
			SetTokenHash(s.TokenHash).
			SetExpiresAt(s.ExpiresAt).
			SetMetadata(s.Metadata).
			SetCreatedAt(s.CreatedAt).
			SetUpdatedAt(s.UpdatedAt).
			Save(ctx)
		return err
	})
}

func (r *SessionRepository) GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (*auth.Session, error) {
	var result *auth.Session
	err := withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		row, err := tx.Session.Query().
			Where(
				session.ID(sessionID),
				session.TenantID(tenantID),
				session.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toAuthSession(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, auth.ErrNotFound
	}
	return result, err
}

func (r *SessionRepository) Revoke(ctx context.Context, tenantID, sessionID uuid.UUID, at time.Time) error {
	err := withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		_, err := tx.Session.UpdateOneID(sessionID).
			Where(session.TenantID(tenantID)).
			SetRevokedAt(at).
			SetUpdatedAt(at).
			Save(ctx)
		return err
	})
	if ent.IsNotFound(err) {
		return auth.ErrNotFound
	}
	return err
}

func (r *SessionRepository) RevokeAll(ctx context.Context, tenantID, userID uuid.UUID, at time.Time) error {
	return r.revokeAll(ctx, tenantID, userID, uuid.Nil, at)
}

func (r *SessionRepository) RevokeAllExcept(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error {
	return r.revokeAll(ctx, tenantID, userID, exceptSessionID, at)
}

func (r *SessionRepository) revokeAll(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error {
	return withAuthTx(ctx, r.client, func(tx *ent.Tx) error {
		query := tx.Session.Update().
			Where(
				session.TenantID(tenantID),
				session.UserID(userID),
				session.RevokedAtIsNil(),
				session.DeletedAtIsNil(),
			)
		if exceptSessionID != uuid.Nil {
			query = query.Where(session.IDNEQ(exceptSessionID))
		}
		_, err := query.SetRevokedAt(at).SetUpdatedAt(at).Save(ctx)
		return err
	})
}

func withAuthTx(ctx context.Context, client *ent.Client, fn func(*ent.Tx) error) error {
	if client == nil {
		return errors.New("repository: nil ent client")
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit transaction: %w", err)
	}
	return nil
}

func toAuthCredentials(e *ent.UserCredentials) *auth.Credentials {
	return &auth.Credentials{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, Email: e.Email,
		PasswordHash: e.PasswordHash, TOTPSecret: e.TotpSecret,
		TOTPEnabled: e.TotpEnabled, BackupCodesHash: e.BackupCodesHash,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toAuthSession(e *ent.Session) *auth.Session {
	return &auth.Session{
		ID: e.ID, TenantID: e.TenantID, UserID: e.UserID, TokenHash: e.TokenHash,
		ExpiresAt: e.ExpiresAt, RevokedAt: e.RevokedAt, LastUsedAt: e.LastUsedAt,
		Metadata: e.Metadata, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}
