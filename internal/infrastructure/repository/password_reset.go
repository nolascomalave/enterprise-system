package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/passwordresettoken"
	"enterprise-system/internal/auth"
	"github.com/google/uuid"
)

var _ auth.PasswordResetTokenRepository = (*PasswordResetTokenRepository)(nil)

type PasswordResetTokenRepository struct {
	client *ent.Client
}

func NewPasswordResetTokenRepository(client *ent.Client) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{client: client}
}

func (r *PasswordResetTokenRepository) Create(ctx context.Context, token *auth.PasswordResetToken) error {
	if token == nil {
		return errors.New("repository: nil password reset token")
	}
	return withAuthTx(ctx, r.client, func(client *ent.Client) error {
		_, err := client.PasswordResetToken.Create().
			SetID(token.ID).
			SetTenantID(token.TenantID).
			SetPartyID(token.PartyID).
			SetTokenHash(token.TokenHash).
			SetExpiresAt(token.ExpiresAt).
			SetCreatedAt(token.CreatedAt).
			SetUpdatedAt(token.UpdatedAt).
			Save(ctx)
		return err
	})
}

func (r *PasswordResetTokenRepository) InvalidateActive(ctx context.Context, tenantID, partyID uuid.UUID, at time.Time) error {
	return withAuthTx(ctx, r.client, func(client *ent.Client) error {
		_, err := client.PasswordResetToken.Update().
			Where(
				passwordresettoken.TenantID(tenantID),
				passwordresettoken.PartyID(partyID),
				passwordresettoken.UsedAtIsNil(),
				passwordresettoken.ExpiresAtGT(at),
			).
			SetUsedAt(at).
			SetUpdatedAt(at).
			Save(ctx)
		return err
	})
}

func (r *PasswordResetTokenRepository) Consume(ctx context.Context, tenantID uuid.UUID, tokenHash []byte, now time.Time) (*auth.PasswordResetToken, error) {
	var result *auth.PasswordResetToken
	err := withAuthTx(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PasswordResetToken.Query().
			Where(
				passwordresettoken.TenantID(tenantID),
				passwordresettoken.TokenHash(tokenHash),
				passwordresettoken.UsedAtIsNil(),
				passwordresettoken.ExpiresAtGT(now),
			).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return auth.ErrInvalidResetToken
			}
			return err
		}

		affected, err := client.PasswordResetToken.Update().
			Where(
				passwordresettoken.ID(row.ID),
				passwordresettoken.UsedAtIsNil(),
				passwordresettoken.ExpiresAtGT(now),
			).
			SetUsedAt(now).
			SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return err
		}
		if affected == 0 {
			return auth.ErrInvalidResetToken
		}
		result = toAuthPasswordResetToken(row)
		result.UsedAt = &now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func toAuthPasswordResetToken(e *ent.PasswordResetToken) *auth.PasswordResetToken {
	return &auth.PasswordResetToken{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, TokenHash: e.TokenHash,
		ExpiresAt: e.ExpiresAt, UsedAt: e.UsedAt, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
