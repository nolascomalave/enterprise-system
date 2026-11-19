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

var errClientApplicationsRollback = errors.New("rollback client applications test transaction")

func TestClientApplicationRepositoryListsOnlyTenantApplications(t *testing.T) {
	client, manager := openRepositoryIntegrationClient(t)
	tenantA, tenantB := uuid.New(), uuid.New()
	ctx := enttenant.WithSessionContext(context.Background(), enttenant.SessionContext{TenantID: tenantA})
	repo := repository.NewClientApplicationRepository(client)
	now := time.Now().UTC().Truncate(time.Second)

	appA1, appA2, appADeleted, appB := uuid.New(), uuid.New(), uuid.New(), uuid.New()

	err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)

		_, err := tx.ExecContext(ctx, `
			INSERT INTO global.subscriptions (id, name, slug, status)
			VALUES ($1, 'Tenant A', $2, 'ACTIVE'), ($3, 'Tenant B', $4, 'ACTIVE')`,
			tenantA, "tenant-a-"+tenantA.String(), tenantB, "tenant-b-"+tenantB.String())
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO global.client_applications
				(id, tenant_id, name, client_id, client_secret_hash, scopes,
				 client_type, application_scope, is_active, created_at, updated_at, deleted_at)
			VALUES
				($1, $2, 'Web', $3, 'secret', ARRAY['parties:read'], 'FIRST_PARTY', 'TENANT_SPECIFIC', true, $4, $4, NULL),
				($5, $2, 'Mobile', $6, 'secret', ARRAY['parties:read'], 'FIRST_PARTY', 'TENANT_SPECIFIC', false, $7, $7, NULL),
				($8, $2, 'Deleted', $9, 'secret', ARRAY['parties:read'], 'FIRST_PARTY', 'TENANT_SPECIFIC', true, $10, $10, now()),
				($11, $12, 'Other Tenant', $13, 'secret', ARRAY['parties:read'], 'FIRST_PARTY', 'TENANT_SPECIFIC', true, $14, $14, NULL)`,
			appA1, tenantA, "web-"+appA1.String(), now.Add(-time.Minute),
			appA2, "mobile-"+appA2.String(), now.Add(-2*time.Minute),
			appADeleted, "deleted-"+appADeleted.String(), now.Add(-3*time.Minute),
			appB, tenantB, "other-"+appB.String(), now)
		require.NoError(t, err)

		applications, err := repo.ListByTenant(ctx, tenantA)
		require.NoError(t, err)
		require.Len(t, applications, 2)
		assert.Equal(t, "Web", applications[0].Name, "newest active app first")
		assert.Equal(t, "Mobile", applications[1].Name)
		assert.False(t, applications[1].IsActive, "inactive apps must be listed")
		for _, application := range applications {
			require.NotNil(t, application.TenantID)
			assert.Equal(t, tenantA, *application.TenantID)
		}
		return errClientApplicationsRollback
	})
	require.ErrorIs(t, err, errClientApplicationsRollback)
}
