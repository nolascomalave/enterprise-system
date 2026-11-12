package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMergeRepository struct {
	parties   map[uuid.UUID]*domain.Party
	updates   []*domain.Party
	logs      []*domain.PartyMergeLog
	getErr    error
	findErr   error
	updateErr error
	logErr    error
}

func newFakeMergeRepository(parties ...*domain.Party) *fakeMergeRepository {
	repo := &fakeMergeRepository{parties: make(map[uuid.UUID]*domain.Party, len(parties))}
	for _, p := range parties {
		repo.parties[p.ID] = p
	}
	return repo
}

func (r *fakeMergeRepository) GetParty(ctx context.Context, partyID uuid.UUID) (*domain.Party, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	p, ok := r.parties[partyID]
	if !ok {
		return nil, errors.New("party not found")
	}
	return p, nil
}

func (r *fakeMergeRepository) FindPartiesByCanonicalID(ctx context.Context, canonicalPartyID uuid.UUID) ([]*domain.Party, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	var result []*domain.Party
	for _, p := range r.parties {
		if p.CanonicalPartyID != nil && *p.CanonicalPartyID == canonicalPartyID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *fakeMergeRepository) UpdateParty(ctx context.Context, party *domain.Party) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.parties[party.ID] = party
	r.updates = append(r.updates, party)
	return nil
}

func (r *fakeMergeRepository) CreatePartyMergeLog(ctx context.Context, entry *domain.PartyMergeLog) error {
	if r.logErr != nil {
		return r.logErr
	}
	r.logs = append(r.logs, entry)
	return nil
}

func testParty(t *testing.T, name string) *domain.Party {
	t.Helper()

	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	p, err := domain.NewParty(uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001"), domain.PartyTypeIndividual, name, now)
	require.NoError(t, err)
	return p
}

func TestMergePartiesUseCaseExecutesInTransaction(t *testing.T) {
	source := testParty(t, "Source")
	target := testParty(t, "Target")
	repo := newFakeMergeRepository(source, target)
	txManager := newFakeTxManager()
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	actor := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-0000000000a1")
	uc := NewMergeParties(txManager, repo, now, &actor)

	result, err := uc.Execute(context.Background(), source.ID, target.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, txManager.calls)
	assert.Equal(t, source.ID, result.SourceID)
	assert.Equal(t, target.ID, result.TargetID)
	assert.Equal(t, domain.PartyRecordStatusMerged, source.RecordStatus)
	require.Len(t, repo.logs, 1)
	require.Len(t, repo.updates, 1)
	require.NotNil(t, repo.logs[0].MergedBy)
	assert.Equal(t, actor, *repo.logs[0].MergedBy)
}

func TestMergePartiesUseCasePropagatesDomainError(t *testing.T) {
	source := testParty(t, "Source")
	target := testParty(t, "Target")
	repo := newFakeMergeRepository(source, target)
	repo.getErr = errors.New("db down")
	txManager := newFakeTxManager()
	uc := NewMergeParties(txManager, repo, nil, nil)

	result, err := uc.Execute(context.Background(), source.ID, target.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, txManager.calls)
	assert.Empty(t, repo.logs)
}

func TestMergePartiesUseCasePropagatesTransactionError(t *testing.T) {
	txErr := errors.New("commit failed")
	txManager := newFakeTxManager()
	txManager.err = txErr
	uc := NewMergeParties(txManager, newFakeMergeRepository(), nil, nil)

	result, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.ErrorIs(t, err, txErr)
	assert.Nil(t, result)
	assert.Equal(t, 1, txManager.calls)
}

func TestMergePartiesUseCaseRejectsNilTransactor(t *testing.T) {
	uc := NewMergeParties(nil, nil, nil, nil)

	result, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	require.ErrorIs(t, err, domain.ErrInvalidPartyMerge)
	assert.Nil(t, result)
}
