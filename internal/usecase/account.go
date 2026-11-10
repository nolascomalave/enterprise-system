package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	"github.com/google/uuid"
)

type ChangePasswordInput struct {
	TenantID        uuid.UUID
	PartyID         uuid.UUID
	SessionID       uuid.UUID
	CurrentPassword string
	NewPassword     string
}

type ForgotPasswordInput struct {
	TenantID uuid.UUID
	Email    string
	Metadata map[string]any
}

type ResetPasswordInput struct {
	Token       string
	NewPassword string
}

type AccountService struct {
	manager     *auth.Manager
	credentials auth.CredentialRepository
	sessions    auth.SessionRepository
	resetTokens auth.PasswordResetTokenRepository
	emailSender auth.EmailSender
	binder      TenantContextBinder
	now         func() time.Time
}

func NewAccountService(
	manager *auth.Manager,
	credentials auth.CredentialRepository,
	sessions auth.SessionRepository,
	resetTokens auth.PasswordResetTokenRepository,
	emailSender auth.EmailSender,
	binder TenantContextBinder,
	now func() time.Time,
) *AccountService {
	if now == nil {
		now = time.Now
	}
	return &AccountService{
		manager: manager, credentials: credentials, sessions: sessions,
		resetTokens: resetTokens, emailSender: emailSender, binder: binder, now: now,
	}
}

func (s *AccountService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil || in.SessionID == uuid.Nil {
		return auth.ErrInvalidCredentials
	}
	if in.CurrentPassword == "" {
		return auth.ErrInvalidCurrentPassword
	}
	if in.NewPassword == "" {
		return auth.ErrWeakPassword
	}
	if in.CurrentPassword == in.NewPassword {
		return auth.ErrSamePassword
	}
	ctx = s.binder.BindTenant(ctx, in.TenantID, in.PartyID, in.SessionID)

	credentials, err := s.credentials.GetByPartyID(ctx, in.TenantID, in.PartyID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return auth.ErrInvalidCredentials
		}
		return fmt.Errorf("get credentials: %w", err)
	}
	if !s.manager.VerifyPassword(credentials.PasswordHash, in.CurrentPassword) {
		return auth.ErrInvalidCurrentPassword
	}
	newHash, err := s.manager.HashPassword(in.NewPassword)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.resetTokens.InvalidateActive(ctx, in.TenantID, in.PartyID, now); err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}
	credentials.PasswordHash = newHash
	credentials.UpdatedAt = now
	if err := s.credentials.Update(ctx, credentials); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.sessions.RevokeAllExcept(ctx, in.TenantID, in.PartyID, in.SessionID, now); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	return nil
}

// ForgotPassword always behaves the same for known and unknown emails to avoid
// account enumeration.
func (s *AccountService) ForgotPassword(ctx context.Context, in ForgotPasswordInput) error {
	if in.TenantID == uuid.Nil || strings.TrimSpace(in.Email) == "" {
		return auth.ErrInvalidCredentials
	}
	ctx = s.binder.BindTenant(ctx, in.TenantID, uuid.Nil, uuid.Nil)

	credentials, err := s.credentials.FindByEmail(ctx, in.TenantID, normalizeEmail(in.Email))
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("find credentials: %w", err)
	}

	rawToken, tokenHash, err := s.manager.GenerateResetToken(in.TenantID, credentials.PartyID)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.resetTokens.InvalidateActive(ctx, in.TenantID, credentials.PartyID, now); err != nil {
		return fmt.Errorf("invalidate previous reset tokens: %w", err)
	}

	tokenID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate reset token id: %w", err)
	}
	resetToken := &auth.PasswordResetToken{
		ID: tokenID, TenantID: in.TenantID, PartyID: credentials.PartyID,
		TokenHash: tokenHash, ExpiresAt: now.Add(s.manager.ResetTTL()),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.resetTokens.Create(ctx, resetToken); err != nil {
		return fmt.Errorf("create reset token: %w", err)
	}
	if err := s.emailSender.SendPasswordReset(ctx, credentials.Email, rawToken); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	return nil
}

func (s *AccountService) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	if strings.TrimSpace(in.Token) == "" {
		return auth.ErrInvalidResetToken
	}
	if in.NewPassword == "" {
		return auth.ErrWeakPassword
	}
	tenantID, partyID, err := s.manager.ParseResetToken(in.Token)
	if err != nil {
		return auth.ErrInvalidResetToken
	}
	ctx = s.binder.BindTenant(ctx, tenantID, partyID, uuid.Nil)

	now := s.now().UTC()
	consumed, err := s.resetTokens.Consume(ctx, tenantID, s.manager.HashResetToken(in.Token), now)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidResetToken) {
			return auth.ErrInvalidResetToken
		}
		return fmt.Errorf("consume reset token: %w", err)
	}
	if consumed.PartyID != partyID {
		return auth.ErrInvalidResetToken
	}

	credentials, err := s.credentials.GetByPartyID(ctx, tenantID, partyID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return auth.ErrInvalidResetToken
		}
		return fmt.Errorf("get credentials: %w", err)
	}
	if s.manager.VerifyPassword(credentials.PasswordHash, in.NewPassword) {
		return auth.ErrSamePassword
	}
	newHash, err := s.manager.HashPassword(in.NewPassword)
	if err != nil {
		return err
	}
	if err := s.resetTokens.InvalidateActive(ctx, tenantID, partyID, now); err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}
	credentials.PasswordHash = newHash
	credentials.UpdatedAt = now
	if err := s.credentials.Update(ctx, credentials); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.sessions.RevokeAll(ctx, tenantID, partyID, now); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	return nil
}
