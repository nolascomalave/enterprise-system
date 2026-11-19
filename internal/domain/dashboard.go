package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidDashboardTenant = errors.New("invalid dashboard tenant")

type DashboardRecentParty struct {
	ID        uuid.UUID
	Name      string
	PartyType PartyType
	CreatedAt time.Time
}

type DashboardSummary struct {
	TotalParties     int
	TotalEmployees   int
	TotalDepartments int
	RecentParties    []*DashboardRecentParty
}

type DashboardRepository interface {
	CountActiveParties(ctx context.Context, tenantID uuid.UUID) (int, error)
	CountActiveEmployees(ctx context.Context, tenantID uuid.UUID) (int, error)
	CountActiveDepartments(ctx context.Context, tenantID uuid.UUID) (int, error)
	ListRecentParties(ctx context.Context, tenantID uuid.UUID, limit int) ([]*DashboardRecentParty, error)
}
