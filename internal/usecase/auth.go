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

// TenantContextBinder isolates the usecase from the HTTP/Ent context details.
type TenantContextBinder interface {
	BindTenant(ctx context.Context, tenantID, userID, sessionID uuid.UUID) context.Context
}

type AuthService struct {
	manager     *auth.Manager
	credentials auth.CredentialRepository
	sessions    auth.SessionRepository
	binder      TenantContextBinder
	now         func() time.Time
}

func NewAuthService(
	manager *auth.Manager,
	credentials auth.CredentialRepository,
	sessions auth.SessionRepository,
	binder TenantContextBinder,
	now func() time.Time,
) *AuthService {
	if now == nil {
		now = time.Now
	}
	return &AuthService{
		manager: manager, credentials: credentials, sessions: sessions,
		binder: binder, now: now,
	}
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
	credentials, err := s.credentials.FindByEmail(ctx, in.TenantID, normalizeEmail(in.Email))
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			s.manager.DummyVerify(in.Password)
			return nil, auth.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find credentials: %w", err)
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
	credentials, err := s.credentials.GetByPartyID(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil, auth.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find credentials: %w", err)
	}
	if !credentials.TOTPEnabled || credentials.TOTPSecret == "" {
		return nil, auth.Err2FANotEnabled
	}

	if !s.manager.ValidateTOTP(credentials.TOTPSecret, in.Code) {
		remaining, ok := s.manager.VerifyBackupCode(credentials.BackupCodesHash, in.Code)
		if !ok {
			return nil, auth.ErrInvalidTOTP
		}
		credentials.BackupCodesHash = remaining
		credentials.UpdatedAt = s.now().UTC()
		if err := s.credentials.Update(ctx, credentials); err != nil {
			return nil, fmt.Errorf("update backup codes: %w", err)
		}
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

func (s *AuthService) Refresh(ctx context.Context, refreshToken string, metadata map[string]any) (*RefreshResult, error) {
	tenantID, sessionID, err := s.manager.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}

	ctx = s.binder.BindTenant(ctx, tenantID, uuid.Nil, sessionID)
	session, err := s.sessions.GetByID(ctx, tenantID, sessionID)
	if err != nil {
		return nil, auth.ErrInvalidToken
	}
	if session.RevokedAt != nil {
		return nil, auth.ErrSessionRevoked
	}
	now := s.now().UTC()
	if !session.ExpiresAt.After(now) {
		return nil, auth.ErrSessionExpired
	}
	if !s.manager.RefreshTokenMatches(refreshToken, session.TokenHash) {
		return nil, auth.ErrInvalidToken
	}

	if err := s.sessions.Revoke(ctx, tenantID, sessionID, now); err != nil {
		return nil, fmt.Errorf("revoke rotated session: %w", err)
	}

	credentials := &auth.Credentials{
		TenantID: tenantID,
		PartyID:  session.UserID,
	}
	result, err := s.issueSession(ctx, credentials, metadata)
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
	if err := s.sessions.Revoke(ctx, tenantID, sessionID, s.now().UTC()); err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
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
	if err := s.sessions.Create(ctx, session); err != nil {
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
