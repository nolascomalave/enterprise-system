package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDashboardRepository struct {
	totalParties     int
	totalEmployees   int
	totalDepartments int
	recent           []*domain.DashboardRecentParty
	countErr         error
	recentErr        error
	recentLimit      int
	countCalls       int
	recentCalls      int
}

func (r *fakeDashboardRepository) CountActiveParties(ctx context.Context, tenantID uuid.UUID) (int, error) {
	r.countCalls++
	if r.countErr != nil {
		return 0, r.countErr
	}
	return r.totalParties, nil
}

func (r *fakeDashboardRepository) CountActiveEmployees(ctx context.Context, tenantID uuid.UUID) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	return r.totalEmployees, nil
}

func (r *fakeDashboardRepository) CountActiveDepartments(ctx context.Context, tenantID uuid.UUID) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}
	return r.totalDepartments, nil
}

func (r *fakeDashboardRepository) ListRecentParties(ctx context.Context, tenantID uuid.UUID, limit int) ([]*domain.DashboardRecentParty, error) {
	r.recentCalls++
	r.recentLimit = limit
	if r.recentErr != nil {
		return nil, r.recentErr
	}
	return r.recent, nil
}

func dashboardTestService(repo *fakeDashboardRepository) *DashboardService {
	return NewDashboardService(repo, newFakeTxManager(), time.Now)
}

func TestGetDashboardSummary(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	recent := []*domain.DashboardRecentParty{{
		ID: uuid.New(), Name: "Jane Doe", PartyType: domain.PartyTypeIndividual, CreatedAt: now,
	}}
	repo := &fakeDashboardRepository{
		totalParties: 10, totalEmployees: 5, totalDepartments: 3, recent: recent,
	}
	service := dashboardTestService(repo)

	summary, err := service.GetDashboardSummary(context.Background(), uuid.New())

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, 10, summary.TotalParties)
	assert.Equal(t, 5, summary.TotalEmployees)
	assert.Equal(t, 3, summary.TotalDepartments)
	assert.Equal(t, recent, summary.RecentParties)
	assert.Equal(t, 5, repo.recentLimit, "the dashboard must request the 5 most recent parties")
}

func TestGetDashboardSummaryRequiresTenant(t *testing.T) {
	service := dashboardTestService(&fakeDashboardRepository{})

	_, err := service.GetDashboardSummary(context.Background(), uuid.Nil)

	require.ErrorIs(t, err, domain.ErrInvalidDashboardTenant)
}

func TestGetDashboardSummaryPropagatesCountError(t *testing.T) {
	repo := &fakeDashboardRepository{countErr: errors.New("database down")}
	service := dashboardTestService(repo)

	_, err := service.GetDashboardSummary(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database down")
}

func TestGetDashboardSummaryPropagatesRecentError(t *testing.T) {
	repo := &fakeDashboardRepository{recentErr: errors.New("recent query failed")}
	service := dashboardTestService(repo)

	_, err := service.GetDashboardSummary(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "recent query failed")
}
