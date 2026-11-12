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

type fakePartyRepo struct {
	detail  *domain.PartyDetail
	parties []*domain.Party
	create  *domain.PartyDetail
	err     error
}

func (r *fakePartyRepo) CreateParty(ctx context.Context, party *domain.Party, person *domain.PersonProfile, org *domain.OrganizationProfile) error {
	if r.err != nil {
		return r.err
	}
	r.create = &domain.PartyDetail{Party: party, PersonProfile: person, OrganizationProfile: org}
	return nil
}

func (r *fakePartyRepo) GetPartyDetail(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyDetail, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.detail == nil {
		return nil, domain.ErrPartyNotFound
	}
	return r.detail, nil
}

func (r *fakePartyRepo) ListParties(ctx context.Context, tenantID uuid.UUID) ([]*domain.Party, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.parties, nil
}

func partyTestNow() time.Time {
	return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
}

func TestPartyServiceCreateIndividual(t *testing.T) {
	repo := &fakePartyRepo{}
	txManager := newFakeTxManager()
	uc := NewPartyService(repo, txManager, partyTestNow)

	detail, err := uc.Create(context.Background(), CreatePartyInput{
		TenantID:    testTenantID(),
		PartyType:   domain.PartyTypeIndividual,
		DisplayName: "Jane Doe",
		Person: &CreatePersonProfileInput{
			FirstName: "Jane", LastName: "Doe",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, domain.PartyTypeIndividual, detail.Party.PartyType)
	require.NotNil(t, detail.PersonProfile)
	assert.Equal(t, "Doe", detail.PersonProfile.LastName)
	assert.Nil(t, detail.OrganizationProfile)
	require.NotNil(t, repo.create)
	assert.Equal(t, 1, txManager.calls, "the write use case must open a transaction")
}

func TestPartyServiceRequiresProfile(t *testing.T) {
	uc := NewPartyService(&fakePartyRepo{}, newFakeTxManager(), partyTestNow)

	_, err := uc.Create(context.Background(), CreatePartyInput{
		TenantID: testTenantID(), PartyType: domain.PartyTypeIndividual, DisplayName: "Jane",
	})
	require.ErrorIs(t, err, domain.ErrInvalidParty)

	_, err = uc.Create(context.Background(), CreatePartyInput{
		TenantID: testTenantID(), PartyType: domain.PartyTypeOrganization, DisplayName: "Acme",
	})
	require.ErrorIs(t, err, domain.ErrInvalidParty)
}

func TestPartyServiceCreateOrganization(t *testing.T) {
	uc := NewPartyService(&fakePartyRepo{}, newFakeTxManager(), partyTestNow)

	detail, err := uc.Create(context.Background(), CreatePartyInput{
		TenantID: testTenantID(), PartyType: domain.PartyTypeOrganization, DisplayName: "Acme",
		Organization: &CreateOrganizationProfileInput{
			LegalName: "Acme LLC", TaxID: "J-123",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, detail.OrganizationProfile)
	assert.Equal(t, "Acme LLC", detail.OrganizationProfile.LegalName)
	assert.Nil(t, detail.PersonProfile)
}

func TestPartyServiceRejectsWrongProfile(t *testing.T) {
	uc := NewPartyService(&fakePartyRepo{}, newFakeTxManager(), partyTestNow)

	_, err := uc.Create(context.Background(), CreatePartyInput{
		TenantID: testTenantID(), PartyType: domain.PartyTypeIndividual, DisplayName: "Jane",
		Person:       &CreatePersonProfileInput{FirstName: "Jane", LastName: "Doe"},
		Organization: &CreateOrganizationProfileInput{LegalName: "Acme", TaxID: "J-1"},
	})
	require.ErrorIs(t, err, domain.ErrInvalidParty)
}

func TestPartyServiceGetAndList(t *testing.T) {
	party := &domain.Party{ID: uuid.New(), TenantID: testTenantID(), DisplayName: "Jane"}
	repo := &fakePartyRepo{detail: &domain.PartyDetail{Party: party}, parties: []*domain.Party{party}}
	txManager := newFakeTxManager()
	uc := NewPartyService(repo, txManager, partyTestNow)

	detail, err := uc.Get(context.Background(), testTenantID(), party.ID)
	require.NoError(t, err)
	assert.Equal(t, party.ID, detail.Party.ID)

	list, err := uc.List(context.Background(), testTenantID())
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, 2, txManager.calls, "RLS-protected reads must also run inside a transaction")
}

func TestPartyServicePropagatesRepositoryError(t *testing.T) {
	repo := &fakePartyRepo{err: errors.New("db down")}
	uc := NewPartyService(repo, newFakeTxManager(), partyTestNow)

	_, err := uc.Create(context.Background(), CreatePartyInput{
		TenantID: testTenantID(), PartyType: domain.PartyTypeIndividual, DisplayName: "Jane",
		Person: &CreatePersonProfileInput{FirstName: "Jane", LastName: "Doe"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create party")
}
