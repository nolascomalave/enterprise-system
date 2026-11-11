package transaction

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"enterprise-system/ent"
)

// ErrNoActiveTransaction is returned when a savepoint is requested outside a
// transaction started by Manager.WithinTransaction.
var ErrNoActiveTransaction = errors.New("transaction: no active transaction")

// Manager is the single entry point for opening transactions. Repositories
// must never call client.Tx directly; they obtain the ambient client with
// ClientFromContext.
type Manager struct {
	client *ent.Client
	seq    atomic.Uint64
}

func NewManager(client *ent.Client) *Manager {
	return &Manager{client: client}
}

// WithinTransaction runs fn inside a transaction. If the context already
// carries an ambient transaction, fn joins it instead of opening a second
// one. The transaction is committed when fn returns nil and rolled back
// otherwise. A panic also rolls the transaction back before propagating.
func (m *Manager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if m == nil || m.client == nil {
		return errors.New("transaction: nil ent client")
	}
	if fn == nil {
		return errors.New("transaction: nil callback")
	}
	if _, ok := TxFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := m.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("transaction: begin: %w", err)
	}

	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback()
		}
	}()

	txCtx := withTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		finished = true
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("transaction: rollback: %w", rollbackErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		finished = true
		_ = tx.Rollback()
		return fmt.Errorf("transaction: commit: %w", err)
	}
	finished = true
	return nil
}

// WithinSavepoint opens a savepoint inside the ambient transaction. If fn
// fails, only the work performed after the savepoint is rolled back; the
// parent transaction stays alive and can still commit.
func (m *Manager) WithinSavepoint(ctx context.Context, fn func(context.Context) error) error {
	if m == nil {
		return errors.New("transaction: nil manager")
	}
	if fn == nil {
		return errors.New("transaction: nil callback")
	}
	tx, ok := TxFromContext(ctx)
	if !ok {
		return ErrNoActiveTransaction
	}

	name := fmt.Sprintf("sp_%d", m.seq.Add(1))
	if err := execRaw(ctx, tx, "SAVEPOINT "+name); err != nil {
		return err
	}

	finished := false
	defer func() {
		if !finished {
			// A panic must not leave the savepoint open; the parent
			// transaction is still rolled back by its own guard.
			rollbackCtx := context.WithoutCancel(ctx)
			_ = rollbackSavepoint(rollbackCtx, tx, name)
		}
	}()

	if err := fn(ctx); err != nil {
		// Rollback must happen even if the caller's context was cancelled.
		rollbackCtx := context.WithoutCancel(ctx)
		if rollbackErr := rollbackSavepoint(rollbackCtx, tx, name); rollbackErr != nil {
			finished = true
			return errors.Join(err, rollbackErr)
		}
		finished = true
		return err
	}

	if err := execRaw(ctx, tx, "RELEASE SAVEPOINT "+name); err != nil {
		return err
	}
	finished = true
	return nil
}

// TxFromContext returns the ambient transaction, if any. It relies on the
// context key owned by Ent so hooks and integrations observe the same
// transaction.
func TxFromContext(ctx context.Context) (*ent.Tx, bool) {
	tx := ent.TxFromContext(ctx)
	return tx, tx != nil
}

// ClientFromContext returns a client bound to the ambient transaction or the
// provided fallback when no transaction is active. Repositories must use this
// helper instead of opening their own transactions.
func ClientFromContext(ctx context.Context, fallback *ent.Client) *ent.Client {
	if tx, ok := TxFromContext(ctx); ok {
		return tx.Client()
	}
	return fallback
}

func withTx(ctx context.Context, tx *ent.Tx) context.Context {
	return ent.NewTxContext(ctx, tx)
}

func rollbackSavepoint(ctx context.Context, tx *ent.Tx, name string) error {
	if err := execRaw(ctx, tx, "ROLLBACK TO SAVEPOINT "+name); err != nil {
		return err
	}
	return execRaw(ctx, tx, "RELEASE SAVEPOINT "+name)
}

func execRaw(ctx context.Context, tx *ent.Tx, query string) error {
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("transaction: %s: %w", query, err)
	}
	return nil
}
