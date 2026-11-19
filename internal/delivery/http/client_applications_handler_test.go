package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClientApplicationsUseCase struct {
	apps     []*domain.ClientApplication
	err      error
	tenantID uuid.UUID
	calls    int
}

func (f *fakeClientApplicationsUseCase) ListClientApplications(ctx context.Context, tenantID uuid.UUID) ([]*domain.ClientApplication, error) {
	f.calls++
	f.tenantID = tenantID
	return f.apps, f.err
}

func clientApplicationsTestRouter(uc ClientApplicationsUseCase, sc enttenant.SessionContext) *gin.Engine {
	router := gin.New()
	router.Use(ErrorMiddleware())
	handler := NewClientApplicationsHandler(uc)
	group := router.Group("")
	group.Use(sessionContextMiddleware(sc))
	group.GET("/api/v1/client-applications", handler.List)
	return router
}

func TestListClientApplicationsEndpoint(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	tenantID := uuid.New()
	uc := &fakeClientApplicationsUseCase{apps: []*domain.ClientApplication{{
		ID: uuid.New(), TenantID: &tenantID, Name: "Web", ClientID: "web-client",
		Scopes: []string{"parties:read", "org:read"}, IsActive: true, CreatedAt: now,
	}}}
	router := clientApplicationsTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: uuid.New()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/client-applications", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	app := data[0].(map[string]any)
	assert.Equal(t, "Web", app["name"])
	assert.Equal(t, "web-client", app["client_id"])
	assert.Equal(t, []any{"parties:read", "org:read"}, app["scopes"])
	assert.Equal(t, true, app["is_active"])
	assert.NotContains(t, app, "client_secret")
	assert.Equal(t, tenantID, uc.tenantID)
}

func TestListClientApplicationsEndpointRequiresSession(t *testing.T) {
	uc := &fakeClientApplicationsUseCase{}
	router := clientApplicationsTestRouter(uc, enttenant.SessionContext{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/client-applications", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, 0, uc.calls)
}
