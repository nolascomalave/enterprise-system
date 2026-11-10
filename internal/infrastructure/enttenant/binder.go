package enttenant

import (
	"context"

	"github.com/google/uuid"
)

// ContextBinder is the adapter used by usecases that need to establish the
// tenant/session context before talking to an RLS-protected repository.
type ContextBinder struct{}

func (ContextBinder) BindTenant(ctx context.Context, tenantID, userID, sessionID uuid.UUID) context.Context {
	return WithSessionContext(ctx, SessionContext{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionID,
	})
}
