package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requestWithTokenAndApp(router *gin.Engine, method, path string, body []byte, token, appClientID string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if appClientID != "" {
		req.Header.Set("X-App-Client-ID", appClientID)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

type fakeRBACUseCase struct {
	input     usecase.ResolvePermissionsInput
	menuInput usecase.GetUserMenuInput
	universe  *domain.PermissionUniverse
	menuTree  []domain.MenuTreeNode
	err       error
}

func (f *fakeRBACUseCase) ResolvePermissions(ctx context.Context, input usecase.ResolvePermissionsInput) (*domain.PermissionUniverse, error) {
	f.input = input
	return f.universe, f.err
}

func (f *fakeRBACUseCase) GetUserMenu(ctx context.Context, input usecase.GetUserMenuInput) ([]domain.MenuTreeNode, error) {
	f.menuInput = input
	return f.menuTree, f.err
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

	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read"}}}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithTokenAndApp(router, http.MethodGet, "/api/v1/rbac/my-permissions", nil, token, uuid.New().String())

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

	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read", "party:read", "org:read"}}}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithTokenAndApp(router, http.MethodGet,
		"/api/v1/rbac/my-permissions?app_client_id="+appClientID.String(), nil, token, appClientID.String())

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.input.TenantID)
	assert.Equal(t, userID, uc.input.UserCredentialsID)
	assert.Equal(t, appClientID, uc.input.AppClientID)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	functions := data["functions"].([]any)
	require.Len(t, functions, 3)
	assert.Equal(t, "rbac:read", functions[0])
	assert.Equal(t, "party:read", functions[1])
	assert.Equal(t, "org:read", functions[2])
}

func TestMyPermissionsPropagatesUseCaseError(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)
	appClientID := uuid.New()

	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read"}}, err: errors.New("boom")}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithTokenAndApp(router, http.MethodGet,
		"/api/v1/rbac/my-permissions?app_client_id="+appClientID.String(), nil, token, appClientID.String())

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestMyMenuRequiresAuthentication(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read"}}, menuTree: []domain.MenuTreeNode{}}
	router := NewRouter(Dependencies{RBACUseCase: uc})

	recorder := requestWithTokenAndApp(router, http.MethodGet, "/api/v1/rbac/my-menu?app_client_id="+uuid.New().String(), nil, "", uuid.New().String())

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMyMenuRequiresAppClientID(t *testing.T) {
	manager := testAuthManager(t)
	token := accessToken(t, manager, uuid.New(), uuid.New(), uuid.New())
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read"}}, menuTree: []domain.MenuTreeNode{}}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithTokenAndApp(router, http.MethodGet, "/api/v1/rbac/my-menu", nil, token, uuid.New().String())

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "missing_app_client", body["code"])
}

func TestMyMenuReturnsTree(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)
	appClientID := uuid.New()

	expected := []domain.MenuTreeNode{
		{ID: uuid.New(), Code: "admin", Name: "Admin", SortOrder: 1, Options: []domain.MenuTreeOption{
			{ID: uuid.New(), Code: "users", Name: "Users", Route: "/users", SortOrder: 1},
		}},
	}
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"rbac:read"}}, menuTree: expected}
	router := NewRouter(Dependencies{RBACUseCase: uc, TokenManager: manager})

	recorder := requestWithTokenAndApp(router, http.MethodGet,
		"/api/v1/rbac/my-menu?app_client_id="+appClientID.String(), nil, token, appClientID.String())

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.menuInput.TenantID)
	assert.Equal(t, userID, uc.menuInput.UserCredentialsID)
	assert.Equal(t, appClientID, uc.menuInput.AppClientID)
	body := decodeBody(t, recorder)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	node := data[0].(map[string]any)
	assert.Equal(t, "admin", node["code"])
}
