package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"enterprise-system/ent"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/infrastructure/repository"
	"enterprise-system/internal/infrastructure/transaction"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errPreferencesRollback = errors.New("rollback test transaction")

type preferencesFixture struct {
	tenantID uuid.UUID
	partyA   uuid.UUID
	partyB   uuid.UUID
	credsA   uuid.UUID
	credsB   uuid.UUID
	appID    uuid.UUID
}

func openRepositoryIntegrationClient(t *testing.T) (*ent.Client, *transaction.Manager) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	client, err := enttenant.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client, transaction.NewManager(client)
}

func newPreferencesFixture() *preferencesFixture {
	return &preferencesFixture{
		tenantID: uuid.New(),
		partyA:   uuid.New(),
		partyB:   uuid.New(),
		credsA:   uuid.New(),
		credsB:   uuid.New(),
		appID:    uuid.New(),
	}
}

func insertPreferencesFixture(ctx context.Context, tx *ent.Tx, fx *preferencesFixture) error {
	contactA, contactB := uuid.New(), uuid.New()
	relationA, relationB := uuid.New(), uuid.New()
	slug := "pref-" + fx.tenantID.String()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO global.subscriptions (id, name, slug, status)
		VALUES ($1, 'Preferences Tenant', $2, 'ACTIVE')`,
		fx.tenantID, slug); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenants.parties (id, tenant_id, party_type, display_name, record_status)
		VALUES ($1, $2, 'INDIVIDUAL', 'User A', 'ACTIVE'),
		       ($3, $2, 'INDIVIDUAL', 'User B', 'ACTIVE')`,
		fx.partyA, fx.tenantID, fx.partyB); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO global.contact_mechanisms (id, type, value, can_receive_security_code)
		VALUES ($1, 'EMAIL', $2, true), ($3, 'EMAIL', $4, true)`,
		contactA, "a-"+contactA.String()+"@example.com",
		contactB, "b-"+contactB.String()+"@example.com"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenants.party_contact_mechanisms
			(id, tenant_id, party_id, contact_mechanism_id, contact_mechanism_order)
		VALUES ($1, $2, $3, $4, 0), ($5, $2, $6, $7, 0)`,
		relationA, fx.tenantID, fx.partyA, contactA,
		relationB, fx.partyB, contactB); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenants.user_credentials
			(id, tenant_id, party_id, party_contact_mechanism_id, password_hash, backup_codes_hash)
		VALUES ($1, $2, $3, $4, 'hash', '[]'::jsonb), ($5, $2, $6, $7, 'hash', '[]'::jsonb)`,
		fx.credsA, fx.tenantID, fx.partyA, relationA,
		fx.credsB, fx.partyB, relationB); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO global.client_applications
			(id, tenant_id, name, client_id, client_secret_hash, scopes)
		VALUES ($1, $2, 'Preferences App', $3, 'secret', ARRAY['preferences:read','preferences:write'])`,
		fx.appID, fx.tenantID, "pref-app-"+fx.appID.String()); err != nil {
		return err
	}
	return nil
}

func preferenceCount(t *testing.T, tx *ent.Tx, ctx context.Context, query string) int {
	t.Helper()
	rows, err := tx.QueryContext(ctx, query)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next())
	var count int
	require.NoError(t, rows.Scan(&count))
	return count
}

func TestUserPreferencesUpsertOverwritesJSONB(t *testing.T) {
	client, manager := openRepositoryIntegrationClient(t)
	fx := newPreferencesFixture()
	ctx := enttenant.WithSessionContext(context.Background(), enttenant.SessionContext{TenantID: fx.tenantID, UserID: fx.partyA})
	repo := repository.NewUserPreferencesRepository(client)
	now := time.Now().UTC()

	err := manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)
		if err := insertPreferencesFixture(ctx, tx, fx); err != nil {
			return err
		}

		first := &domain.UserPreferences{
			TenantID: fx.tenantID, UserID: fx.partyA, ClientApplicationID: fx.appID,
			Preferences: map[string]any{"theme": "light", "locale": "es"},
			CreatedAt:   now, UpdatedAt: now,
		}
		require.NoError(t, repo.Upsert(ctx, first))

		second := &domain.UserPreferences{
			TenantID: fx.tenantID, UserID: fx.partyA, ClientApplicationID: fx.appID,
			Preferences: map[string]any{"theme": "dark", "sidebar_collapsed": true},
			CreatedAt:   now, UpdatedAt: now.Add(time.Minute),
		}
		require.NoError(t, repo.Upsert(ctx, second))

		stored, err := repo.Get(ctx, fx.tenantID, fx.partyA, fx.appID)
		require.NoError(t, err)
		assert.Equal(t, "dark", stored.Preferences["theme"])
		assert.Equal(t, true, stored.Preferences["sidebar_collapsed"])
		assert.Nil(t, stored.Preferences["locale"], "upsert must replace, not merge")
		return errPreferencesRollback
	})
	require.ErrorIs(t, err, errPreferencesRollback)
}

func TestUserPreferencesRLSIsolatesUsers(t *testing.T) {
	_, manager := openRepositoryIntegrationClient(t)
	fx := newPreferencesFixture()
	ctxA := enttenant.WithSessionContext(context.Background(), enttenant.SessionContext{TenantID: fx.tenantID, UserID: fx.partyA})

	err := manager.WithinTransaction(ctxA, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)
		if err := insertPreferencesFixture(ctx, tx, fx); err != nil {
			return err
		}
		ctxA = enttenant.WithSessionContext(ctx, enttenant.SessionContext{TenantID: fx.tenantID, UserID: fx.partyA})
		ctxB := enttenant.WithSessionContext(ctx, enttenant.SessionContext{TenantID: fx.tenantID, UserID: fx.partyB})

		var err error
		_, err = tx.ExecContext(ctxA, `
			INSERT INTO tenants.user_app_preferences
				(id, tenant_id, user_credentials_id, client_application_id, preferences)
			VALUES ($1, $2, $3, $4, '{"theme":"light"}'::jsonb)`,
			uuid.New(), fx.tenantID, fx.credsA, fx.appID)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctxB, `
			INSERT INTO tenants.user_app_preferences
				(id, tenant_id, user_credentials_id, client_application_id, preferences)
			VALUES ($1, $2, $3, $4, '{"theme":"dark"}'::jsonb)`,
			uuid.New(), fx.tenantID, fx.credsB, fx.appID)
		require.NoError(t, err)

		countA := preferenceCount(t, tx, ctxA, `SELECT count(*) FROM tenants.user_app_preferences`)
		assert.Equal(t, 1, countA, "User A must only see its own preferences")
		countB := preferenceCount(t, tx, ctxB, `SELECT count(*) FROM tenants.user_app_preferences`)
		assert.Equal(t, 1, countB, "User B must only see its own preferences")

		otherCount := preferenceCount(t, tx, ctxA,
			`SELECT count(*) FROM tenants.user_app_preferences WHERE user_credentials_id = '`+fx.credsB.String()+`'`)
		assert.Equal(t, 0, otherCount, "User A must not see User B's row")

		// WITH CHECK must reject inserting a row for another user.
		subErr := manager.WithinSavepoint(ctxA, func(ctx context.Context) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO tenants.user_app_preferences
					(id, tenant_id, user_credentials_id, client_application_id, preferences)
				VALUES ($1, $2, $3, $4, '{}'::jsonb)`,
				uuid.New(), fx.tenantID, fx.credsB, fx.appID)
			return err
		})
		require.Error(t, subErr)
		return errPreferencesRollback
	})
	require.ErrorIs(t, err, errPreferencesRollback)
}
