package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResetRepo struct {
	tokens []*auth.PasswordResetToken
	err    error
}

func (r *fakeResetRepo) Create(ctx context.Context, token *auth.PasswordResetToken) error {
	if r.err != nil {
		return r.err
	}
	r.tokens = append(r.tokens, token)
	return nil
}

func (r *fakeResetRepo) InvalidateActive(ctx context.Context, tenantID, partyID uuid.UUID, at time.Time) error {
	if r.err != nil {
		return r.err
	}
	for _, token := range r.tokens {
		if token.TenantID == tenantID && token.PartyID == partyID && token.UsedAt == nil {
			t := at
			token.UsedAt = &t
		}
	}
	return nil
}

func (r *fakeResetRepo) Consume(ctx context.Context, tenantID uuid.UUID, tokenHash []byte, now time.Time) (*auth.PasswordResetToken, error) {
	if r.err != nil {
		return nil, r.err
	}
	for _, token := range r.tokens {
		if token.TenantID == tenantID && token.UsedAt == nil && token.ExpiresAt.After(now) && string(token.TokenHash) == string(tokenHash) {
			t := now
			token.UsedAt = &t
			return token, nil
		}
	}
	return nil, auth.ErrInvalidResetToken
}

type fakeEmailSender struct {
	to    string
	token string
	err   error
}

func (s *fakeEmailSender) SendPasswordReset(ctx context.Context, to, token string) error {
	s.to, s.token = to, token
	return s.err
}

func accountTestManager(t *testing.T) *auth.Manager {
	t.Helper()
	m, err := auth.NewManager(auth.Config{
		Secret:     []byte("01234567890123456789012345678901"),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		TempTTL:    5 * time.Minute,
		ResetTTL:   time.Hour,
		BcryptCost: 4,
		Now:        func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	return m
}

func accountTestCreds(t *testing.T, manager *auth.Manager) *auth.Credentials {
	t.Helper()
	hash, err := manager.HashPassword("current-password")
	require.NoError(t, err)
	return &auth.Credentials{
		ID: uuid.New(), TenantID: uuid.New(), PartyID: uuid.New(),
		Email: "user@example.com", PasswordHash: hash,
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	currentSession := uuid.New()
	otherSession := uuid.New()
	sessionRepo.sessions[currentSession] = &auth.Session{ID: currentSession, TenantID: creds.TenantID, UserID: creds.PartyID}
	sessionRepo.sessions[otherSession] = &auth.Session{ID: otherSession, TenantID: creds.TenantID, UserID: creds.PartyID}
	resetRepo := &fakeResetRepo{tokens: []*auth.PasswordResetToken{{
		TenantID: creds.TenantID, PartyID: creds.PartyID,
	}}}
	service := NewAccountService(manager, credRepo, sessionRepo, resetRepo, &fakeEmailSender{}, &fakeBinder{}, time.Now)

	err := service.ChangePassword(context.Background(), ChangePasswordInput{
		TenantID: creds.TenantID, PartyID: creds.PartyID, SessionID: currentSession,
		CurrentPassword: "current-password", NewPassword: "new-password-123",
	})

	require.NoError(t, err)
	assert.True(t, manager.VerifyPassword(credRepo.credentials.PasswordHash, "new-password-123"))
	assert.Nil(t, sessionRepo.sessions[currentSession].RevokedAt)
	assert.NotNil(t, sessionRepo.sessions[otherSession].RevokedAt)
	assert.NotNil(t, resetRepo.tokens[0].UsedAt)
}

func TestChangePasswordRejectsWrongCurrent(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	service := NewAccountService(manager, &fakeCredentialRepo{credentials: creds},
		newFakeSessionRepo(), &fakeResetRepo{}, &fakeEmailSender{}, &fakeBinder{}, time.Now)

	err := service.ChangePassword(context.Background(), ChangePasswordInput{
		TenantID: creds.TenantID, PartyID: creds.PartyID, SessionID: uuid.New(),
		CurrentPassword: "wrong", NewPassword: "new-password-123",
	})

	require.ErrorIs(t, err, auth.ErrInvalidCurrentPassword)
}

func TestForgotPasswordSendsToken(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	resetRepo := &fakeResetRepo{}
	emailSender := &fakeEmailSender{}
	service := NewAccountService(manager, &fakeCredentialRepo{credentials: creds},
		newFakeSessionRepo(), resetRepo, emailSender, &fakeBinder{}, time.Now)

	err := service.ForgotPassword(context.Background(), ForgotPasswordInput{
		TenantID: creds.TenantID, Email: "USER@example.com",
	})

	require.NoError(t, err)
	require.Len(t, resetRepo.tokens, 1)
	assert.Equal(t, creds.Email, emailSender.to)
	assert.NotEmpty(t, emailSender.token)

	tenantID, partyID, err := manager.ParseResetToken(emailSender.token)
	require.NoError(t, err)
	assert.Equal(t, creds.TenantID, tenantID)
	assert.Equal(t, creds.PartyID, partyID)
	assert.Equal(t, resetRepo.tokens[0].TokenHash, manager.HashResetToken(emailSender.token))
}

func TestForgotPasswordDoesNotRevealUnknownEmail(t *testing.T) {
	manager := accountTestManager(t)
	resetRepo := &fakeResetRepo{}
	emailSender := &fakeEmailSender{}
	service := NewAccountService(manager, &fakeCredentialRepo{}, newFakeSessionRepo(),
		resetRepo, emailSender, &fakeBinder{}, time.Now)

	err := service.ForgotPassword(context.Background(), ForgotPasswordInput{
		TenantID: uuid.New(), Email: "unknown@example.com",
	})

	require.NoError(t, err)
	assert.Empty(t, resetRepo.tokens)
	assert.Empty(t, emailSender.token)
}

func TestResetPasswordConsumesTokenAndRevokesSessions(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	resetRepo := &fakeResetRepo{}
	emailSender := &fakeEmailSender{}
	sessionRepo := newFakeSessionRepo()
	sessionRepo.sessions[uuid.New()] = &auth.Session{TenantID: creds.TenantID, UserID: creds.PartyID}
	service := NewAccountService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, resetRepo, emailSender, &fakeBinder{}, time.Now)

	require.NoError(t, service.ForgotPassword(context.Background(), ForgotPasswordInput{
		TenantID: creds.TenantID, Email: creds.Email,
	}))
	require.NotEmpty(t, emailSender.token)

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Token: emailSender.token, NewPassword: "new-password-123",
	})

	require.NoError(t, err)
	assert.True(t, manager.VerifyPassword(creds.PasswordHash, "new-password-123"))
	assert.NotNil(t, resetRepo.tokens[0].UsedAt)
	for _, session := range sessionRepo.sessions {
		assert.NotNil(t, session.RevokedAt)
	}
}

func TestResetPasswordRejectsUsedToken(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	resetRepo := &fakeResetRepo{}
	emailSender := &fakeEmailSender{}
	service := NewAccountService(manager, &fakeCredentialRepo{credentials: creds},
		newFakeSessionRepo(), resetRepo, emailSender, &fakeBinder{}, time.Now)

	require.NoError(t, service.ForgotPassword(context.Background(), ForgotPasswordInput{
		TenantID: creds.TenantID, Email: creds.Email,
	}))
	require.NoError(t, service.ResetPassword(context.Background(), ResetPasswordInput{
		Token: emailSender.token, NewPassword: "new-password-123",
	}))

	err := service.ResetPassword(context.Background(), ResetPasswordInput{
		Token: emailSender.token, NewPassword: "another-password-123",
	})
	require.ErrorIs(t, err, auth.ErrInvalidResetToken)
}

func TestForgotPasswordPropagatesEmailError(t *testing.T) {
	manager := accountTestManager(t)
	creds := accountTestCreds(t, manager)
	emailSender := &fakeEmailSender{err: errors.New("smtp down")}
	service := NewAccountService(manager, &fakeCredentialRepo{credentials: creds},
		newFakeSessionRepo(), &fakeResetRepo{}, emailSender, &fakeBinder{}, time.Now)

	err := service.ForgotPassword(context.Background(), ForgotPasswordInput{
		TenantID: creds.TenantID, Email: creds.Email,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "send reset email")
}
