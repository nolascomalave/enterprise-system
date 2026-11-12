package usecase

import (
	"context"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCredentialRepo struct {
	credentials       *auth.Credentials
	err               error
	updated           *auth.Credentials
	updatePasswordErr error
	consumeBackupErr  error
}

func (r *fakeCredentialRepo) FindByPartyContactID(ctx context.Context, tenantID, partyContactMechanismID uuid.UUID) (*auth.Credentials, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.credentials == nil || r.credentials.PartyContactMechanismID != partyContactMechanismID {
		return nil, auth.ErrNotFound
	}
	return r.credentials, nil
}

func (r *fakeCredentialRepo) GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*auth.Credentials, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.credentials == nil {
		return nil, auth.ErrNotFound
	}
	return r.credentials, nil
}

func (r *fakeCredentialRepo) UpdatePasswordIfCurrent(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	currentHash, newHash string,
	at time.Time,
) error {
	if r.err != nil {
		return r.err
	}
	if r.updatePasswordErr != nil {
		return r.updatePasswordErr
	}
	if r.credentials == nil || r.credentials.PasswordHash != currentHash {
		return auth.ErrConcurrentModification
	}
	r.credentials.PasswordHash = newHash
	r.credentials.UpdatedAt = at
	return nil
}

func (r *fakeCredentialRepo) ConsumeBackupCodeIfUnchanged(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	expectedUpdatedAt time.Time,
	remaining []string,
	at time.Time,
) error {
	if r.err != nil {
		return r.err
	}
	if r.consumeBackupErr != nil {
		return r.consumeBackupErr
	}
	if r.credentials == nil || !r.credentials.UpdatedAt.Equal(expectedUpdatedAt) {
		return auth.ErrConcurrentModification
	}
	updated := *r.credentials
	updated.BackupCodesHash = remaining
	updated.UpdatedAt = at
	r.credentials = &updated
	r.updated = &updated
	return nil
}

type fakeSessionRepo struct {
	sessions          map[uuid.UUID]*auth.Session
	created           []*auth.Session
	err               error
	revokeIfActiveErr error
}

func newFakeSessionRepo() *fakeSessionRepo {
	return &fakeSessionRepo{sessions: make(map[uuid.UUID]*auth.Session)}
}

func (r *fakeSessionRepo) Create(ctx context.Context, session *auth.Session) error {
	if r.err != nil {
		return r.err
	}
	r.sessions[session.ID] = session
	r.created = append(r.created, session)
	return nil
}

func (r *fakeSessionRepo) GetByID(ctx context.Context, tenantID, sessionID uuid.UUID) (*auth.Session, error) {
	if r.err != nil {
		return nil, r.err
	}
	session, ok := r.sessions[sessionID]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return session, nil
}

func (r *fakeSessionRepo) RevokeIfActive(ctx context.Context, tenantID, sessionID uuid.UUID, tokenHash []byte, at time.Time) error {
	if r.err != nil {
		return r.err
	}
	if r.revokeIfActiveErr != nil {
		return r.revokeIfActiveErr
	}
	session, ok := r.sessions[sessionID]
	if !ok || session.RevokedAt != nil || !session.ExpiresAt.After(at) || string(session.TokenHash) != string(tokenHash) {
		return auth.ErrSessionRevoked
	}
	session.RevokedAt = &at
	return nil
}

func (r *fakeSessionRepo) RevokeAll(ctx context.Context, tenantID, userID uuid.UUID, at time.Time) error {
	return r.RevokeAllExcept(ctx, tenantID, userID, uuid.Nil, at)
}

func (r *fakeSessionRepo) RevokeAllExcept(ctx context.Context, tenantID, userID, exceptSessionID uuid.UUID, at time.Time) error {
	if r.err != nil {
		return r.err
	}
	for id, session := range r.sessions {
		if id == exceptSessionID || session.UserID != userID || session.TenantID != tenantID {
			continue
		}
		session.RevokedAt = &at
	}
	return nil
}

type fakeBinder struct {
	tenantID  uuid.UUID
	userID    uuid.UUID
	sessionID uuid.UUID
	called    bool
}

func (b *fakeBinder) BindTenant(ctx context.Context, tenantID, userID, sessionID uuid.UUID) context.Context {
	b.tenantID = tenantID
	b.userID = userID
	b.sessionID = sessionID
	b.called = true
	return ctx
}

func authTestManager(t *testing.T) *auth.Manager {
	t.Helper()
	m, err := auth.NewManager(auth.Config{
		Secret:     []byte("01234567890123456789012345678901"),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		TempTTL:    5 * time.Minute,
		BcryptCost: 4,
		Now:        func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	return m
}

func authTestCredentials(t *testing.T, manager *auth.Manager) *auth.Credentials {
	t.Helper()
	hash, err := manager.HashPassword("correct horse battery staple")
	require.NoError(t, err)
	return &auth.Credentials{
		ID: uuid.New(), TenantID: uuid.New(), PartyID: uuid.New(),
		PartyContactMechanismID: uuid.New(), PasswordHash: hash,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func newAuthTestService(manager *auth.Manager, credentials *fakeCredentialRepo, sessions *fakeSessionRepo, binder *fakeBinder) *AuthService {
	contact, relation := contactForCredentials(credentials.credentials)
	return NewAuthService(manager, credentials, &fakeContactRepo{contact: contact, relation: relation},
		sessions, newFakeTxManager(), binder, func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) })
}

func TestAuthLoginSuccess(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	binder := &fakeBinder{}
	service := newAuthTestService(manager, credRepo, sessionRepo, binder)

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})

	require.NoError(t, err)
	assert.False(t, result.Requires2FA)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Len(t, sessionRepo.created, 1)
	assert.True(t, binder.called)
	assert.Equal(t, creds.TenantID, binder.tenantID)

	claims, err := manager.ParseAccessToken(result.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, creds.TenantID.String(), claims.TenantID)
	assert.Equal(t, creds.PartyID.String(), claims.UserID)
}

func TestAuthLoginInvalidPassword(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		newFakeSessionRepo(), &fakeBinder{})

	_, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "wrong",
	})

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuthLoginRejectsInactiveEmail(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	contact, relation := contactForCredentials(creds)
	end := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	relation.EndDate = &end
	service := NewAuthService(manager, credRepo, &fakeContactRepo{contact: contact, relation: relation},
		sessionRepo, newFakeTxManager(), &fakeBinder{}, func() time.Time { return end })

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	assert.Nil(t, result)
	assert.Empty(t, sessionRepo.created)
}

func TestAuthLoginRequires2FA(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	secret, _, err := manager.GenerateTOTPSecret("enterprise-system", "user@example.com")
	require.NoError(t, err)
	creds.TOTPEnabled = true
	creds.TOTPSecret = secret
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	result, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})

	require.NoError(t, err)
	assert.True(t, result.Requires2FA)
	assert.NotEmpty(t, result.TempToken)
	assert.Empty(t, result.AccessToken)
	assert.Empty(t, sessionRepo.created)
}

func TestAuthVerify2FAWithTOTP(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	secret, _, err := manager.GenerateTOTPSecret("enterprise-system", "user@example.com")
	require.NoError(t, err)
	creds.TOTPEnabled = true
	creds.TOTPSecret = secret
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	code, err := totp.GenerateCode(secret, time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	result, err := service.Verify2FA(context.Background(), Verify2FAInput{
		TempToken: login.TempToken, Code: code,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Len(t, sessionRepo.created, 1)
}

func TestAuthVerify2FAWithBackupCode(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	secret, _, err := manager.GenerateTOTPSecret("enterprise-system", "user@example.com")
	require.NoError(t, err)
	creds.TOTPEnabled = true
	creds.TOTPSecret = secret
	plain, hashes, err := manager.GenerateBackupCodes(2)
	require.NoError(t, err)
	creds.BackupCodesHash = hashes
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, credRepo, sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	result, err := service.Verify2FA(context.Background(), Verify2FAInput{
		TempToken: login.TempToken, Code: plain[0],
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	require.NotNil(t, credRepo.updated)
	assert.Len(t, credRepo.updated.BackupCodesHash, 1)
}

func TestAuthRefreshRotates(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	refreshed, err := service.Refresh(context.Background(), login.RefreshToken, nil)

	require.NoError(t, err)
	assert.NotEqual(t, login.RefreshToken, refreshed.RefreshToken)
	assert.NotEmpty(t, refreshed.AccessToken)
	assert.Len(t, sessionRepo.created, 2)
	assert.NotNil(t, sessionRepo.sessions[login.SessionID].RevokedAt)
}

func TestAuthRefreshRejectsTamperedToken(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	_, err = service.Refresh(context.Background(), login.RefreshToken+"tamper", nil)
	require.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestAuthLogoutRevokesSession(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	require.NoError(t, service.Logout(context.Background(), login.RefreshToken))
	assert.NotNil(t, sessionRepo.sessions[login.SessionID].RevokedAt)
}

func TestAuthLogoutRejectsTamperedToken(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	err = service.Logout(context.Background(), login.RefreshToken+"tamper")
	require.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Nil(t, sessionRepo.sessions[login.SessionID].RevokedAt)
}

func TestAuthLoginUsesTransactionManager(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	contact, relation := contactForCredentials(creds)
	txManager := newFakeTxManager()
	service := NewAuthService(manager, credRepo,
		&fakeContactRepo{contact: contact, relation: relation}, sessionRepo,
		txManager, &fakeBinder{}, func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) })

	_, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, txManager.calls, 2, "lookup and session creation must run in transactions")
	assert.Len(t, sessionRepo.created, 1)
}

func TestAuthRefreshRejectsConcurrentRotation(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, &fakeCredentialRepo{credentials: creds},
		sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	// Simulate another request rotating/revoking the same session between the
	// read and the conditional update.
	sessionRepo.revokeIfActiveErr = auth.ErrSessionRevoked

	_, err = service.Refresh(context.Background(), login.RefreshToken, nil)

	require.ErrorIs(t, err, auth.ErrSessionRevoked)
	assert.Len(t, sessionRepo.created, 1, "a lost rotation race must not issue a second session")
}

func TestAuthVerify2FATreatsConsumedBackupCodeAsInvalid(t *testing.T) {
	manager := authTestManager(t)
	creds := authTestCredentials(t, manager)
	secret, _, err := manager.GenerateTOTPSecret("enterprise-system", "user@example.com")
	require.NoError(t, err)
	creds.TOTPEnabled = true
	creds.TOTPSecret = secret
	plain, hashes, err := manager.GenerateBackupCodes(2)
	require.NoError(t, err)
	creds.BackupCodesHash = hashes
	credRepo := &fakeCredentialRepo{credentials: creds}
	sessionRepo := newFakeSessionRepo()
	service := newAuthTestService(manager, credRepo, sessionRepo, &fakeBinder{})

	login, err := service.Login(context.Background(), LoginInput{
		TenantID: creds.TenantID, Email: "user@example.com", Password: "correct horse battery staple",
	})
	require.NoError(t, err)

	// Another request consumed the same backup code first.
	credRepo.consumeBackupErr = auth.ErrConcurrentModification

	_, err = service.Verify2FA(context.Background(), Verify2FAInput{
		TempToken: login.TempToken, Code: plain[0],
	})

	require.ErrorIs(t, err, auth.ErrInvalidTOTP)
	assert.Empty(t, sessionRepo.created, "no session may be issued for a consumed code")
}
