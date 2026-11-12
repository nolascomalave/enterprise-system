package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOAuthUseCase struct {
	createInput  usecase.CreateClientApplicationInput
	created      *usecase.CreatedClientApplication
	token        *usecase.ClientToken
	clientID     string
	clientSecret string
	err          error
}

func (f *fakeOAuthUseCase) Create(ctx context.Context, in usecase.CreateClientApplicationInput) (*usecase.CreatedClientApplication, error) {
	f.createInput = in
	return f.created, f.err
}

func (f *fakeOAuthUseCase) IssueToken(ctx context.Context, clientID, clientSecret string) (*usecase.ClientToken, error) {
	f.clientID = clientID
	f.clientSecret = clientSecret
	return f.token, f.err
}

func TestCreateClientApplicationHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	applicationTenant := tenantID
	application := &domain.ClientApplication{
		ID: uuid.New(), TenantID: &applicationTenant, Name: "Website", ClientID: "client-123",
		Scopes: []string{"parties:read"}, ClientType: domain.ClientTypeThirdParty,
		ApplicationScope: domain.ApplicationScopeTenantSpecific,
		IsActive:         true, CreatedAt: now, UpdatedAt: now,
	}
	uc := &fakeOAuthUseCase{created: &usecase.CreatedClientApplication{
		Application: application, ClientSecret: "plain-secret-once",
	}}
	router := NewRouter(Dependencies{OAuthUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/client-applications",
		[]byte(`{"name":"Website","scopes":["parties:read"]}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, uc.createInput.TenantID)
	assert.Equal(t, tenantID, *uc.createInput.TenantID)
	assert.Equal(t, "Website", uc.createInput.Name)
	assert.Equal(t, []string{"parties:read"}, uc.createInput.Scopes)
	body := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, "client-123", body["client_id"])
	assert.Equal(t, "plain-secret-once", body["client_secret"])
}

func TestOAuthTokenHandler(t *testing.T) {
	uc := &fakeOAuthUseCase{token: &usecase.ClientToken{
		AccessToken: "jwt-token", ExpiresIn: 300, Scopes: []string{"parties:read", "org:read"},
	}}
	router := NewRouter(Dependencies{OAuthUseCase: uc})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/oauth/token",
		[]byte(`{"client_id":"client-123","client_secret":"secret"}`), "")

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "client-123", uc.clientID)
	assert.Equal(t, "secret", uc.clientSecret)
	body := decodeBody(t, recorder)
	assert.Equal(t, "jwt-token", body["access_token"])
	assert.Equal(t, "Bearer", body["token_type"])
	assert.Equal(t, "parties:read org:read", body["scope"])
}

func TestCreateFirstPartyApplicationRequiresPlatformKey(t *testing.T) {
	uc := &fakeOAuthUseCase{}
	router := NewRouter(Dependencies{OAuthUseCase: uc, PlatformAdminKey: "test-key"})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/client-applications/first-party",
		strings.NewReader(`{"name":"Official","scopes":["platform:admin"],"application_scope":"PLATFORM"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestCreateFirstPartyApplicationWithPlatformKey(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	application := &domain.ClientApplication{
		ID: uuid.New(), Name: "Official", ClientID: "client-first",
		Scopes: []string{"platform:admin"}, ClientType: domain.ClientTypeFirstParty,
		ApplicationScope: domain.ApplicationScopePlatform, IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	uc := &fakeOAuthUseCase{created: &usecase.CreatedClientApplication{
		Application: application, ClientSecret: "secret-once",
	}}
	router := NewRouter(Dependencies{OAuthUseCase: uc, PlatformAdminKey: "test-key"})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/client-applications/first-party",
		strings.NewReader(`{"name":"Official","scopes":["platform:admin"],"application_scope":"PLATFORM"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-Admin-Key", "test-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, domain.ClientTypeFirstParty, uc.createInput.ClientType)
	assert.Equal(t, domain.ApplicationScopePlatform, uc.createInput.ApplicationScope)
	assert.Nil(t, uc.createInput.TenantID)
}
