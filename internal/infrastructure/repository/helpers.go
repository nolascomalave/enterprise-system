package repository

import (
	"context"
	"errors"

	"enterprise-system/ent"
	"enterprise-system/internal/infrastructure/transaction"
)

var errNilClient = errors.New("repository: nil ent client")

// clientFromContext returns the ambient transaction client when the use case
// opened one through TransactionManager. The root client is only a fallback
// for non-RLS/global access.
func clientFromContext(ctx context.Context, client *ent.Client) (*ent.Client, error) {
	if client == nil {
		return nil, errNilClient
	}
	return transaction.ClientFromContext(ctx, client), nil
}

// execute runs fn with the ambient client. It never opens a transaction:
// use cases are responsible for wrapping reads and writes in
// TransactionManager.WithinTransaction so RLS receives the tenant context.
func execute(ctx context.Context, client *ent.Client, fn func(*ent.Client) error) error {
	execClient, err := clientFromContext(ctx, client)
	if err != nil {
		return repoError(err)
	}
	return fn(execClient)
}
