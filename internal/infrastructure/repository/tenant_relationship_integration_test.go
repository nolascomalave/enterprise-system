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

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, subsidiary.String())
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`,
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

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, subsidiary.String())
	require.NoError(t, err)

	relationshipID := uuid.New()
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'APPROVED')`,
		relationshipID, holding, subsidiary)
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, holding.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_transfers
		(id, relationship_id, requested_by_tenant_id, status)
		VALUES ($1, $2, $3, 'TRANSFER_REQUESTED')`,
		uuid.New(), relationshipID, holding)
	require.NoError(t, err)

	// Holding can see its transfer.
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, holding.String())
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM global.tenant_transfers WHERE relationship_id = $1`, relationshipID).Scan(&count))
	require.Equal(t, 1, count)

	// Outsider sees nothing.
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, outsider.String())
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM global.tenant_transfers WHERE relationship_id = $1`, relationshipID).Scan(&count))
	require.Equal(t, 0, count)

	// Outsider cannot insert.
	_, err = tx.Exec(`SAVEPOINT sp_outsider`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_transfers
		(id, relationship_id, requested_by_tenant_id, status)
		VALUES ($1, $2, $3, 'TRANSFER_REQUESTED')`,
		uuid.New(), relationshipID, outsider)
	require.Error(t, err)
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT sp_outsider`)
	require.NoError(t, err)
	_, err = tx.Exec(`RELEASE SAVEPOINT sp_outsider`)
	require.NoError(t, err)
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
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, holding.String())
	require.NoError(t, err)
	_, err = tx.Exec(`SAVEPOINT sp_holding`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.Error(t, err)
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT sp_holding`)
	require.NoError(t, err)
	_, err = tx.Exec(`RELEASE SAVEPOINT sp_holding`)
	require.NoError(t, err)

	// Subsidiary can.
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, subsidiary.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`,
		uuid.New(), holding, subsidiary)
	require.NoError(t, err)
}

func TestRelationshipCycleFunctionDetectsCrossTenantCycle(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'A', $2, 'ACTIVE'), ($3, 'B', $4, 'ACTIVE'),
		($5, 'C', $6, 'ACTIVE'), ($7, 'D', $8, 'ACTIVE')`,
		a, "a-"+a.String(), b, "b-"+b.String(), c, "c-"+c.String(), d, "d-"+d.String())
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, c.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'APPROVED')`, uuid.New(), b, c)
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, a.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'APPROVED')`, uuid.New(), c, a)
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, b.String())
	require.NoError(t, err)
	var cycle bool
	require.NoError(t, tx.QueryRow(`SELECT global.fn_check_relationship_cycle($1, $2)`, a, b).Scan(&cycle))
	require.True(t, cycle, "the cross-tenant cycle must be detected")

	var nonCycle bool
	require.NoError(t, tx.QueryRow(`SELECT global.fn_check_relationship_cycle($1, $2)`, d, b).Scan(&nonCycle))
	require.False(t, nonCycle)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, c.String())
	require.NoError(t, err)
	_, err = tx.Exec(`SELECT global.fn_check_relationship_cycle($1, $2)`, a, b)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "42501", string(pqErr.Code), "outsider must be rejected")
}

func TestRelationshipCycleFunctionRejectsGraphTooDeep(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	_, err = tx.Exec(`CREATE TEMP TABLE audit_nodes ON COMMIT DROP AS
		SELECT gs AS idx, gen_random_uuid() AS id FROM generate_series(1, 23) gs`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status)
		SELECT id, 'Node '||idx, 'node-'||id, 'ACTIVE' FROM audit_nodes`)
	require.NoError(t, err)

	_, err = tx.Exec(`DO $$
	DECLARE
		v_holding uuid;
		v_subsidiary uuid;
		i int;
	BEGIN
		FOR i IN 1..22 LOOP
			SELECT id INTO v_holding FROM audit_nodes WHERE idx = i;
			SELECT id INTO v_subsidiary FROM audit_nodes WHERE idx = i + 1;
			PERFORM set_config('app.current_tenant_id', v_subsidiary::text, true);
			INSERT INTO global.tenant_relationships
				(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
			VALUES
				(gen_random_uuid(), v_holding, v_subsidiary, 'SUBSIDIARY_OF', 'APPROVED');
		END LOOP;
	END
	$$`)
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', (SELECT id::text FROM audit_nodes WHERE idx = 2), true)`)
	require.NoError(t, err)

	var result bool
	err = tx.QueryRow(`SELECT global.fn_check_relationship_cycle(
		(SELECT id FROM audit_nodes WHERE idx = 1),
		(SELECT id FROM audit_nodes WHERE idx = 2)
	)`).Scan(&result)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "54000", string(pqErr.Code))
}

func TestRelationshipTypeCatalogCheck(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	holding, subsidiary := uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Holding', $2, 'ACTIVE'), ($3, 'Subsidiary', $4, 'ACTIVE')`,
		holding, "holding-"+holding.String(), subsidiary, "subsidiary-"+subsidiary.String())
	require.NoError(t, err)
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, subsidiary.String())
	require.NoError(t, err)

	_, err = tx.Exec(`SAVEPOINT sp_type`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'OWNERSHIP', 'PENDING')`, uuid.New(), holding, subsidiary)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "23514", string(pqErr.Code))
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT sp_type`)
	require.NoError(t, err)
	_, err = tx.Exec(`RELEASE SAVEPOINT sp_type`)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`, uuid.New(), holding, subsidiary)
	require.NoError(t, err)
}

func TestUniqueActiveInvitePerRelationship(t *testing.T) {
	db := tenantRelationshipTestDB(t)

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	holding, subsidiary, relationshipID := uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Holding', $2, 'ACTIVE'), ($3, 'Subsidiary', $4, 'ACTIVE')`,
		holding, "holding-"+holding.String(), subsidiary, "subsidiary-"+subsidiary.String())
	require.NoError(t, err)
	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, subsidiary.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationships
		(id, tenant_id_holding, tenant_id_subsidiary, relationship_type, status)
		VALUES ($1, $2, $3, 'SUBSIDIARY_OF', 'PENDING')`,
		relationshipID, holding, subsidiary)
	require.NoError(t, err)

	_, err = tx.Exec(`SELECT set_config('app.current_tenant_id', $1, true)`, holding.String())
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationship_invites
		(relationship_id, token_hash, expires_at)
		VALUES ($1, decode('01','hex'), now() + interval '1 day')`, relationshipID)
	require.NoError(t, err)

	_, err = tx.Exec(`SAVEPOINT sp_duplicate_invite`)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationship_invites
		(relationship_id, token_hash, expires_at)
		VALUES ($1, decode('02','hex'), now() + interval '1 day')`, relationshipID)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "23505", string(pqErr.Code))
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT sp_duplicate_invite`)
	require.NoError(t, err)
	_, err = tx.Exec(`RELEASE SAVEPOINT sp_duplicate_invite`)
	require.NoError(t, err)

	_, err = tx.Exec(`UPDATE global.tenant_relationship_invites
		SET canceled_at = now(), canceled_by_tenant_id = $1
		WHERE relationship_id = $2 AND canceled_at IS NULL AND consumed_at IS NULL`,
		holding, relationshipID)
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO global.tenant_relationship_invites
		(relationship_id, token_hash, expires_at)
		VALUES ($1, decode('03','hex'), now() + interval '1 day')`, relationshipID)
	require.NoError(t, err)
}
