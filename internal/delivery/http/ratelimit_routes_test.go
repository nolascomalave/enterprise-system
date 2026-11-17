package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFirstPartyApplicationRateLimit(t *testing.T) {
	now := time.Now()
	application := &domain.ClientApplication{
		ID: uuid.New(), Name: "Official", ClientID: "official-client",
		Scopes: []string{"platform:admin"}, ClientType: domain.ClientTypeFirstParty,
		ApplicationScope: domain.ApplicationScopePlatform, IsActive: true,
		CreatedAt: now, UpdatedAt: now,
	}
	uc := &fakeOAuthUseCase{created: &usecase.CreatedClientApplication{
		Application: application, ClientSecret: "secret-once",
	}}
	key := strings.Repeat("a", 32)
	router := NewRouter(Dependencies{OAuthUseCase: uc, PlatformAdminKey: key})
	body := `{"name":"Official","scopes":["platform:admin"],"application_scope":"PLATFORM"}`

	for i := 0; i < 5; i++ {
		request := httptest.NewRequest(http.MethodPost,
			"/api/v1/client-applications/first-party", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Platform-Admin-Key", key)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusCreated, recorder.Code, "request %d", i+1)
	}

	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/client-applications/first-party", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-Admin-Key", key)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
}

func TestOAuthTokenRateLimit(t *testing.T) {
	uc := &fakeOAuthUseCase{token: &usecase.ClientToken{
		AccessToken: "jwt-token", ExpiresIn: 300, Scopes: []string{"parties:read"},
	}}
	router := NewRouter(Dependencies{OAuthUseCase: uc})
	body := []byte(`{"client_id":"client","client_secret":"secret"}`)

	for i := 0; i < 5; i++ {
		recorder := requestWithToken(router, http.MethodPost, "/api/v1/oauth/token", body, "")
		require.Equal(t, http.StatusOK, recorder.Code, "request %d", i+1)
	}

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/oauth/token", body, "")
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
}
