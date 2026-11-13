package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTenantRelationshipUseCase struct {
	relationship *domain.TenantRelationship
	token        string
	err          error

	lastInitiate usecase.StartPendingRelationshipInput
	lastEmit     usecase.EmitInviteTokenInput
	lastConsume  usecase.ConsumeInviteTokenInput
}

func (f *fakeTenantRelationshipUseCase) StartPendingRelationship(ctx context.Context, input usecase.StartPendingRelationshipInput) (*domain.TenantRelationship, error) {
	f.lastInitiate = input
	if f.err != nil {
		return nil, f.err
	}
	f.relationship = &domain.TenantRelationship{
		ID:                 uuid.New(),
		TenantIDHolding:    input.HoldingTenantID,
		TenantIDSubsidiary: input.SubsidiaryTenantID,
		RelationshipType:   input.RelationshipType,
		Status:             domain.RelationshipPending,
		RequestedAt:        time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
	}
	return f.relationship, nil
}

func (f *fakeTenantRelationshipUseCase) EmitInviteToken(ctx context.Context, input usecase.EmitInviteTokenInput) (*usecase.EmitInviteTokenResult, error) {
	f.lastEmit = input
	if f.err != nil {
		return nil, f.err
	}
	f.token = "invite-token-123"
	return &usecase.EmitInviteTokenResult{
		InviteCode: f.token,
		Invite: &domain.TenantRelationshipInvite{
			ID:             uuid.New(),
			RelationshipID: input.RelationshipID,
			ExpiresAt:      time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
		},
	}, nil
}

func (f *fakeTenantRelationshipUseCase) ConsumeInviteToken(ctx context.Context, input usecase.ConsumeInviteTokenInput) (*domain.TenantRelationship, error) {
	f.lastConsume = input
	if f.err != nil {
		return nil, f.err
	}
	if f.relationship == nil || f.relationship.TenantIDSubsidiary != input.SubsidiaryID {
		return nil, domain.ErrUnauthorizedRelationshipAction
	}
	if input.InviteCode != f.token {
		return nil, apperror.NewValidation("invite_not_active", "invite is not active", domain.ErrInviteNotActive)
	}
	f.relationship.Status = domain.RelationshipApproved
	now := time.Date(2026, 11, 13, 12, 1, 0, 0, time.UTC)
	f.relationship.ApprovedAt = &now
	return f.relationship, nil
}

func (f *fakeTenantRelationshipUseCase) RejectRelationship(ctx context.Context, input usecase.RejectRelationshipInput) (*domain.TenantRelationship, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.relationship.Status = domain.RelationshipRejected
	return f.relationship, nil
}

func (f *fakeTenantRelationshipUseCase) CancelPendingRelationship(ctx context.Context, input usecase.CancelPendingRelationshipInput) (*domain.TenantRelationship, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.relationship.Status = domain.RelationshipCanceled
	return f.relationship, nil
}

func (f *fakeTenantRelationshipUseCase) RevokeApprovedRelationship(ctx context.Context, input usecase.RevokeApprovedRelationshipInput) (*domain.TenantRelationship, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.relationship.Status = domain.RelationshipCanceled
	return f.relationship, nil
}

func relationshipRouter(t *testing.T, tr usecase.TenantRelationshipUseCase) (*gin.Engine, *auth.Manager) {
	t.Helper()
	manager := testAuthManager(t)
	rbac := &fakeRBACUseCase{universe: &domain.PermissionUniverse{Functions: []string{"org:write"}}}
	router := NewRouter(Dependencies{
		TokenManager:              manager,
		RBACUseCase:               rbac,
		TenantRelationshipUseCase: tr,
	})
	return router, manager
}

func TestTenantRelationshipFullFlow(t *testing.T) {
	tr := &fakeTenantRelationshipUseCase{}
	router, manager := relationshipRouter(t, tr)
	holding, subsidiary := uuid.New(), uuid.New()
	userID, sessionID := uuid.New(), uuid.New()
	subsidiaryToken := accessToken(t, manager, subsidiary, userID, sessionID)
	holdingToken := accessToken(t, manager, holding, userID, sessionID)
	appClientID := uuid.New().String()

	initiate := requestWithTokenAndApp(router, http.MethodPost,
		"/api/v1/tenant-relationships/initiate",
		[]byte(`{"holding_tenant_id":"`+holding.String()+`","relationship_type":"OWNERSHIP"}`),
		subsidiaryToken, appClientID)
	require.Equal(t, http.StatusCreated, initiate.Code, initiate.Body.String())
	assert.Equal(t, subsidiary, tr.lastInitiate.SubsidiaryTenantID)
	assert.Equal(t, holding, tr.lastInitiate.HoldingTenantID)

	body := decodeBody(t, initiate)
	data := body["data"].(map[string]any)
	relationshipID := data["id"].(string)
	assert.Equal(t, "PENDING", data["status"])

	emit := requestWithTokenAndApp(router, http.MethodPost,
		"/api/v1/tenant-relationships/"+relationshipID+"/emit-token", nil, holdingToken, appClientID)
	require.Equal(t, http.StatusOK, emit.Code, emit.Body.String())
	assert.Equal(t, holding, tr.lastEmit.HoldingID)
	emitBody := decodeBody(t, emit)
	inviteCode := emitBody["data"].(map[string]any)["invite_code"].(string)
	assert.NotEmpty(t, inviteCode)

	consume := requestWithTokenAndApp(router, http.MethodPost,
		"/api/v1/tenant-relationships/consume-token",
		[]byte(`{"invite_code":"`+inviteCode+`"}`), subsidiaryToken, appClientID)
	require.Equal(t, http.StatusOK, consume.Code, consume.Body.String())
	assert.Equal(t, subsidiary, tr.lastConsume.SubsidiaryID)
	consumeBody := decodeBody(t, consume)
	approved := consumeBody["data"].(map[string]any)
	assert.Equal(t, "APPROVED", approved["status"])
}

func TestTenantRelationshipOutsiderGetsForbidden(t *testing.T) {
	tr := &fakeTenantRelationshipUseCase{
		relationship: &domain.TenantRelationship{
			ID:                 uuid.New(),
			TenantIDHolding:    uuid.New(),
			TenantIDSubsidiary: uuid.New(),
			Status:             domain.RelationshipPending,
		},
	}
	router, manager := relationshipRouter(t, tr)
	outsider := uuid.New()
	token := accessToken(t, manager, outsider, uuid.New(), uuid.New())

	recorder := requestWithTokenAndApp(router, http.MethodPost,
		"/api/v1/tenant-relationships/consume-token",
		[]byte(`{"invite_code":"whatever"}`), token, uuid.New().String())

	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	body := decodeBody(t, recorder)
	assert.Equal(t, "forbidden", body["code"])
}

func TestTenantRelationshipInvalidTokenReturnsBadRequest(t *testing.T) {
	subsidiary := uuid.New()
	tr := &fakeTenantRelationshipUseCase{
		relationship: &domain.TenantRelationship{
			ID:                 uuid.New(),
			TenantIDHolding:    uuid.New(),
			TenantIDSubsidiary: subsidiary,
			Status:             domain.RelationshipPending,
		},
		token: "expected-token",
	}
	router, manager := relationshipRouter(t, tr)
	token := accessToken(t, manager, subsidiary, uuid.New(), uuid.New())

	recorder := requestWithTokenAndApp(router, http.MethodPost,
		"/api/v1/tenant-relationships/consume-token",
		[]byte(`{"invite_code":"wrong-token"}`), token, uuid.New().String())

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	body := decodeBody(t, recorder)
	assert.Equal(t, "invite_not_active", body["code"])
}
