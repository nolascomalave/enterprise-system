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

// TestDepartmentCompositeFKBlocksCrossTenant runs only when TEST_DATABASE_URL
// points to a database with the migrations applied. It proves that the
// composite (department_id, tenant_id) FK blocks a row that mixes tenants.
func TestDepartmentCompositeFKBlocksCrossTenant(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// RLS would reject the cross-tenant row before the FK; the test targets the
	// FK itself. The transaction rollback restores RLS.
	_, err = tx.Exec(`ALTER TABLE tenants.department_contact_mechanisms DISABLE ROW LEVEL SECURITY`)
	require.NoError(t, err)

	tenantA, tenantB := uuid.New(), uuid.New()
	partyA, partyB := uuid.New(), uuid.New()
	deptA, deptB := uuid.New(), uuid.New()
	contactID := uuid.New()

	_, err = tx.Exec(`INSERT INTO global.subscriptions (id, name, slug, status) VALUES
		($1, 'Tenant A', $2, 'ACTIVE'), ($3, 'Tenant B', $4, 'ACTIVE')`,
		tenantA, "fk-a-"+tenantA.String(), tenantB, "fk-b-"+tenantB.String())
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO tenants.parties (id, tenant_id, party_type, display_name, record_status) VALUES
		($1, $2, 'ORGANIZATION', 'Party A', 'ACTIVE'),
		($3, $4, 'ORGANIZATION', 'Party B', 'ACTIVE')`,
		partyA, tenantA, partyB, tenantB)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO tenants.departments (id, tenant_id, name, code, unit_type) VALUES
		($1, $2, 'Dept A', 'DA', 'DEPARTMENT'),
		($3, $4, 'Dept B', 'DB', 'DEPARTMENT')`,
		deptA, tenantA, deptB, tenantB)
	require.NoError(t, err)

	_, err = tx.Exec(`INSERT INTO global.contact_mechanisms (id, type, value) VALUES ($1, 'PHONE', $2)`,
		contactID, "+58-"+contactID.String())
	require.NoError(t, err)

	// Same tenant: valid.
	_, err = tx.Exec(`INSERT INTO tenants.department_contact_mechanisms
		(id, tenant_id, department_id, contact_mechanism_id, contact_mechanism_order, is_primary)
		VALUES ($1, $2, $3, $4, 0, false)`,
		uuid.New(), tenantA, deptA, contactID)
	require.NoError(t, err)

	// Different tenant: must fail with foreign_key_violation.
	_, err = tx.Exec(`INSERT INTO tenants.department_contact_mechanisms
		(id, tenant_id, department_id, contact_mechanism_id, contact_mechanism_order, is_primary)
		VALUES ($1, $2, $3, $4, 0, false)`,
		uuid.New(), tenantA, deptB, contactID)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, "23503", string(pqErr.Code), "expected foreign_key_violation")
}
