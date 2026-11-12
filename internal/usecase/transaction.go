package usecase

import (
	"context"
	"errors"
)

// TransactionManager is the application port for transaction boundaries. It
// is implemented by internal/infrastructure/transaction.Manager.
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(context.Context) error) error
	WithinSavepoint(ctx context.Context, fn func(context.Context) error) error
}

// readWithinTransaction runs a read inside a transaction. PostgreSQL applies
// the tenant session context with SET LOCAL, so RLS-protected reads need an
// open transaction just like writes do.
func readWithinTransaction[T any](
	ctx context.Context,
	tx TransactionManager,
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T
	if tx == nil {
		return zero, errors.New("usecase: nil transaction manager")
	}
	var result T
	err := tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		result, err = fn(ctx)
		return err
	})
	if err != nil {
		return zero, err
	}
	return result, nil
}
