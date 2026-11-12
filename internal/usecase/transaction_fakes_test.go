package usecase

import "context"

// fakeTxManager is a pass-through TransactionManager for use case tests.
type fakeTxManager struct {
	calls        int
	savepoints   int
	err          error
	savepointErr error
}

func newFakeTxManager() *fakeTxManager {
	return &fakeTxManager{}
}

func (m *fakeTxManager) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	m.calls++
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

func (m *fakeTxManager) WithinSavepoint(ctx context.Context, fn func(context.Context) error) error {
	m.savepoints++
	if m.savepointErr != nil {
		return m.savepointErr
	}
	return fn(ctx)
}
