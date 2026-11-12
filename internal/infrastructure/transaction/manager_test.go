package transaction_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"enterprise-system/ent"
	"enterprise-system/internal/infrastructure/transaction"
	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSubtransaction = errors.New("subtransaction failed")

// recordingTx is a dialect.Tx that remembers raw SQL and whether the outer
// transaction was committed or rolled back.
type recordingTx struct {
	queries  []string
	commit   bool
	rollback bool
}

func (t *recordingTx) Exec(ctx context.Context, query string, args, v any) error {
	t.queries = append(t.queries, query)
	return nil
}

func (t *recordingTx) Query(ctx context.Context, query string, args, v any) error {
	return nil
}

func (t *recordingTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	t.queries = append(t.queries, query)
	return driverResult{}, nil
}

func (t *recordingTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (t *recordingTx) Commit() error {
	t.commit = true
	return nil
}

func (t *recordingTx) Rollback() error {
	t.rollback = true
	return nil
}

type driverResult struct{}

func (driverResult) LastInsertId() (int64, error) { return 0, nil }
func (driverResult) RowsAffected() (int64, error) { return 0, nil }

type recordingDriver struct {
	tx       *recordingTx
	txErr    error
	txBegins int
}

func (d *recordingDriver) Exec(ctx context.Context, query string, args, v any) error { return nil }
func (d *recordingDriver) Query(ctx context.Context, query string, args, v any) error {
	return nil
}
func (d *recordingDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	d.txBegins++
	if d.txErr != nil {
		return nil, d.txErr
	}
	return d.tx, nil
}
func (d *recordingDriver) Close() error    { return nil }
func (d *recordingDriver) Dialect() string { return dialect.Postgres }

func newTestManager(t *testing.T) (*transaction.Manager, *recordingDriver) {
	t.Helper()
	drv := &recordingDriver{tx: &recordingTx{}}
	client := ent.NewClient(ent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })
	return transaction.NewManager(client), drv
}

func TestWithinTransactionCommitsOnSuccess(t *testing.T) {
	manager, drv := newTestManager(t)
	ran := false

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		ran = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, drv.tx.commit)
	assert.False(t, drv.tx.rollback)
}

func TestWithinTransactionRollsBackOnError(t *testing.T) {
	manager, drv := newTestManager(t)
	sentinel := errors.New("boom")

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
	assert.True(t, drv.tx.rollback)
	assert.False(t, drv.tx.commit)
}

func TestWithinTransactionJoinsAmbientTransaction(t *testing.T) {
	manager, drv := newTestManager(t)

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return manager.WithinTransaction(ctx, func(context.Context) error { return nil })
	})

	require.NoError(t, err)
	assert.Equal(t, 1, drv.txBegins, "nested WithinTransaction must not open a second transaction")
	assert.True(t, drv.tx.commit)
	assert.False(t, drv.tx.rollback)
}

func TestWithinSavepointRequiresAmbientTransaction(t *testing.T) {
	manager, _ := newTestManager(t)

	err := manager.WithinSavepoint(context.Background(), func(context.Context) error { return nil })

	require.ErrorIs(t, err, transaction.ErrNoActiveTransaction)
}

func TestWithinSavepointReleasesOnSuccess(t *testing.T) {
	manager, drv := newTestManager(t)

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return manager.WithinSavepoint(ctx, func(context.Context) error { return nil })
	})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"SAVEPOINT sp_1",
		"RELEASE SAVEPOINT sp_1",
	}, drv.tx.queries)
	assert.True(t, drv.tx.commit)
}

func TestWithinSavepointRollsBackOnlySubtransaction(t *testing.T) {
	manager, drv := newTestManager(t)
	afterSubtransaction := false

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		subErr := manager.WithinSavepoint(ctx, func(context.Context) error {
			return errSubtransaction
		})
		require.ErrorIs(t, subErr, errSubtransaction)
		afterSubtransaction = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, afterSubtransaction, "parent transaction must continue after a rolled-back savepoint")
	assert.Equal(t, []string{
		"SAVEPOINT sp_1",
		"ROLLBACK TO SAVEPOINT sp_1",
		"RELEASE SAVEPOINT sp_1",
	}, drv.tx.queries)
	assert.True(t, drv.tx.commit, "parent transaction must still commit")
	assert.False(t, drv.tx.rollback, "parent transaction must not roll back")
}

func TestWithinSavepointNestedRollsBackOnlyInner(t *testing.T) {
	manager, drv := newTestManager(t)

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		return manager.WithinSavepoint(ctx, func(ctx context.Context) error {
			innerErr := manager.WithinSavepoint(ctx, func(context.Context) error {
				return errSubtransaction
			})
			require.ErrorIs(t, innerErr, errSubtransaction)
			return nil
		})
	})

	require.NoError(t, err)
	assert.Equal(t, []string{
		"SAVEPOINT sp_1",
		"SAVEPOINT sp_2",
		"ROLLBACK TO SAVEPOINT sp_2",
		"RELEASE SAVEPOINT sp_2",
		"RELEASE SAVEPOINT sp_1",
	}, drv.tx.queries)
	assert.True(t, drv.tx.commit)
	assert.False(t, drv.tx.rollback)
}

func TestWithinTransactionRollsBackOnPanic(t *testing.T) {
	manager, drv := newTestManager(t)

	assert.Panics(t, func() {
		_ = manager.WithinTransaction(context.Background(), func(context.Context) error {
			panic("boom")
		})
	})

	assert.True(t, drv.tx.rollback, "panic must roll back the transaction")
	assert.False(t, drv.tx.commit)
}

func TestWithinSavepointRollsBackOnPanic(t *testing.T) {
	manager, drv := newTestManager(t)

	assert.Panics(t, func() {
		_ = manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
			return manager.WithinSavepoint(ctx, func(context.Context) error {
				panic("boom")
			})
		})
	})

	assert.Equal(t, []string{
		"SAVEPOINT sp_1",
		"ROLLBACK TO SAVEPOINT sp_1",
		"RELEASE SAVEPOINT sp_1",
	}, drv.tx.queries)
	assert.True(t, drv.tx.rollback, "eventually the parent transaction is rolled back")
	assert.False(t, drv.tx.commit)
}

func TestWithinTransactionPublishesEntTxContext(t *testing.T) {
	manager, _ := newTestManager(t)

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NotNil(t, ent.TxFromContext(ctx), "Ent hooks must be able to see the ambient transaction")
		return nil
	})

	require.NoError(t, err)
}

func TestClientFromContextReturnsTransactionClient(t *testing.T) {
	manager, _ := newTestManager(t)

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		require.NotNil(t, transaction.ClientFromContext(ctx, nil))
		return nil
	})

	require.NoError(t, err)
}
