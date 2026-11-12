package http

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRBACUseCase struct {
	input      usecase.ResolvePermissionsInput
	universe   *domain.PermissionUniverse
	err        error
}

func (f *fakeRBACUseCase) ResolvePermissions(ctx context.Context, input usecase.ResolvePermissionsInput) (*domain.PermissionUniverse, error) {
	f.input = input
	return f.universe, f.err
}

func TestMyPermissionsRequiresAuthentication(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{}}}
	router := NewRouter(Dependencies{RBACUseCase: uc})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/rbac/my-permissions?app_client_id="+uuid.New().String(), nil, "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMyPermissionsRequiresAppClientID(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)

	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{}}}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/rbac/my-permissions", nil, token)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "invalid_app_client", body["code"])
}

func TestMyPermissionsReturnsFunctions(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)
	appClientID := uuid.New()

	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"party:read", "org:read"}}}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodGet,
		"/api/v1/rbac/my-permissions?app_client_id="+appClientID.String(), nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.input.TenantID)
	assert.Equal(t, userID, uc.input.UserCredentialsID)
	assert.Equal(t, appClientID, uc.input.AppClientID)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	functions := data["functions"].([]any)
	require.Len(t, functions, 2)
	assert.Equal(t, "party:read", functions[0])
	assert.Equal(t, "org:read", functions[1])
}

func TestMyPermissionsPropagatesUseCaseError(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)
	appClientID := uuid.New()

	uc := &fakeRBACUseCase{err: errors.New("boom")}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodGet,
		"/api/v1/rbac/my-permissions?app_client_id="+appClientID.String(), nil, token)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}
