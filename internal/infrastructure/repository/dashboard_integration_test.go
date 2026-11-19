package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/infrastructure/repository"
	"enterprise-system/internal/infrastructure/transaction"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDashboardRollback = errors.New("rollback dashboard test transaction")

func TestDashboardRepositoryCountsAndRecentParties(t *testing.T) {
	client, manager := openRepositoryIntegrationClient(t)
	tenantID := uuid.New()
	ctx := enttenant.WithSessionContext(context.Background(), enttenant.SessionContext{TenantID: tenantID})
	repo := repository.NewDashboardRepository(client)

	base := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	parties := make([]uuid.UUID, 8)
	for i := range parties {
		parties[i] = uuid.New()
	}
	jobFamilyID, jobID, departmentID := uuid.New(), uuid.New(), uuid.New()
	deletedDepartmentID := uuid.New()
	employeeParty, terminatedParty, deletedEmployeeParty := parties[0], parties[1], parties[2]

	err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO global.subscriptions (id, name, slug, status)
			VALUES ($1, 'Dashboard Tenant', $2, 'ACTIVE')`,
			tenantID, "dashboard-"+tenantID.String())
		require.NoError(t, err)

		for i := 0; i < 6; i++ {
			status := "ACTIVE"
			if i == 0 {
				status = "ACTIVE"
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO tenants.parties (id, tenant_id, party_type, display_name, record_status, created_at, updated_at)
				VALUES ($1, $2, 'INDIVIDUAL', $3, $4, $5, $5)`,
				parties[i], tenantID, "Active "+string(rune('A'+i)), status, base.Add(time.Duration(i+1)*time.Minute))
			require.NoError(t, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.parties (id, tenant_id, party_type, display_name, record_status, created_at, updated_at)
			VALUES ($1, $2, 'INDIVIDUAL', 'Archived', 'ARCHIVED', $3, $3),
			       ($4, $2, 'INDIVIDUAL', 'Deleted', 'ACTIVE', $3, $3)`,
			parties[6], tenantID, base, parties[7])
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
			UPDATE tenants.parties SET deleted_at = now() WHERE id = $1`, parties[7])
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.job_families (id, tenant_id, name, code)
			VALUES ($1, $2, 'Engineering', 'ENG')`, jobFamilyID, tenantID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.jobs (id, tenant_id, job_family_id, title, code)
			VALUES ($1, $2, $3, 'Engineer', 'ENG-1')`, jobID, tenantID, jobFamilyID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.departments (id, tenant_id, name, code, unit_type)
			VALUES ($1, $2, 'Platform', 'PLAT', 'DEPARTMENT'),
			       ($3, $2, 'Deleted Dept', 'DEL', 'DEPARTMENT')`,
			departmentID, tenantID, deletedDepartmentID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
			UPDATE tenants.departments SET deleted_at = now() WHERE id = $1`, deletedDepartmentID)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.employees
				(party_id, tenant_id, employee_code, hire_date, status, job_id, department_id)
			VALUES
				($1, $2, 'EMP-1', now(), 'ACTIVE', $3, $4),
				($5, $2, 'EMP-2', now(), 'TERMINATED', $3, $4)`,
			employeeParty, tenantID, jobID, departmentID, terminatedParty)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenants.employees
				(party_id, tenant_id, employee_code, hire_date, status, job_id, department_id, deleted_at)
			VALUES ($1, $2, 'EMP-3', now(), 'ACTIVE', $3, $4, now())`,
			deletedEmployeeParty, tenantID, jobID, departmentID)
		require.NoError(t, err)

		totalParties, err := repo.CountActiveParties(ctx, tenantID)
		require.NoError(t, err)
		assert.Equal(t, 6, totalParties)

		totalEmployees, err := repo.CountActiveEmployees(ctx, tenantID)
		require.NoError(t, err)
		assert.Equal(t, 1, totalEmployees)

		totalDepartments, err := repo.CountActiveDepartments(ctx, tenantID)
		require.NoError(t, err)
		assert.Equal(t, 1, totalDepartments)

		recent, err := repo.ListRecentParties(ctx, tenantID, 5)
		require.NoError(t, err)
		require.Len(t, recent, 5)
		assert.Equal(t, "Active F", recent[0].Name)
		assert.Equal(t, "Active E", recent[1].Name)
		assert.Equal(t, "Active D", recent[2].Name)
		assert.Equal(t, "Active C", recent[3].Name)
		assert.Equal(t, "Active B", recent[4].Name)
		return errDashboardRollback
	})
	require.ErrorIs(t, err, errDashboardRollback)
}
