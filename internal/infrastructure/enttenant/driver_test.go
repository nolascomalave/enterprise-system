package enttenant

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type execCall struct {
	query string
	args  any
	v     any
}

type fakeTx struct {
	calls        []execCall
	execErr      error
	queryCalls   int
	commitCalled bool
	rollback     bool
}

func (t *fakeTx) Exec(ctx context.Context, query string, args, v any) error {
	t.calls = append(t.calls, execCall{query: query, args: args, v: v})
	return t.execErr
}

func (t *fakeTx) Query(ctx context.Context, query string, args, v any) error {
	t.queryCalls++
	return nil
}

func (t *fakeTx) Commit() error {
	t.commitCalled = true
	return nil
}

func (t *fakeTx) Rollback() error {
	t.rollback = true
	return nil
}

type fakeDriver struct {
	tx        *fakeTx
	txErr     error
	beginTx   *fakeTx
	beginErr  error
	beginOpts *sql.TxOptions
}

func (d *fakeDriver) Exec(ctx context.Context, query string, args, v any) error {
	return nil
}

func (d *fakeDriver) Query(ctx context.Context, query string, args, v any) error {
	return nil
}

func (d *fakeDriver) Tx(ctx context.Context) (dialect.Tx, error) {
	if d.txErr != nil {
		return nil, d.txErr
	}
	return d.tx, nil
}

func (d *fakeDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	d.beginOpts = opts
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	return d.beginTx, nil
}

func (d *fakeDriver) Close() error { return nil }

func (d *fakeDriver) Dialect() string { return "postgres" }

func testSessionContext() SessionContext {
	return SessionContext{
		TenantID:  uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001"),
		UserID:    uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002"),
		SessionID: uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000003"),
	}
}

func TestDriverTxInjectsSessionContext(t *testing.T) {
	sc := testSessionContext()
	ftx := &fakeTx{}
	drv := NewDriver(&fakeDriver{tx: ftx})

	tx, err := drv.Tx(WithSessionContext(context.Background(), sc))

	require.NoError(t, err)
	require.Same(t, ftx, tx)
	require.Len(t, ftx.calls, 1)

	call := ftx.calls[0]
	assert.Contains(t, call.query, "set_config('app.current_tenant_id', $1, true)")
	assert.Contains(t, call.query, "set_config('app.current_user_id', $2, true)")
	assert.Contains(t, call.query, "set_config('app.current_session_id', $3, true)")
	assert.Equal(t, []any{sc.TenantID.String(), sc.UserID.String(), sc.SessionID.String()}, call.args)
	assert.Nil(t, call.v)
	assert.False(t, ftx.rollback)
}

func TestDriverTxWithoutSessionContextDoesNotExec(t *testing.T) {
	ftx := &fakeTx{}
	drv := NewDriver(&fakeDriver{tx: ftx})

	tx, err := drv.Tx(context.Background())

	require.NoError(t, err)
	require.Same(t, ftx, tx)
	assert.Empty(t, ftx.calls)
}

func TestDriverTxSkipsZeroValues(t *testing.T) {
	sc := SessionContext{TenantID: testSessionContext().TenantID}
	ftx := &fakeTx{}
	drv := NewDriver(&fakeDriver{tx: ftx})

	_, err := drv.Tx(WithSessionContext(context.Background(), sc))

	require.NoError(t, err)
	require.Len(t, ftx.calls, 1)
	assert.Contains(t, ftx.calls[0].query, "set_config('app.current_tenant_id', $1, true)")
	assert.NotContains(t, ftx.calls[0].query, "app.current_user_id")
	assert.Equal(t, []any{sc.TenantID.String()}, ftx.calls[0].args)
}

func TestDriverTxRollsBackWhenContextInjectionFails(t *testing.T) {
	ftx := &fakeTx{execErr: errors.New("boom")}
	drv := NewDriver(&fakeDriver{tx: ftx})

	tx, err := drv.Tx(WithSessionContext(context.Background(), testSessionContext()))

	require.Error(t, err)
	assert.Nil(t, tx)
	assert.True(t, ftx.rollback)
	assert.Contains(t, err.Error(), "set session context")
}

func TestDriverTxReturnsUnderlyingError(t *testing.T) {
	drv := NewDriver(&fakeDriver{txErr: errors.New("no tx")})

	tx, err := drv.Tx(context.Background())

	require.Error(t, err)
	assert.Nil(t, tx)
	assert.Contains(t, err.Error(), "begin transaction")
}

func TestDriverBeginTxInjectsSessionContext(t *testing.T) {
	ftx := &fakeTx{}
	fdrv := &fakeDriver{beginTx: ftx}
	drv := NewDriver(fdrv)
	opts := &sql.TxOptions{Isolation: sql.LevelSerializable}

	tx, err := drv.BeginTx(WithSessionContext(context.Background(), testSessionContext()), opts)

	require.NoError(t, err)
	require.Same(t, ftx, tx)
	assert.Same(t, opts, fdrv.beginOpts)
	require.Len(t, ftx.calls, 1)
	assert.Contains(t, ftx.calls[0].query, "app.current_tenant_id")
}

func TestNewClientUsesWrappedDriver(t *testing.T) {
	ftx := &fakeTx{}
	client := NewClient(&fakeDriver{tx: ftx})
	t.Cleanup(func() { _ = client.Close() })

	tx, err := client.Tx(WithSessionContext(context.Background(), testSessionContext()))

	require.NoError(t, err)
	require.NotNil(t, tx)
	require.Len(t, ftx.calls, 1)
	assert.Contains(t, ftx.calls[0].query, "app.current_tenant_id")
}
