package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sessionContextMiddleware(sc enttenant.SessionContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := enttenant.WithSessionContext(c.Request.Context(), sc)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func rbacTestRouter(uc RBACUseCase, sc enttenant.SessionContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.Use(sessionContextMiddleware(sc))
	router.Use(RBACMiddleware(uc))
	router.GET("/api/v1/parties", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	router.GET("/api/v1/unmapped", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return router
}

func TestRBACMiddlewareAllowsM2MToken(t *testing.T) {
	uc := &fakeRBACUseCase{}
	router := rbacTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New()})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestRBACMiddlewareBlocksUnmappedRoute(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"party:read"}}}
	router := rbacTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unmapped", nil)
	req.Header.Set("X-App-Client-ID", uuid.New().String())
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "forbidden", body["code"])
}

func TestRBACMiddlewareRequiresAppClientIDHeader(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"party:read"}}}
	router := rbacTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "missing_app_client", body["code"])
}

func TestRBACMiddlewareBlocksMissingPermission(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"org:read"}}}
	tenantID := uuid.New()
	userID := uuid.New()
	router := rbacTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: userID})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	appClientID := uuid.New()
	req.Header.Set("X-App-Client-ID", appClientID.String())
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, tenantID, uc.input.TenantID)
	require.Equal(t, userID, uc.input.UserCredentialsID)
	require.Equal(t, appClientID, uc.input.AppClientID)
}

func TestRBACMiddlewareAllowsWithPermission(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"party:read"}}}
	router := rbacTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	req.Header.Set("X-App-Client-ID", uuid.New().String())
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestRBACMiddlewareInjectsUniverse(t *testing.T) {
	uc := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"party:read"}}}
	router := gin.New()
	router.Use(ErrorMiddleware())
	router.Use(sessionContextMiddleware(enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()}))
	router.Use(RBACMiddleware(uc))
	var injected *domain.PermissionUniverse
	router.GET("/api/v1/parties", func(c *gin.Context) {
		injected, _ = PermissionUniverseFromContext(c.Request.Context())
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/parties", nil)
	req.Header.Set("X-App-Client-ID", uuid.New().String())
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, injected)
	assert.Equal(t, []string{"party:read"}, injected.Functions)
}
