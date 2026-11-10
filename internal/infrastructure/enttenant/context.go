package enttenant

import (
	"context"

	"github.com/google/uuid"
)

// SessionContext carries the values that must be published to PostgreSQL with
// SET LOCAL at the beginning of every transaction. They feed RLS and the audit
// trigger.
type SessionContext struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	SessionID uuid.UUID
}

type sessionContextKey struct{}

func WithSessionContext(ctx context.Context, sc SessionContext) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, sc)
}

func SessionContextFrom(ctx context.Context) (SessionContext, bool) {
	sc, ok := ctx.Value(sessionContextKey{}).(SessionContext)
	return sc, ok
}

func (sc SessionContext) IsZero() bool {
	return sc.TenantID == uuid.Nil && sc.UserID == uuid.Nil && sc.SessionID == uuid.Nil
}
