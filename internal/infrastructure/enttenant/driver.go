package enttenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"enterprise-system/ent"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

// Driver decorates an Ent dialect.Driver and publishes the SessionContext to
// PostgreSQL with SET LOCAL at the start of every transaction. SET LOCAL keeps
// the values scoped to the transaction, so pooled connections cannot leak a
// tenant or user into the next request.
type Driver struct {
	dialect.Driver
}

func NewDriver(driver dialect.Driver) *Driver {
	return &Driver{Driver: driver}
}

func (d *Driver) Tx(ctx context.Context) (dialect.Tx, error) {
	if d == nil || d.Driver == nil {
		return nil, errors.New("enttenant: nil driver")
	}

	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("enttenant: begin transaction: %w", err)
	}
	return d.prepare(ctx, tx)
}

func (d *Driver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	if d == nil || d.Driver == nil {
		return nil, errors.New("enttenant: nil driver")
	}

	beginner, ok := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	})
	if !ok {
		return nil, errors.New("enttenant: underlying driver does not support BeginTx")
	}

	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("enttenant: begin transaction: %w", err)
	}
	return d.prepare(ctx, tx)
}

func (d *Driver) prepare(ctx context.Context, tx dialect.Tx) (dialect.Tx, error) {
	if err := applySessionContext(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, errors.Join(err, fmt.Errorf("enttenant: rollback after context failure: %w", rollbackErr))
		}
		return nil, err
	}
	return tx, nil
}

func applySessionContext(ctx context.Context, tx dialect.Tx) error {
	sc, ok := SessionContextFrom(ctx)
	if !ok || sc.IsZero() {
		return nil
	}

	var (
		settings []string
		args     []any
	)
	add := func(setting string, id uuid.UUID) {
		if id == uuid.Nil {
			return
		}
		args = append(args, id.String())
		settings = append(settings, fmt.Sprintf("set_config('%s', $%d, true)", setting, len(args)))
	}

	add("app.current_tenant_id", sc.TenantID)
	add("app.current_user_id", sc.UserID)
	add("app.current_session_id", sc.SessionID)
	if len(settings) == 0 {
		return nil
	}

	query := "SELECT " + strings.Join(settings, ", ")
	if err := tx.Exec(ctx, query, args, nil); err != nil {
		return fmt.Errorf("enttenant: set session context: %w", err)
	}
	return nil
}

// NewClient builds an Ent client whose driver publishes SessionContext before
// every transaction.
func NewClient(driver dialect.Driver, opts ...ent.Option) *ent.Client {
	opts = append(opts, ent.Driver(NewDriver(driver)))
	return ent.NewClient(opts...)
}

// Open opens a SQL driver and wraps it with the tenant/session context driver.
func Open(driverName, dsn string, opts ...ent.Option) (*ent.Client, error) {
	driver, err := entsql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("enttenant: open database: %w", err)
	}
	return NewClient(driver, opts...), nil
}
