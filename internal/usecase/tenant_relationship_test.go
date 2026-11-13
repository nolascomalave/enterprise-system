package usecase

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTenantRelationshipRepo struct {
	active                map[uuid.UUID]bool
	relationships         map[uuid.UUID]*domain.TenantRelationship
	invitesByHash         map[string]*domain.TenantRelationshipInvite
	createRelationshipErr error
	cancelInvitesCalls    int
	markConsumedCalls     int
	wouldCreateCycle      bool
}

func newFakeTenantRelationshipRepo() *fakeTenantRelationshipRepo {
	return &fakeTenantRelationshipRepo{
		active:        make(map[uuid.UUID]bool),
		relationships: make(map[uuid.UUID]*domain.TenantRelationship),
		invitesByHash: make(map[string]*domain.TenantRelationshipInvite),
	}
}

func (r *fakeTenantRelationshipRepo) TenantActive(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	return r.active[tenantID], nil
}

func (r *fakeTenantRelationshipRepo) CreateRelationship(ctx context.Context, relationship *domain.TenantRelationship) error {
	if r.createRelationshipErr != nil {
		return r.createRelationshipErr
	}
	for _, existing := range r.relationships {
		if existing.Status == domain.RelationshipPending &&
			existing.TenantIDHolding == relationship.TenantIDHolding &&
			existing.TenantIDSubsidiary == relationship.TenantIDSubsidiary &&
			existing.RelationshipType == relationship.RelationshipType {
			return domain.ErrRelationshipAlreadyPending
		}
	}
	r.relationships[relationship.ID] = relationship
	return nil
}

func (r *fakeTenantRelationshipRepo) LockRelationship(ctx context.Context, relationshipID uuid.UUID) (*domain.TenantRelationship, error) {
	relationship, ok := r.relationships[relationshipID]
	if !ok {
		return nil, domain.ErrTenantRelationshipNotFound
	}
	return relationship, nil
}

func (r *fakeTenantRelationshipRepo) FindRelationship(ctx context.Context, relationshipID uuid.UUID) (*domain.TenantRelationship, error) {
	return r.LockRelationship(ctx, relationshipID)
}

func (r *fakeTenantRelationshipRepo) WouldCreateCycle(ctx context.Context, holdingID, subsidiaryID uuid.UUID) (bool, error) {
	return r.wouldCreateCycle, nil
}

func (r *fakeTenantRelationshipRepo) UpdateRelationship(ctx context.Context, relationship *domain.TenantRelationship) error {
	r.relationships[relationship.ID] = relationship
	return nil
}

func (r *fakeTenantRelationshipRepo) CreateInvite(ctx context.Context, invite *domain.TenantRelationshipInvite) error {
	r.invitesByHash[string(invite.TokenHash)] = invite
	return nil
}

func (r *fakeTenantRelationshipRepo) FindInviteByTokenHash(ctx context.Context, tokenHash []byte) (*domain.TenantRelationshipInvite, error) {
	invite, ok := r.invitesByHash[string(tokenHash)]
	if !ok {
		return nil, domain.ErrRelationshipInviteNotFound
	}
	return invite, nil
}

func (r *fakeTenantRelationshipRepo) CancelActiveInvites(ctx context.Context, relationshipID, canceledByTenantID uuid.UUID, at time.Time) error {
	r.cancelInvitesCalls++
	for _, invite := range r.invitesByHash {
		if invite.RelationshipID == relationshipID && invite.CanceledAt == nil && invite.ConsumedAt == nil {
			invite.CanceledAt = &at
			invite.CanceledByTenantID = &canceledByTenantID
		}
	}
	return nil
}

func (r *fakeTenantRelationshipRepo) MarkInviteConsumed(ctx context.Context, inviteID uuid.UUID, at time.Time) error {
	r.markConsumedCalls++
	for _, invite := range r.invitesByHash {
		if invite.ID == inviteID {
			invite.ConsumedAt = &at
		}
	}
	return nil
}

func (r *fakeTenantRelationshipRepo) CreateTransfer(ctx context.Context, transfer *domain.TenantTransfer) error {
	return nil
}

func (r *fakeTenantRelationshipRepo) FindTransfer(ctx context.Context, transferID uuid.UUID) (*domain.TenantTransfer, error) {
	return nil, domain.ErrTenantTransferNotFound
}

func tenantRelationshipService(t *testing.T) (*TenantRelationshipService, *fakeTenantRelationshipRepo, *fakeTxManager) {
	t.Helper()
	repo := newFakeTenantRelationshipRepo()
	tx := newFakeTxManager()
	now := func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }
	return NewTenantRelationshipService(repo, tx, now), repo, tx
}

func startPending(t *testing.T, service *TenantRelationshipService, holding, subsidiary uuid.UUID) *domain.TenantRelationship {
	t.Helper()
	relationship, err := service.StartPendingRelationship(context.Background(), StartPendingRelationshipInput{
		HoldingTenantID: holding, SubsidiaryTenantID: subsidiary, RelationshipType: "OWNERSHIP",
	})
	require.NoError(t, err)
	return relationship
}

func TestStartPendingRelationshipRejectsInactiveHolding(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[subsidiary] = true

	_, err := service.StartPendingRelationship(context.Background(), StartPendingRelationshipInput{
		HoldingTenantID: holding, SubsidiaryTenantID: subsidiary, RelationshipType: "OWNERSHIP",
	})

	require.ErrorIs(t, err, domain.ErrTenantInactive)
}

func TestStartPendingRelationshipRejectsInactiveSubsidiary(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true

	_, err := service.StartPendingRelationship(context.Background(), StartPendingRelationshipInput{
		HoldingTenantID: holding, SubsidiaryTenantID: subsidiary, RelationshipType: "OWNERSHIP",
	})

	require.ErrorIs(t, err, domain.ErrTenantInactive)
}

func TestStartPendingRelationshipCreatesOnlyPending(t *testing.T) {
	service, repo, tx := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true

	relationship := startPending(t, service, holding, subsidiary)

	assert.Equal(t, domain.RelationshipPending, relationship.Status)
	assert.Equal(t, 1, tx.calls)
	assert.Empty(t, repo.invitesByHash, "StartPendingRelationship must not emit a token")
}

func TestStartPendingRelationshipRejectsDuplicatePending(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true

	startPending(t, service, holding, subsidiary)

	_, err := service.StartPendingRelationship(context.Background(), StartPendingRelationshipInput{
		HoldingTenantID: holding, SubsidiaryTenantID: subsidiary, RelationshipType: "OWNERSHIP",
	})
	require.ErrorIs(t, err, domain.ErrRelationshipAlreadyPending)
}

func TestEmitInviteTokenCreatesInvite(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)

	result, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.InviteCode)
	require.NotNil(t, result.Invite)
	assert.Equal(t, relationship.ID, result.Invite.RelationshipID)
	assert.Nil(t, result.Invite.CanceledAt)
	assert.Nil(t, result.Invite.ConsumedAt)
	require.Len(t, repo.invitesByHash, 1)
}

func TestEmitInviteTokenRejectsWrongTenant(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)

	_, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: subsidiary,
	})

	require.ErrorIs(t, err, domain.ErrUnauthorizedRelationshipAction)
	assert.Empty(t, repo.invitesByHash)
}

func TestEmitInviteTokenRequiresPending(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	now := time.Date(2026, 11, 13, 13, 0, 0, 0, time.UTC)
	relationship.Status = domain.RelationshipApproved
	relationship.ApprovedAt = &now

	_, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})

	require.ErrorIs(t, err, domain.ErrInvalidRelationshipTransition)
}

func TestCancelPendingRelationshipInvalidatesInvites(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	canceled, err := service.CancelPendingRelationship(context.Background(), CancelPendingRelationshipInput{
		RelationshipID: relationship.ID, SubsidiaryID: subsidiary, Reason: "no longer needed",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.RelationshipCanceled, canceled.Status)
	assert.Equal(t, 1, repo.cancelInvitesCalls)
	for _, invite := range repo.invitesByHash {
		require.NotNil(t, invite.CanceledAt)
		require.NotNil(t, invite.CanceledByTenantID)
		assert.Equal(t, subsidiary, *invite.CanceledByTenantID)
	}

	_, err = service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary})
	require.ErrorIs(t, err, domain.ErrInviteNotActive)
}

func TestCancelPendingRelationshipRejectsWrongTenant(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)

	_, err := service.CancelPendingRelationship(context.Background(), CancelPendingRelationshipInput{
		RelationshipID: relationship.ID, SubsidiaryID: holding,
	})
	require.ErrorIs(t, err, domain.ErrUnauthorizedRelationshipAction)
}

func TestRejectRelationshipInvalidatesInvites(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	_, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	rejected, err := service.RejectRelationship(context.Background(), RejectRelationshipInput{
		RelationshipID: relationship.ID, HoldingID: holding, Reason: "not valid",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.RelationshipRejected, rejected.Status)
	assert.Equal(t, "not valid", rejected.RejectionReason)
	assert.Equal(t, 1, repo.cancelInvitesCalls)
	for _, invite := range repo.invitesByHash {
		require.NotNil(t, invite.CanceledAt)
		require.NotNil(t, invite.CanceledByTenantID)
		assert.Equal(t, holding, *invite.CanceledByTenantID)
	}
}

func TestRejectRelationshipRejectsWrongTenant(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)

	_, err := service.RejectRelationship(context.Background(), RejectRelationshipInput{
		RelationshipID: relationship.ID, HoldingID: subsidiary,
	})
	require.ErrorIs(t, err, domain.ErrUnauthorizedRelationshipAction)
}

func TestConsumeInviteTokenMarksConsumed(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	consumed, err := service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary})

	require.NoError(t, err)
	assert.Equal(t, relationship.ID, consumed.ID)
	assert.Equal(t, 1, repo.markConsumedCalls)
}

func TestConsumeInviteTokenIdempotentWhenApproved(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	now := time.Date(2026, 11, 13, 13, 0, 0, 0, time.UTC)
	relationship.Status = domain.RelationshipApproved
	relationship.ApprovedAt = &now
	repo.relationships[relationship.ID] = relationship
	for _, invite := range repo.invitesByHash {
		invite.ConsumedAt = &now
	}

	approved, err := service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary})
	require.NoError(t, err)
	assert.Equal(t, domain.RelationshipApproved, approved.Status)
}

func TestConsumeInviteTokenRejectsExpired(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)
	for _, invite := range repo.invitesByHash {
		invite.ExpiresAt = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	}

	_, err = service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary})
	require.ErrorIs(t, err, domain.ErrInviteNotActive)
}

func TestConsumeInviteTokenHashMatches(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	hash := sha256.Sum256([]byte(emitted.InviteCode))
	_, ok := repo.invitesByHash[string(hash[:])]
	assert.True(t, ok)
}

var _ domain.TenantRelationshipRepository = (*fakeTenantRelationshipRepo)(nil)

func TestConsumeInviteTokenRejectsCycle(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)
	repo.wouldCreateCycle = true

	_, err = service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{
		InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary,
	})

	require.ErrorIs(t, err, domain.ErrRelationshipCycleDetected)
	assert.Equal(t, domain.RelationshipPending, relationship.Status)
	assert.Equal(t, 0, repo.markConsumedCalls)
}

func TestConsumeInviteTokenRejectsWrongSubsidiary(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)

	_, err = service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{
		InviteCode: emitted.InviteCode, SubsidiaryID: holding,
	})

	require.ErrorIs(t, err, domain.ErrUnauthorizedRelationshipAction)
}

func TestRevokeApprovedRelationship(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)
	approved, err := service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{
		InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary,
	})
	require.NoError(t, err)
	require.Equal(t, domain.RelationshipApproved, approved.Status)

	revoked, err := service.RevokeApprovedRelationship(context.Background(), RevokeApprovedRelationshipInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.RelationshipCanceled, revoked.Status)
	require.NotNil(t, revoked.CanceledAt)
}

func TestRevokeApprovedRelationshipRejectsWrongTenant(t *testing.T) {
	service, repo, _ := tenantRelationshipService(t)
	holding, subsidiary := uuid.New(), uuid.New()
	repo.active[holding] = true
	repo.active[subsidiary] = true
	relationship := startPending(t, service, holding, subsidiary)
	emitted, err := service.EmitInviteToken(context.Background(), EmitInviteTokenInput{
		RelationshipID: relationship.ID, HoldingID: holding,
	})
	require.NoError(t, err)
	_, err = service.ConsumeInviteToken(context.Background(), ConsumeInviteTokenInput{
		InviteCode: emitted.InviteCode, SubsidiaryID: subsidiary,
	})
	require.NoError(t, err)

	_, err = service.RevokeApprovedRelationship(context.Background(), RevokeApprovedRelationshipInput{
		RelationshipID: relationship.ID, HoldingID: subsidiary,
	})

	require.ErrorIs(t, err, domain.ErrUnauthorizedRelationshipAction)
}
