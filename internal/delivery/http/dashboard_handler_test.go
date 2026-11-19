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

type fakeDashboardUseCase struct {
	summary    *domain.DashboardSummary
	err        error
	tenantID   uuid.UUID
	tenantSeen bool
}

func (f *fakeDashboardUseCase) GetDashboardSummary(ctx context.Context, tenantID uuid.UUID) (*domain.DashboardSummary, error) {
	f.tenantID = tenantID
	f.tenantSeen = true
	return f.summary, f.err
}

func dashboardTestRouter(uc DashboardUseCase, sc enttenant.SessionContext) *gin.Engine {
	router := gin.New()
	router.Use(ErrorMiddleware())
	handler := NewDashboardHandler(uc)
	group := router.Group("")
	group.Use(sessionContextMiddleware(sc))
	group.GET("/api/v1/dashboard/summary", handler.GetSummary)
	return router
}

func TestGetDashboardSummaryEndpoint(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	tenantID := uuid.New()
	uc := &fakeDashboardUseCase{summary: &domain.DashboardSummary{
		TotalParties: 10, TotalEmployees: 5, TotalDepartments: 3,
		RecentParties: []*domain.DashboardRecentParty{{
			ID: uuid.New(), Name: "Jane Doe", PartyType: domain.PartyTypeIndividual, CreatedAt: now,
		}},
	}}
	router := dashboardTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: uuid.New()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(10), data["total_parties"])
	assert.Equal(t, float64(5), data["total_employees"])
	assert.Equal(t, float64(3), data["total_departments"])
	recent := data["recent_parties"].([]any)
	require.Len(t, recent, 1)
	assert.Equal(t, "Jane Doe", recent[0].(map[string]any)["name"])
	assert.Equal(t, tenantID, uc.tenantID)
}

func TestGetDashboardSummaryEndpointRequiresSession(t *testing.T) {
	uc := &fakeDashboardUseCase{}
	router := dashboardTestRouter(uc, enttenant.SessionContext{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.False(t, uc.tenantSeen)
}
