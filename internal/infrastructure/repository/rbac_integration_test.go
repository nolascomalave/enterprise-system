package repository_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestTenantRoleRejectsCrossTenantClientApplication(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	tenantA, tenantB := uuid.New(), uuid.New()
	appA, appB := uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Tenant A', $2, 'ACTIVE'), ($3, 'Tenant B', $4, 'ACTIVE')`,
		tenantA, "a-"+tenantA.String(), tenantB, "b-"+tenantB.String())
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.client_applications
		(id, tenant_id, name, client_id, client_secret_hash, scopes, client_type, application_scope, is_active)
		VALUES
		($1, $2, 'App A', $3, 'dummy-a', ARRAY['parties:read'], 'THIRD_PARTY', 'TENANT_SPECIFIC', true),
		($4, $5, 'App B', $6, 'dummy-b', ARRAY['parties:read'], 'THIRD_PARTY', 'TENANT_SPECIFIC', true)`,
		appA, tenantA, "app-a-"+appA.String(), appB, tenantB, "app-b-"+appB.String())
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, tenantA.String())
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO tenants.tenant_roles (tenant_id, client_application_id, name)
		VALUES ($1, $2, 'Valid Role')`, tenantA, appA)
	require.NoError(t, err)

	_, err = tx.Exec(`SAVEPOINT sp_cross_tenant`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO tenants.tenant_roles (tenant_id, client_application_id, name)
		VALUES ($1, $2, 'Cross Tenant Role')`, tenantA, appB)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "23503", string(pqErr.Code))
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT sp_cross_tenant`)
	require.NoError(t, err)
	_, err = tx.Exec(`RELEASE SAVEPOINT sp_cross_tenant`)
	require.NoError(t, err)
}
