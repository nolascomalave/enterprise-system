package transaction_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/infrastructure/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManagerIntegrationSavepointKeepsParentWork runs only when
// TEST_DATABASE_URL points to a disposable PostgreSQL database. It proves the
// real ROLLBACK TO SAVEPOINT semantics against the server, not just the SQL we
// emit.
func TestManagerIntegrationSavepointKeepsParentWork(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	client, err := enttenant.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	manager := transaction.NewManager(client)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)

		if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS transaction_guardrails_it"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "CREATE TABLE transaction_guardrails_it (id int PRIMARY KEY)"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO transaction_guardrails_it (id) VALUES (1)"); err != nil {
			return err
		}

		subErr := manager.WithinSavepoint(ctx, func(ctx context.Context) error {
			stx, ok := transaction.TxFromContext(ctx)
			require.True(t, ok)
			if _, err := stx.ExecContext(ctx, "INSERT INTO transaction_guardrails_it (id) VALUES (2)"); err != nil {
				return err
			}
			return errSubtransaction
		})
		if !errors.Is(subErr, errSubtransaction) {
			return subErr
		}
		return nil
	})
	require.NoError(t, err)

	var count int
	err = manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, ok := transaction.TxFromContext(ctx)
		require.True(t, ok)
		rows, err := tx.QueryContext(ctx, "SELECT count(*) FROM transaction_guardrails_it")
		if err != nil {
			return err
		}
		defer rows.Close()
		if !rows.Next() {
			return sql.ErrNoRows
		}
		return rows.Scan(&count)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "the row inserted inside the failed savepoint must be rolled back")

	err = manager.WithinTransaction(ctx, func(ctx context.Context) error {
		tx, _ := transaction.TxFromContext(ctx)
		_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS transaction_guardrails_it")
		return err
	})
	require.NoError(t, err)
}
