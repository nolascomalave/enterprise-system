package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
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
	contacts    auth.ContactRepository
	sessions    auth.SessionRepository
	resetTokens auth.PasswordResetTokenRepository
	emailSender auth.EmailSender
	tx          TransactionManager
	binder      TenantContextBinder
	now         func() time.Time
}

func NewAccountService(
	manager *auth.Manager,
	credentials auth.CredentialRepository,
	contacts auth.ContactRepository,
	sessions auth.SessionRepository,
	resetTokens auth.PasswordResetTokenRepository,
	emailSender auth.EmailSender,
	tx TransactionManager,
	binder TenantContextBinder,
	now func() time.Time,
) *AccountService {
	if now == nil {
		now = time.Now
	}
	return &AccountService{
		manager: manager, credentials: credentials, contacts: contacts,
		sessions: sessions, resetTokens: resetTokens, emailSender: emailSender,
		tx: tx, binder: binder, now: now,
	}
}

func (s *AccountService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.tx == nil {
		return errors.New("account: nil transaction manager")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

func (s *AccountService) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil || in.SessionID == uuid.Nil {
		return invalidCredentialsError(auth.ErrInvalidCredentials)
	}
	if in.CurrentPassword == "" {
		return invalidCurrentPasswordError(auth.ErrInvalidCurrentPassword)
	}
	if in.NewPassword == "" {
		return weakPasswordError(auth.ErrWeakPassword)
	}
	if in.CurrentPassword == in.NewPassword {
		return samePasswordError(auth.ErrSamePassword)
	}
	ctx = s.binder.BindTenant(ctx, in.TenantID, in.PartyID, in.SessionID)

	return s.withinTx(ctx, func(ctx context.Context) error {
		credentials, err := s.credentials.GetByPartyID(ctx, in.TenantID, in.PartyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return invalidCredentialsError(auth.ErrInvalidCredentials)
			}
			return fmt.Errorf("get credentials: %w", err)
		}
		if !s.manager.VerifyPassword(credentials.PasswordHash, in.CurrentPassword) {
			return invalidCurrentPasswordError(auth.ErrInvalidCurrentPassword)
		}
		newHash, err := s.manager.HashPassword(in.NewPassword)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if err := s.resetTokens.InvalidateActive(ctx, in.TenantID, in.PartyID, now); err != nil {
			return fmt.Errorf("invalidate reset tokens: %w", err)
		}
		// Compare-and-swap on the hash we verified: if a concurrent change or
		// password reset already replaced it, this request must fail instead of
		// overwriting the winner.
		if err := s.credentials.UpdatePasswordIfCurrent(
			ctx, in.TenantID, in.PartyID, credentials.PasswordHash, newHash, now,
		); err != nil {
			if errors.Is(err, auth.ErrConcurrentModification) {
				return err
			}
			return fmt.Errorf("update password: %w", err)
		}
		if err := s.sessions.RevokeAllExcept(ctx, in.TenantID, in.PartyID, in.SessionID, now); err != nil {
			return fmt.Errorf("revoke other sessions: %w", err)
		}
		return nil
	})
}

// ForgotPassword always behaves the same for known and unknown emails to avoid
// account enumeration.
func (s *AccountService) ForgotPassword(ctx context.Context, in ForgotPasswordInput) error {
	if in.TenantID == uuid.Nil || strings.TrimSpace(in.Email) == "" {
		return invalidCredentialsError(auth.ErrInvalidCredentials)
	}
	ctx = s.binder.BindTenant(ctx, in.TenantID, uuid.Nil, uuid.Nil)
	email := normalizeEmail(in.Email)

	var (
		rawToken  string
		recipient string
	)
	err := s.withinTx(ctx, func(ctx context.Context) error {
		contact, err := s.contacts.FindContactMechanismByTypeValue(ctx, domain.ContactTypeEmail, email)
		if err != nil {
			if errors.Is(err, domain.ErrContactMechanismNotFound) {
				return nil
			}
			return fmt.Errorf("find email contact: %w", err)
		}
		relation, err := s.contacts.FindActivePartyContact(ctx, in.TenantID, contact.ID, s.now().UTC())
		if err != nil {
			if errors.Is(err, domain.ErrPartyContactNotFound) {
				return nil
			}
			return fmt.Errorf("find active party contact: %w", err)
		}
		credentials, err := s.credentials.FindByPartyContactID(ctx, in.TenantID, relation.ID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("find credentials: %w", err)
		}
		if credentials.PartyID != relation.PartyID {
			return nil
		}

		var tokenHash []byte
		rawToken, tokenHash, err = s.manager.GenerateResetToken(in.TenantID, credentials.PartyID)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		tokenID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate reset token id: %w", err)
		}
		resetToken := &auth.PasswordResetToken{
			ID: tokenID, TenantID: in.TenantID, PartyID: credentials.PartyID,
			TokenHash: tokenHash, ExpiresAt: now.Add(s.manager.ResetTTL()),
			CreatedAt: now, UpdatedAt: now,
		}
		if err := s.resetTokens.InvalidateActive(ctx, in.TenantID, credentials.PartyID, now); err != nil {
			return fmt.Errorf("invalidate previous reset tokens: %w", err)
		}
		if err := s.resetTokens.Create(ctx, resetToken); err != nil {
			return fmt.Errorf("create reset token: %w", err)
		}
		recipient = contact.Value
		return nil
	})
	if err != nil {
		return err
	}
	if rawToken == "" {
		// Unknown account: keep the same response to avoid enumeration.
		return nil
	}
	if err := s.emailSender.SendPasswordReset(ctx, recipient, rawToken); err != nil {
		return fmt.Errorf("send reset email: %w", err)
	}
	return nil
}

func (s *AccountService) ResetPassword(ctx context.Context, in ResetPasswordInput) error {
	if strings.TrimSpace(in.Token) == "" {
		return invalidResetTokenError(auth.ErrInvalidResetToken)
	}
	if in.NewPassword == "" {
		return weakPasswordError(auth.ErrWeakPassword)
	}
	tenantID, partyID, err := s.manager.ParseResetToken(in.Token)
	if err != nil {
		return invalidResetTokenError(auth.ErrInvalidResetToken)
	}
	ctx = s.binder.BindTenant(ctx, tenantID, partyID, uuid.Nil)

	now := s.now().UTC()
	return s.withinTx(ctx, func(ctx context.Context) error {
		consumed, err := s.resetTokens.Consume(ctx, tenantID, s.manager.HashResetToken(in.Token), now)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidResetToken) {
				return invalidResetTokenError(auth.ErrInvalidResetToken)
			}
			return fmt.Errorf("consume reset token: %w", err)
		}
		if consumed.PartyID != partyID {
			return invalidResetTokenError(auth.ErrInvalidResetToken)
		}

		credentials, err := s.credentials.GetByPartyID(ctx, tenantID, partyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return invalidResetTokenError(auth.ErrInvalidResetToken)
			}
			return fmt.Errorf("get credentials: %w", err)
		}
		if s.manager.VerifyPassword(credentials.PasswordHash, in.NewPassword) {
			return samePasswordError(auth.ErrSamePassword)
		}
		newHash, err := s.manager.HashPassword(in.NewPassword)
		if err != nil {
			return err
		}
		if err := s.resetTokens.InvalidateActive(ctx, tenantID, partyID, now); err != nil {
			return fmt.Errorf("invalidate reset tokens: %w", err)
		}
		// Conditional write: if the password changed between the read and this
		// point, abort so we do not silently overwrite the concurrent change.
		if err := s.credentials.UpdatePasswordIfCurrent(
			ctx, tenantID, partyID, credentials.PasswordHash, newHash, now,
		); err != nil {
			if errors.Is(err, auth.ErrConcurrentModification) {
				return err
			}
			return fmt.Errorf("update password: %w", err)
		}
		if err := s.sessions.RevokeAll(ctx, tenantID, partyID, now); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return nil
	})
}
