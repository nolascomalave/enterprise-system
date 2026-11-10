package domain

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePartyMergeRepository struct {
	parties   map[uuid.UUID]*Party
	updates   []*Party
	logs      []*PartyMergeLog
	getErr    map[uuid.UUID]error
	findErr   error
	updateErr error
	logErr    error
}

func newFakePartyMergeRepository(parties ...*Party) *fakePartyMergeRepository {
	repo := &fakePartyMergeRepository{
		parties: make(map[uuid.UUID]*Party, len(parties)),
		getErr:  make(map[uuid.UUID]error),
	}
	for _, p := range parties {
		repo.parties[p.ID] = p
	}
	return repo
}

func (r *fakePartyMergeRepository) GetParty(ctx context.Context, partyID uuid.UUID) (*Party, error) {
	if err := r.getErr[partyID]; err != nil {
		return nil, err
	}
	p, ok := r.parties[partyID]
	if !ok {
		return nil, errors.New("party not found")
	}
	return p, nil
}

func (r *fakePartyMergeRepository) FindPartiesByCanonicalID(ctx context.Context, canonicalPartyID uuid.UUID) ([]*Party, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	var result []*Party
	for _, p := range r.parties {
		if p.CanonicalPartyID != nil && *p.CanonicalPartyID == canonicalPartyID {
			result = append(result, p)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID.String() < result[j].ID.String()
	})
	return result, nil
}

func (r *fakePartyMergeRepository) UpdateParty(ctx context.Context, party *Party) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.parties[party.ID] = party
	r.updates = append(r.updates, party)
	return nil
}

func (r *fakePartyMergeRepository) CreatePartyMergeLog(ctx context.Context, entry *PartyMergeLog) error {
	if r.logErr != nil {
		return r.logErr
	}
	r.logs = append(r.logs, entry)
	return nil
}

func activeTestParty(t *testing.T, tenantID uuid.UUID, name string) *Party {
	t.Helper()

	p, err := NewParty(tenantID, PartyTypeIndividual, name, testNow())
	require.NoError(t, err)
	return p
}

func mergedTestParty(t *testing.T, tenantID, canonicalID uuid.UUID, name string) *Party {
	t.Helper()

	p := activeTestParty(t, tenantID, name)
	require.NoError(t, p.MergeInto(canonicalID, testNow()))
	return p
}

func TestMergePartiesCompressesPathAndLogs(t *testing.T) {
	source := activeTestParty(t, testTenantID, "Source")
	target := activeTestParty(t, testTenantID, "Target")
	dependent1 := mergedTestParty(t, testTenantID, source.ID, "Dependent 1")
	dependent2 := mergedTestParty(t, testTenantID, source.ID, "Dependent 2")

	repo := newFakePartyMergeRepository(source, target, dependent1, dependent2)
	actor := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-0000000000a1")
	service := NewPartyMergeService(repo, func() time.Time { return testNow() }, &actor)

	result, err := service.MergeParties(context.Background(), source.ID, target.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, source.ID, result.SourceID)
	assert.Equal(t, target.ID, result.TargetID)
	assert.ElementsMatch(t, []uuid.UUID{dependent1.ID, dependent2.ID}, result.CompressedPartyIDs)
	assert.Equal(t, testNow(), result.MergedAt)

	assert.Equal(t, PartyRecordStatusMerged, source.RecordStatus)
	require.NotNil(t, source.CanonicalPartyID)
	assert.Equal(t, target.ID, *source.CanonicalPartyID)
	require.NotNil(t, source.DeletedAt)
	assert.Equal(t, testNow(), *source.DeletedAt)

	for _, dependent := range []*Party{dependent1, dependent2} {
		require.NotNil(t, dependent.CanonicalPartyID)
		assert.Equal(t, target.ID, *dependent.CanonicalPartyID, "path compression debe apuntar al target final")
		assert.Equal(t, PartyRecordStatusMerged, dependent.RecordStatus)
	}

	require.Len(t, repo.logs, 1)
	logEntry := repo.logs[0]
	assert.Equal(t, testTenantID, logEntry.TenantID)
	assert.Equal(t, source.ID, logEntry.SourcePartyID)
	assert.Equal(t, target.ID, logEntry.TargetPartyID)
	assert.Equal(t, testNow(), logEntry.MergedAt)
	require.NotNil(t, logEntry.MergedBy)
	assert.Equal(t, actor, *logEntry.MergedBy)
	assert.Len(t, repo.updates, 3)
}

func TestMergePartiesRejectsInvalidEndpoints(t *testing.T) {
	otherTenant := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000009")
	source := activeTestParty(t, testTenantID, "Source")
	target := activeTestParty(t, testTenantID, "Target")
	foreignTarget := activeTestParty(t, otherTenant, "Foreign")
	mergedTarget := mergedTestParty(t, testTenantID, target.ID, "Already merged")
	deletedSource := activeTestParty(t, testTenantID, "Deleted")
	require.NoError(t, deletedSource.SoftDelete(testNow()))

	tests := []struct {
		name    string
		source  *Party
		target  *Party
		wantErr error
	}{
		{name: "different tenants", source: source, target: foreignTarget, wantErr: ErrPartyTenantMismatch},
		{name: "source merged", source: mergedTestParty(t, testTenantID, target.ID, "Merged source"), target: target, wantErr: ErrPartyNotMergeable},
		{name: "source deleted", source: deletedSource, target: target, wantErr: ErrPartyNotMergeable},
		{name: "target merged", source: source, target: mergedTarget, wantErr: ErrPartyMergeTargetNotActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakePartyMergeRepository(tt.source, tt.target)
			service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

			result, err := service.MergeParties(context.Background(), tt.source.ID, tt.target.ID)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, result)
			assert.Empty(t, repo.updates)
			assert.Empty(t, repo.logs)
		})
	}
}

func TestMergePartiesRejectsInvalidIDs(t *testing.T) {
	service := NewPartyMergeService(newFakePartyMergeRepository(), func() time.Time { return testNow() }, nil)

	_, err := service.MergeParties(context.Background(), uuid.Nil, uuid.New())
	require.ErrorIs(t, err, ErrInvalidPartyMerge)

	_, err = service.MergeParties(context.Background(), uuid.New(), uuid.Nil)
	require.ErrorIs(t, err, ErrInvalidPartyMerge)

	id := uuid.New()
	_, err = service.MergeParties(context.Background(), id, id)
	require.ErrorIs(t, err, ErrInvalidPartyMerge)
}

func TestMergePartiesPropagatesRepositoryErrors(t *testing.T) {
	t.Run("get source", func(t *testing.T) {
		source := activeTestParty(t, testTenantID, "Source")
		target := activeTestParty(t, testTenantID, "Target")
		repo := newFakePartyMergeRepository(source, target)
		repo.getErr[source.ID] = errors.New("db down")
		service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

		_, err := service.MergeParties(context.Background(), source.ID, target.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get source party")
	})

	t.Run("find dependents", func(t *testing.T) {
		source := activeTestParty(t, testTenantID, "Source")
		target := activeTestParty(t, testTenantID, "Target")
		repo := newFakePartyMergeRepository(source, target)
		repo.findErr = errors.New("find failed")
		service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

		_, err := service.MergeParties(context.Background(), source.ID, target.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "find parties pointing to")
	})

	t.Run("update source", func(t *testing.T) {
		source := activeTestParty(t, testTenantID, "Source")
		target := activeTestParty(t, testTenantID, "Target")
		repo := newFakePartyMergeRepository(source, target)
		repo.updateErr = errors.New("update failed")
		service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

		_, err := service.MergeParties(context.Background(), source.ID, target.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "update merged source party")
	})

	t.Run("create log", func(t *testing.T) {
		source := activeTestParty(t, testTenantID, "Source")
		target := activeTestParty(t, testTenantID, "Target")
		repo := newFakePartyMergeRepository(source, target)
		repo.logErr = errors.New("log failed")
		service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

		_, err := service.MergeParties(context.Background(), source.ID, target.ID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "create party merge log")
	})
}

func TestMergePartiesRejectsNonMergedDependent(t *testing.T) {
	source := activeTestParty(t, testTenantID, "Source")
	target := activeTestParty(t, testTenantID, "Target")
	invalidDependent := activeTestParty(t, testTenantID, "Invalid")
	sourceID := source.ID
	invalidDependent.CanonicalPartyID = &sourceID

	repo := newFakePartyMergeRepository(source, target, invalidDependent)
	service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

	_, err := service.MergeParties(context.Background(), source.ID, target.ID)

	require.ErrorIs(t, err, ErrInvalidParty)
	assert.Empty(t, repo.logs)
}

func TestMergePartiesHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := NewPartyMergeService(newFakePartyMergeRepository(), func() time.Time { return testNow() }, nil)
	_, err := service.MergeParties(ctx, uuid.New(), uuid.New())

	require.ErrorIs(t, err, context.Canceled)
}

func TestMergePartiesCompressesTransitivePath(t *testing.T) {
	source := activeTestParty(t, testTenantID, "Source")
	target := activeTestParty(t, testTenantID, "Target")
	direct := mergedTestParty(t, testTenantID, source.ID, "Direct")
	grandchild := mergedTestParty(t, testTenantID, direct.ID, "Grandchild")

	repo := newFakePartyMergeRepository(source, target, direct, grandchild)
	service := NewPartyMergeService(repo, func() time.Time { return testNow() }, nil)

	result, err := service.MergeParties(context.Background(), source.ID, target.ID)

	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{direct.ID, grandchild.ID}, result.CompressedPartyIDs)
	require.NotNil(t, direct.CanonicalPartyID)
	assert.Equal(t, target.ID, *direct.CanonicalPartyID)
	require.NotNil(t, grandchild.CanonicalPartyID)
	assert.Equal(t, target.ID, *grandchild.CanonicalPartyID, "la compresión debe ser transitiva")
}
