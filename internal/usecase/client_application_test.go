package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClientApplicationRepo struct {
	applications map[string]*domain.ClientApplication
	createErr    error
	listErr      error
	listedTenant uuid.UUID
}

func newFakeClientApplicationRepo() *fakeClientApplicationRepo {
	return &fakeClientApplicationRepo{applications: make(map[string]*domain.ClientApplication)}
}

func (r *fakeClientApplicationRepo) Create(ctx context.Context, application *domain.ClientApplication) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.applications[application.ClientID] = application
	return nil
}

func (r *fakeClientApplicationRepo) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.ClientApplication, error) {
	r.listedTenant = tenantID
	if r.listErr != nil {
		return nil, r.listErr
	}
	result := make([]*domain.ClientApplication, 0, len(r.applications))
	for _, application := range r.applications {
		if application.TenantID != nil && *application.TenantID == tenantID {
			result = append(result, application)
		}
	}
	return result, nil
}

func (r *fakeClientApplicationRepo) FindByClientID(ctx context.Context, clientID string) (*domain.ClientApplication, error) {
	application, ok := r.applications[clientID]
	if !ok {
		return nil, domain.ErrClientApplicationNotFound
	}
	return application, nil
}

func clientApplicationTestManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager, err := auth.NewManager(auth.Config{
		Secret:     []byte("01234567890123456789012345678901"),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		ClientTTL:  5 * time.Minute,
		BcryptCost: 4,
		Now:        func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	return manager
}

func TestCreateClientApplicationReturnsSecretOnce(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	manager := clientApplicationTestManager(t)
	service := NewClientApplicationService(repo, manager, newFakeTxManager(),
		func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) })
	tenantID := uuid.New()

	created, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: &tenantID, Name: "Marketing site", Scopes: []string{"parties:read", "parties:read", "locations:read"},
	})

	require.NoError(t, err)
	require.NotNil(t, created.Application)
	assert.NotEmpty(t, created.ClientSecret)
	assert.NotEqual(t, created.ClientSecret, created.Application.ClientSecretHash)
	assert.True(t, manager.VerifyClientSecret(created.Application.ClientSecretHash, created.ClientSecret))
	assert.Equal(t, []string{"parties:read", "locations:read"}, created.Application.Scopes)
	stored, ok := repo.applications[created.Application.ClientID]
	require.True(t, ok)
	require.NotNil(t, stored.TenantID)
	assert.Equal(t, tenantID, *stored.TenantID)
	assert.True(t, stored.IsActive)
}

func TestCreateClientApplicationRejectsUnknownScope(t *testing.T) {
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t),
		newFakeTxManager(), time.Now)

	_, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: ptrUUID(uuid.New()), Name: "App", Scopes: []string{"unknown:write"},
	})

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestCreateClientApplicationRejectsEmptyScopes(t *testing.T) {
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t),
		newFakeTxManager(), time.Now)

	_, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: ptrUUID(uuid.New()), Name: "App",
	})

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestIssueClientToken(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	manager := clientApplicationTestManager(t)
	service := NewClientApplicationService(repo, manager, newFakeTxManager(), time.Now)
	created, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: ptrUUID(uuid.New()), Name: "App", Scopes: []string{"parties:read", "org:read"},
	})
	require.NoError(t, err)

	token, err := service.IssueToken(context.Background(), created.Application.ClientID, created.ClientSecret)

	require.NoError(t, err)
	assert.NotEmpty(t, token.AccessToken)
	assert.Equal(t, int(manager.ClientTTL().Seconds()), token.ExpiresIn)
	assert.Equal(t, created.Application.Scopes, token.Scopes)
	claims, err := manager.ParseClientToken(token.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, created.Application.TenantID)
	assert.Equal(t, created.Application.TenantID.String(), claims.TenantID)
	assert.Equal(t, created.Application.ClientID, claims.ClientID)
	assert.Equal(t, created.Application.Scopes, claims.Scopes)
	assert.Equal(t, "THIRD_PARTY", claims.ClientType)
	assert.Equal(t, "TENANT_SPECIFIC", claims.ApplicationScope)
}

func TestIssueClientTokenRejectsInvalidSecret(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	manager := clientApplicationTestManager(t)
	service := NewClientApplicationService(repo, manager, newFakeTxManager(), time.Now)
	created, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: ptrUUID(uuid.New()), Name: "App", Scopes: []string{"parties:read"},
	})
	require.NoError(t, err)

	_, err = service.IssueToken(context.Background(), created.Application.ClientID, "wrong-secret")

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestIssueClientTokenRejectsInactiveApplication(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	manager := clientApplicationTestManager(t)
	service := NewClientApplicationService(repo, manager, newFakeTxManager(), time.Now)
	secret := "S3cret-Client-Secret-1234567890"
	hash := manager.HashClientSecret(secret)
	app := &domain.ClientApplication{
		ID: uuid.New(), TenantID: ptrUUID(uuid.New()), Name: "App", ClientID: "client",
		ClientSecretHash: hash, Scopes: []string{"parties:read"}, IsActive: false,
	}
	repo.applications[app.ClientID] = app

	_, err := service.IssueToken(context.Background(), app.ClientID, secret)

	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestCreatePlatformFirstPartyApplication(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	manager := clientApplicationTestManager(t)
	service := NewClientApplicationService(repo, manager, newFakeTxManager(), time.Now)

	created, err := service.Create(context.Background(), CreateClientApplicationInput{
		Name: "Official Flutter", Scopes: []string{"platform:admin", "reports:global"},
		ClientType: domain.ClientTypeFirstParty, ApplicationScope: domain.ApplicationScopePlatform,
	})

	require.NoError(t, err)
	assert.Nil(t, created.Application.TenantID)
	assert.Equal(t, domain.ClientTypeFirstParty, created.Application.ClientType)
	assert.Equal(t, domain.ApplicationScopePlatform, created.Application.ApplicationScope)

	token, err := service.IssueToken(context.Background(), created.Application.ClientID, created.ClientSecret)
	require.NoError(t, err)
	claims, err := manager.ParseClientToken(token.AccessToken)
	require.NoError(t, err)
	assert.Empty(t, claims.TenantID)
	assert.Equal(t, "FIRST_PARTY", claims.ClientType)
	assert.Equal(t, "PLATFORM", claims.ApplicationScope)
}

func TestCreatePlatformRejectsThirdParty(t *testing.T) {
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t),
		newFakeTxManager(), time.Now)

	_, err := service.Create(context.Background(), CreateClientApplicationInput{
		Name: "Bad platform", Scopes: []string{"platform:admin"},
		ClientType: domain.ClientTypeThirdParty, ApplicationScope: domain.ApplicationScopePlatform,
	})

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestCreateTenantSpecificRequiresTenant(t *testing.T) {
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t),
		newFakeTxManager(), time.Now)

	_, err := service.Create(context.Background(), CreateClientApplicationInput{
		Name: "No tenant", Scopes: []string{"parties:read"},
		ClientType: domain.ClientTypeThirdParty, ApplicationScope: domain.ApplicationScopeTenantSpecific,
	})

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestPlatformScopesRejectedForThirdParty(t *testing.T) {
	tenantID := uuid.New()
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t),
		newFakeTxManager(), time.Now)

	_, err := service.Create(context.Background(), CreateClientApplicationInput{
		TenantID: &tenantID, Name: "Third party", Scopes: []string{"platform:admin"},
		ClientType: domain.ClientTypeThirdParty, ApplicationScope: domain.ApplicationScopeTenantSpecific,
	})

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestListClientApplicationsFiltersByTenant(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	service := NewClientApplicationService(repo, clientApplicationTestManager(t), newFakeTxManager(), time.Now)
	tenantA, tenantB := uuid.New(), uuid.New()
	repo.applications["app-a"] = &domain.ClientApplication{
		ID: uuid.New(), TenantID: ptrUUID(tenantA), Name: "App A", ClientID: "app-a", IsActive: true,
	}
	repo.applications["app-b"] = &domain.ClientApplication{
		ID: uuid.New(), TenantID: ptrUUID(tenantB), Name: "App B", ClientID: "app-b", IsActive: true,
	}

	apps, err := service.ListClientApplications(context.Background(), tenantA)

	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Equal(t, "App A", apps[0].Name)
	assert.Equal(t, tenantA, repo.listedTenant)
}

func TestListClientApplicationsRequiresTenant(t *testing.T) {
	service := NewClientApplicationService(newFakeClientApplicationRepo(), clientApplicationTestManager(t), newFakeTxManager(), time.Now)

	_, err := service.ListClientApplications(context.Background(), uuid.Nil)

	require.ErrorIs(t, err, domain.ErrInvalidClientApplication)
}

func TestListClientApplicationsPropagatesRepositoryError(t *testing.T) {
	repo := newFakeClientApplicationRepo()
	repo.listErr = errors.New("database down")
	service := NewClientApplicationService(repo, clientApplicationTestManager(t), newFakeTxManager(), time.Now)

	_, err := service.ListClientApplications(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database down")
}
