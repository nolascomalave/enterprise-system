package http

import (
	"net/http"
	"testing"

	"enterprise-system/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientCredentialsScopesRestrictRoutes(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	token, err := manager.IssueClientToken(&tenantID, "client-123", "THIRD_PARTY", "TENANT_SPECIFIC", []string{auth.ScopePartiesRead})
	require.NoError(t, err)

	router := NewRouter(Dependencies{
		PartyUseCase: &fakePartyUseCase{},
		TokenManager: manager,
	})

	// Allowed: read scope.
	read := requestWithToken(router, http.MethodGet, "/api/v1/parties", nil, token)
	assert.Equal(t, http.StatusOK, read.Code)

	// Denied: write scope required.
	write := requestWithToken(router, http.MethodPost, "/api/v1/parties", []byte(`{}`), token)
	assert.Equal(t, http.StatusForbidden, write.Code)
	body := decodeBody(t, write)
	assert.Equal(t, "forbidden", body["code"])

	// Denied: route is not mapped for client credentials.
	unmapped := requestWithToken(router, http.MethodPost, "/api/v1/auth/change-password",
		[]byte(`{}`), token)
	assert.Equal(t, http.StatusForbidden, unmapped.Code)
}

func TestClientCredentialsTenantIsInjected(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	token, err := manager.IssueClientToken(&tenantID, "client-123", "THIRD_PARTY", "TENANT_SPECIFIC", []string{auth.ScopePartiesRead})
	require.NoError(t, err)

	partyUseCase := &fakePartyUseCase{}
	router := NewRouter(Dependencies{PartyUseCase: partyUseCase, TokenManager: manager})
	recorder := requestWithToken(router, http.MethodGet, "/api/v1/parties", nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, partyUseCase.gotTenant)
}

func TestFirstPartyPlatformTokenAccessesPlatformRoute(t *testing.T) {
	manager := testAuthManager(t)
	token, err := manager.IssueClientToken(nil, "first-party-app", "FIRST_PARTY", "PLATFORM",
		[]string{auth.ScopePlatformAdmin})
	require.NoError(t, err)

	router := NewRouter(Dependencies{TokenManager: manager})
	recorder := requestWithToken(router, http.MethodGet, "/api/v1/platform/status", nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestPlatformTokenWithoutScopeIsForbidden(t *testing.T) {
	manager := testAuthManager(t)
	token, err := manager.IssueClientToken(nil, "first-party-app", "FIRST_PARTY", "PLATFORM",
		[]string{"reports:global"})
	require.NoError(t, err)

	router := NewRouter(Dependencies{TokenManager: manager})
	recorder := requestWithToken(router, http.MethodGet, "/api/v1/platform/status", nil, token)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}
