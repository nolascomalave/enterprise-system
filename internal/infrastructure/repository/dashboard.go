package repository

import (
	"context"

	"enterprise-system/ent"
	"enterprise-system/ent/department"
	"enterprise-system/ent/employee"
	"enterprise-system/ent/party"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.DashboardRepository = (*DashboardRepository)(nil)

type DashboardRepository struct {
	client *ent.Client
}

func NewDashboardRepository(client *ent.Client) *DashboardRepository {
	return &DashboardRepository{client: client}
}

func (r *DashboardRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

func (r *DashboardRepository) CountActiveParties(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.withClient(ctx, func(client *ent.Client) error {
		var err error
		count, err = client.Party.Query().
			Where(
				party.TenantID(tenantID),
				party.DeletedAtIsNil(),
				party.RecordStatusEQ(party.RecordStatusACTIVE),
			).
			Count(ctx)
		return repoError(err)
	})
	return count, err
}

func (r *DashboardRepository) CountActiveEmployees(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.withClient(ctx, func(client *ent.Client) error {
		var err error
		count, err = client.Employee.Query().
			Where(
				employee.TenantID(tenantID),
				employee.DeletedAtIsNil(),
				employee.StatusEQ(employee.StatusACTIVE),
			).
			Count(ctx)
		return repoError(err)
	})
	return count, err
}

func (r *DashboardRepository) CountActiveDepartments(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := r.withClient(ctx, func(client *ent.Client) error {
		var err error
		count, err = client.Department.Query().
			Where(
				department.TenantID(tenantID),
				department.DeletedAtIsNil(),
			).
			Count(ctx)
		return repoError(err)
	})
	return count, err
}

func (r *DashboardRepository) ListRecentParties(ctx context.Context, tenantID uuid.UUID, limit int) ([]*domain.DashboardRecentParty, error) {
	if limit <= 0 {
		return []*domain.DashboardRecentParty{}, nil
	}
	var result []*domain.DashboardRecentParty
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.Party.Query().
			Where(
				party.TenantID(tenantID),
				party.DeletedAtIsNil(),
				party.RecordStatusEQ(party.RecordStatusACTIVE),
			).
			Order(ent.Desc(party.FieldCreatedAt)).
			Limit(limit).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.DashboardRecentParty, 0, len(rows))
		for _, row := range rows {
			result = append(result, &domain.DashboardRecentParty{
				ID:        row.ID,
				Name:      row.DisplayName,
				PartyType: domain.PartyType(row.PartyType),
				CreatedAt: row.CreatedAt,
			})
		}
		return nil
	})
	return result, err
}
