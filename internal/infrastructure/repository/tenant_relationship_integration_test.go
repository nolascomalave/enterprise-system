package repository_test

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func tenantRelationshipTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var exists bool
	err = db.QueryRow(`SELECT to_regclass('global.tenant_relationships') IS NOT NULL`).Scan(&exists)
	require.NoError(t, err)
	if !exists {
		t.Skip("migration 0021 is not applied")
	}
	return db
}

func TestTenantRelationshipsRSLIsEnabled(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	for _, table := range []string{"tenant_relationships", "tenant_relationship_invites", "tenant_transfers"} {
		var enabled, forced bool
		err := db.QueryRow(`
			SELECT c.relrowsecurity, c.relforcerowsecurity
			  FROM pg_class c
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = 'global' AND c.relname = $1`, table).Scan(&enabled, &forced)
		require.NoError(t, err)
		require.True(t, enabled, "%s must have RLS enabled", table)
		require.True(t, forced, "%s must have RLS forced", table)
	}
}

func TestTenantRelationshipsHasExplicitPolicies(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	expected := map[string][]string{
		"tenant_relationships":        {"SELECT", "INSERT", "UPDATE"},
		"tenant_relationship_invites": {"SELECT", "INSERT", "UPDATE"},
		"tenant_transfers":            {"SELECT", "INSERT", "UPDATE"},
	}
	for table, commands := range expected {
		for _, command := range commands {
			var count int
			err := db.QueryRow(`
				SELECT count(*) FROM pg_policies
				 WHERE schemaname = 'global' AND tablename = $1 AND cmd = $2`, table, command).Scan(&count)
			require.NoError(t, err)
			require.Greater(t, count, 0, "%s must have a %s policy", table, command)
		}
	}
}

func TestTenantRelationshipsPendingUniqueness(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	holding, subsidiary := uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Holding', $2, 'ACTIVE'), ($3, 'Subsidiary', $4, 'ACTIVE')`,
		holding, "holding-"+holding.String(), subsidiary, "subsidiary-"+subsidiary.String())
	require.NoError(t, err)

	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, subsidiary.String())
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.Error(t, err)

	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "23505", string(pqErr.Code), "expected unique_violation")
}

func TestTenantTransfersRLSTenantIsolation(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	var superuser bool
	require.NoError(t, db.QueryRow(`SELECT current_setting('is_superuser')::boolean`).Scan(&superuser))
	if superuser {
		t.Skip("RLS data tests require a non-superuser role")
	}

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	holding, subsidiary, outsider := uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Holding', $2, 'ACTIVE'), ($3, 'Subsidiary', $4, 'ACTIVE'), ($5, 'Outsider', $6, 'ACTIVE')`,
		holding, "holding-"+holding.String(),
		subsidiary, "subsidiary-"+subsidiary.String(),
		outsider, "outsider-"+outsider.String())
	require.NoError(t, err)

	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, subsidiary.String())
	require.NoError(t, err)

	relationshipID := uuid.New()
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'APPROVED')`,
		relationshipID, holding, subsidiary)
	require.NoError(t, err)

	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, holding.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_transfers
		(id, relationship_id, requested_by_tenant_id, status)
		VALUES ($1, $2, $3, 'TRANSFER_REQUESTED')`,
		uuid.New(), relationshipID, holding)
	require.NoError(t, err)

	// Holding can see its transfer.
	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, holding.String())
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM global.tenant_transfers WHERE relationship_id = $1`, relationshipID).Scan(&count))
	require.Equal(t, 1, count)

	// Outsider sees nothing.
	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, outsider.String())
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM global.tenant_transfers WHERE relationship_id = $1`, relationshipID).Scan(&count))
	require.Equal(t, 0, count)

	// Outsider cannot insert.
	_, err = tx.Exec(`INSERT INTO global.tenant_transfers
		(id, relationship_id, requested_by_tenant_id, status)
		VALUES ($1, $2, $3, 'TRANSFER_REQUESTED')`,
		uuid.New(), relationshipID, outsider)
	require.Error(t, err)
}

func TestTenantRelationshipsInsertOnlyBySubsidiary(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	var superuser bool
	require.NoError(t, db.QueryRow(`SELECT current_setting('is_superuser')::boolean`).Scan(&superuser))
	if superuser {
		t.Skip("RLS data tests require a non-superuser role")
	}

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	holding, subsidiary := uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Holding', $2, 'ACTIVE'), ($3, 'Subsidiary', $4, 'ACTIVE')`,
		holding, "holding-"+holding.String(), subsidiary, "subsidiary-"+subsidiary.String())
	require.NoError(t, err)

	// Holding cannot create the PENDING relationship.
	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, holding.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.Error(t, err)

	// Subsidiary can.
	_, err = tx.Exec(`SET LOCAL app.current_tenant_id = $1`, subsidiary.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.NoError(t, err)
}
