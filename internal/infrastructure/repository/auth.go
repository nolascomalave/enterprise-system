package repository

import (
	"context"
	"errors"
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

func (r *CredentialRepository) FindByPartyContactID(ctx context.Context, tenantID, partyContactMechanismID uuid.UUID) (*auth.Credentials, error) {
	var result *auth.Credentials
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		row, err := client.UserCredentials.Query().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.PartyContactMechanismID(partyContactMechanismID),
				usercredentials.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
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
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		row, err := client.UserCredentials.Query().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.PartyID(partyID),
				usercredentials.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toAuthCredentials(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, auth.ErrNotFound
	}
	return result, err
}

func (r *CredentialRepository) UpdatePasswordIfCurrent(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	currentHash, newHash string,
	at time.Time,
) error {
	var affected int
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.UserCredentials.Update().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.PartyID(partyID),
				usercredentials.PasswordHashEQ(currentHash),
				usercredentials.DeletedAtIsNil(),
			).
			SetPasswordHash(newHash).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return repoError(err)
	}
	if affected == 0 {
		return auth.ErrConcurrentModification
	}
	return nil
}

func (r *CredentialRepository) ConsumeBackupCodeIfUnchanged(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	expectedUpdatedAt time.Time,
	remaining []string,
	at time.Time,
) error {
	var affected int
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.UserCredentials.Update().
			Where(
				usercredentials.TenantID(tenantID),
				usercredentials.PartyID(partyID),
				usercredentials.UpdatedAtEQ(expectedUpdatedAt),
				usercredentials.DeletedAtIsNil(),
			).
			SetBackupCodesHash(remaining).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return repoError(err)
	}
	if affected == 0 {
		return auth.ErrConcurrentModification
	}
	return nil
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
	return withAuthTx(ctx, r.client, func(client *ent.Client) error {
		_, err := client.Session.Create().
			SetID(s.ID).
			SetTenantID(s.TenantID).
			SetUserID(s.UserID).
			SetTokenHash(s.TokenHash).
			SetExpiresAt(s.ExpiresAt).
			SetMetadata(s.Metadata).
			SetCreatedAt(s.CreatedAt).
			SetUpdatedAt(s.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *SessionRepository) GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (*auth.Session, error) {
	var result *auth.Session
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		row, err := client.Session.Query().
			Where(
				session.ID(sessionID),
				session.TenantID(tenantID),
				session.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toAuthSession(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, auth.ErrNotFound
	}
	return result, err
}

func (r *SessionRepository) RevokeIfActive(
	ctx context.Context,
	tenantID, sessionID uuid.UUID,
	tokenHash []byte,
	at time.Time,
) error {
	var affected int
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.Session.Update().
			Where(
				session.ID(sessionID),
				session.TenantID(tenantID),
				session.RevokedAtIsNil(),
				session.DeletedAtIsNil(),
				session.ExpiresAtGT(at),
				session.TokenHashEQ(tokenHash),
			).
			SetRevokedAt(at).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return repoError(err)
	}
	if affected == 0 {
		return auth.ErrSessionRevoked
	}
	return nil
}

func (r *SessionRepository) RevokeAll(ctx context.Context, tenantID, userID uuid.UUID, at time.Time) error {
	return r.revokeAll(ctx, tenantID, userID, uuid.Nil, at)
}

func (r *SessionRepository) RevokeAllExcept(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error {
	return r.revokeAll(ctx, tenantID, userID, exceptSessionID, at)
}

func (r *SessionRepository) revokeAll(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error {
	return withAuthTx(ctx, r.client, func(client *ent.Client) error {
		query := client.Session.Update().
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
		return repoError(err)
	})
}

func withAuthTx(ctx context.Context, client *ent.Client, fn func(*ent.Client) error) error {
	return execute(ctx, client, fn)
}

func toAuthCredentials(e *ent.UserCredentials) *auth.Credentials {
	return &auth.Credentials{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		PartyContactMechanismID: e.PartyContactMechanismID,
		PasswordHash:            e.PasswordHash, TOTPSecret: e.TotpSecret,
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
