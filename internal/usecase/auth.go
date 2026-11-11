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

// TenantContextBinder isolates the usecase from the HTTP/Ent context details.
type TenantContextBinder interface {
	BindTenant(ctx context.Context, tenantID, userID, sessionID uuid.UUID) context.Context
}

type AuthService struct {
	manager     *auth.Manager
	credentials auth.CredentialRepository
	contacts    auth.ContactRepository
	sessions    auth.SessionRepository
	tx          TransactionManager
	binder      TenantContextBinder
	now         func() time.Time
}

func NewAuthService(
	manager *auth.Manager,
	credentials auth.CredentialRepository,
	contacts auth.ContactRepository,
	sessions auth.SessionRepository,
	tx TransactionManager,
	binder TenantContextBinder,
	now func() time.Time,
) *AuthService {
	if now == nil {
		now = time.Now
	}
	return &AuthService{
		manager: manager, credentials: credentials, contacts: contacts,
		sessions: sessions, tx: tx, binder: binder, now: now,
	}
}

func (s *AuthService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.tx == nil {
		return errors.New("auth: nil transaction manager")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

type LoginInput struct {
	TenantID uuid.UUID
	Email    string
	Password string
	Metadata map[string]any
}

type LoginResult struct {
	Requires2FA  bool
	TempToken    string
	AccessToken  string
	RefreshToken string
	SessionID    uuid.UUID
	ExpiresIn    int
}

type Verify2FAInput struct {
	TempToken string
	Code      string
	Metadata  map[string]any
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	SessionID    uuid.UUID
	ExpiresIn    int
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if in.TenantID == uuid.Nil || strings.TrimSpace(in.Email) == "" || in.Password == "" {
		return nil, auth.ErrInvalidCredentials
	}

	ctx = s.binder.BindTenant(ctx, in.TenantID, uuid.Nil, uuid.Nil)
	email := normalizeEmail(in.Email)

	var (
		contact     *domain.ContactMechanism
		relation    *domain.PartyContactMechanism
		credentials *auth.Credentials
	)
	err := s.withinTx(ctx, func(ctx context.Context) error {
		var err error
		contact, err = s.contacts.FindContactMechanismByTypeValue(ctx, domain.ContactTypeEmail, email)
		if err != nil {
			return err
		}
		relation, err = s.contacts.FindActivePartyContact(ctx, in.TenantID, contact.ID, s.now().UTC())
		if err != nil {
			return err
		}
		credentials, err = s.credentials.FindByPartyContactID(ctx, in.TenantID, relation.ID)
		return err
	})
	if err != nil {
		return nil, s.loginLookupError(err, in.Password)
	}
	if credentials.PartyID != relation.PartyID {
		s.manager.DummyVerify(in.Password)
		return nil, auth.ErrInvalidCredentials
	}
	if !s.manager.VerifyPassword(credentials.PasswordHash, in.Password) {
		return nil, auth.ErrInvalidCredentials
	}

	if credentials.TOTPEnabled {
		if credentials.TOTPSecret == "" {
			return nil, auth.Err2FANotEnabled
		}
		tempToken, err := s.manager.IssueTempToken(in.TenantID, credentials.PartyID)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Requires2FA: true, TempToken: tempToken}, nil
	}

	result, err := s.issueSession(ctx, credentials, in.Metadata)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		SessionID: result.SessionID, ExpiresIn: result.ExpiresIn,
	}, nil
}

func (s *AuthService) loginLookupError(err error, password string) error {
	switch {
	case errors.Is(err, domain.ErrContactMechanismNotFound),
		errors.Is(err, domain.ErrPartyContactNotFound),
		errors.Is(err, auth.ErrNotFound):
		s.manager.DummyVerify(password)
		return auth.ErrInvalidCredentials
	default:
		return fmt.Errorf("resolve login contact: %w", err)
	}
}
func (s *AuthService) Verify2FA(ctx context.Context, in Verify2FAInput) (*LoginResult, error) {
	claims, err := s.manager.ParseTempToken(in.TempToken)
	if err != nil {
		return nil, err
	}
	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	ctx = s.binder.BindTenant(ctx, tenantID, userID, uuid.Nil)

	var result *LoginResult
	err = s.withinTx(ctx, func(ctx context.Context) error {
		credentials, err := s.credentials.GetByPartyID(ctx, tenantID, userID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return auth.ErrInvalidCredentials
			}
			return fmt.Errorf("find credentials: %w", err)
		}
		if !credentials.TOTPEnabled || credentials.TOTPSecret == "" {
			return auth.Err2FANotEnabled
		}

		if !s.manager.ValidateTOTP(credentials.TOTPSecret, in.Code) {
			remaining, ok := s.manager.VerifyBackupCode(credentials.BackupCodesHash, in.Code)
			if !ok {
				return auth.ErrInvalidTOTP
			}
			// Optimistic update: the code is only consumed if nobody else
			// modified the credentials since they were read. A concurrent
			// consumer wins and this request treats the code as already used.
			if err := s.credentials.ConsumeBackupCodeIfUnchanged(
				ctx, tenantID, userID, credentials.UpdatedAt, remaining, s.now().UTC(),
			); err != nil {
				if errors.Is(err, auth.ErrConcurrentModification) {
					return auth.ErrInvalidTOTP
				}
				return fmt.Errorf("consume backup code: %w", err)
			}
		}

		var issueErr error
		result, issueErr = s.issueSession(ctx, credentials, in.Metadata)
		return issueErr
	})
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		SessionID: result.SessionID, ExpiresIn: result.ExpiresIn,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string, metadata map[string]any) (*RefreshResult, error) {
	tenantID, sessionID, err := s.manager.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	ctx = s.binder.BindTenant(ctx, tenantID, uuid.Nil, sessionID)

	var result *LoginResult
	err = s.withinTx(ctx, func(ctx context.Context) error {
		session, err := s.sessions.GetByID(ctx, tenantID, sessionID)
		if err != nil {
			return auth.ErrInvalidToken
		}
		if session.RevokedAt != nil {
			return auth.ErrSessionRevoked
		}
		now := s.now().UTC()
		if !session.ExpiresAt.After(now) {
			return auth.ErrSessionExpired
		}
		if !s.manager.RefreshTokenMatches(refreshToken, session.TokenHash) {
			return auth.ErrInvalidToken
		}

		credentials := &auth.Credentials{
			TenantID: tenantID,
			PartyID:  session.UserID,
		}
		// Conditional revocation: only the request that still owns the active
		// token can rotate it. Losing the race means the token was already
		// rotated/revoked, so it must not issue a second session.
		if err := s.sessions.RevokeIfActive(ctx, tenantID, sessionID, session.TokenHash, now); err != nil {
			if errors.Is(err, auth.ErrSessionRevoked) {
				return err
			}
			return fmt.Errorf("revoke rotated session: %w", err)
		}
		var issueErr error
		result, issueErr = s.issueSession(ctx, credentials, metadata)
		return issueErr
	})
	if err != nil {
		return nil, err
	}
	return &RefreshResult{
		AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		SessionID: result.SessionID, ExpiresIn: result.ExpiresIn,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	tenantID, sessionID, err := s.manager.ParseRefreshToken(refreshToken)
	if err != nil {
		return auth.ErrInvalidToken
	}
	ctx = s.binder.BindTenant(ctx, tenantID, uuid.Nil, sessionID)
	return s.withinTx(ctx, func(ctx context.Context) error {
		session, err := s.sessions.GetByID(ctx, tenantID, sessionID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("load session for logout: %w", err)
		}
		if !s.manager.RefreshTokenMatches(refreshToken, session.TokenHash) {
			return auth.ErrInvalidToken
		}
		if session.RevokedAt != nil {
			return nil
		}
		if err := s.sessions.RevokeIfActive(ctx, tenantID, sessionID, session.TokenHash, s.now().UTC()); err != nil {
			if errors.Is(err, auth.ErrSessionRevoked) || errors.Is(err, auth.ErrNotFound) {
				// Already revoked by a concurrent request: logout is idempotent.
				return nil
			}
			return fmt.Errorf("revoke session: %w", err)
		}
		return nil
	})
}

func (s *AuthService) issueSession(ctx context.Context, credentials *auth.Credentials, metadata map[string]any) (*LoginResult, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	refreshToken, tokenHash, err := s.manager.GenerateRefreshToken(credentials.TenantID, sessionID)
	if err != nil {
		return nil, err
	}

	now := s.now().UTC()
	session := &auth.Session{
		ID: sessionID, TenantID: credentials.TenantID, UserID: credentials.PartyID,
		TokenHash: tokenHash, ExpiresAt: now.Add(s.manager.RefreshTTL()),
		Metadata: metadata, CreatedAt: now, UpdatedAt: now,
	}
	ctx = s.binder.BindTenant(ctx, credentials.TenantID, credentials.PartyID, sessionID)
	if err := s.withinTx(ctx, func(ctx context.Context) error {
		return s.sessions.Create(ctx, session)
	}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	accessToken, err := s.manager.IssueAccessToken(credentials.TenantID, credentials.PartyID, sessionID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken: accessToken, RefreshToken: refreshToken,
		SessionID: sessionID, ExpiresIn: int(s.manager.AccessTTL().Seconds()),
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
