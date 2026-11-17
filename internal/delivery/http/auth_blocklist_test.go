package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type auditSessionBlocklist struct {
	blocked uuid.UUID
}

func (b auditSessionBlocklist) Block(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error {
	return nil
}

func (b auditSessionBlocklist) IsBlocked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	return sessionID == b.blocked, nil
}

func TestTenantMiddlewareRejectsBlockedSession(t *testing.T) {
	manager := testAuthManager(t)
	tenantID, userID, sessionID := uuid.New(), uuid.New(), uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)

	router := NewRouter(Dependencies{
		TokenManager:     manager,
		SessionBlocklist: auditSessionBlocklist{blocked: sessionID},
	})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/platform/status", nil, token)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	body := decodeBody(t, recorder)
	require.Equal(t, "session_revoked", body["code"])
}

func TestTenantMiddlewareAllowsUnblockedSession(t *testing.T) {
	manager := testAuthManager(t)
	tenantID, userID, sessionID := uuid.New(), uuid.New(), uuid.New()
	token := accessToken(t, manager, tenantID, userID, sessionID)

	router := NewRouter(Dependencies{
		TokenManager:     manager,
		SessionBlocklist: auditSessionBlocklist{blocked: uuid.New()},
	})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/platform/status", nil, token)
	require.Equal(t, http.StatusOK, recorder.Code)
}
