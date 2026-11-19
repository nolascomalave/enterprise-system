package usecase

import (
	"context"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

const dashboardRecentPartiesLimit = 5

type DashboardService struct {
	repo domain.DashboardRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewDashboardService(repo domain.DashboardRepository, tx TransactionManager, now func() time.Time) *DashboardService {
	if now == nil {
		now = time.Now
	}
	return &DashboardService{repo: repo, tx: tx, now: now}
}

// GetDashboardSummary returns the tenant dashboard counters and the most recent
// parties in a single read transaction so RLS receives the tenant context.
func (s *DashboardService) GetDashboardSummary(ctx context.Context, tenantID uuid.UUID) (*domain.DashboardSummary, error) {
	if tenantID == uuid.Nil {
		return nil, validationError("invalid_dashboard_tenant", "tenant_id is required", domain.ErrInvalidDashboardTenant)
	}

	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.DashboardSummary, error) {
		totalParties, err := s.repo.CountActiveParties(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("count active parties: %w", err)
		}
		totalEmployees, err := s.repo.CountActiveEmployees(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("count active employees: %w", err)
		}
		totalDepartments, err := s.repo.CountActiveDepartments(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("count active departments: %w", err)
		}
		recentParties, err := s.repo.ListRecentParties(ctx, tenantID, dashboardRecentPartiesLimit)
		if err != nil {
			return nil, fmt.Errorf("list recent parties: %w", err)
		}
		return &domain.DashboardSummary{
			TotalParties:     totalParties,
			TotalEmployees:   totalEmployees,
			TotalDepartments: totalDepartments,
			RecentParties:    recentParties,
		}, nil
	})
}
