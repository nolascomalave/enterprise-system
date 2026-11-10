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

type fakePartyContactRepo struct {
	contacts   map[uuid.UUID]*domain.ContactMechanism
	addresses  map[uuid.UUID]*domain.Address
	createCErr error
	updateCErr error
	createAErr error
	updateAErr error
}

func newFakePartyContactRepo() *fakePartyContactRepo {
	return &fakePartyContactRepo{
		contacts:  make(map[uuid.UUID]*domain.ContactMechanism),
		addresses: make(map[uuid.UUID]*domain.Address),
	}
}

func (r *fakePartyContactRepo) CreateContactMechanism(ctx context.Context, e *domain.ContactMechanism) error {
	if r.createCErr != nil {
		return r.createCErr
	}
	r.contacts[e.ID] = e
	return nil
}
func (r *fakePartyContactRepo) GetContactMechanism(ctx context.Context, id uuid.UUID) (*domain.ContactMechanism, error) {
	e, ok := r.contacts[id]
	if !ok {
		return nil, errors.New("contact not found")
	}
	return e, nil
}
func (r *fakePartyContactRepo) ListContactMechanisms(ctx context.Context, partyID uuid.UUID) ([]*domain.ContactMechanism, error) {
	out := make([]*domain.ContactMechanism, 0)
	for _, e := range r.contacts {
		if e.PartyID == partyID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (r *fakePartyContactRepo) UpdateContactMechanism(ctx context.Context, e *domain.ContactMechanism) error {
	if r.updateCErr != nil {
		return r.updateCErr
	}
	r.contacts[e.ID] = e
	return nil
}

func (r *fakePartyContactRepo) CreateAddress(ctx context.Context, e *domain.Address) error {
	if r.createAErr != nil {
		return r.createAErr
	}
	r.addresses[e.ID] = e
	return nil
}
func (r *fakePartyContactRepo) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	e, ok := r.addresses[id]
	if !ok {
		return nil, errors.New("address not found")
	}
	return e, nil
}
func (r *fakePartyContactRepo) ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*domain.Address, error) {
	out := make([]*domain.Address, 0)
	for _, e := range r.addresses {
		if e.PartyID == partyID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (r *fakePartyContactRepo) UpdateAddress(ctx context.Context, e *domain.Address) error {
	if r.updateAErr != nil {
		return r.updateAErr
	}
	r.addresses[e.ID] = e
	return nil
}

func contactTestNow() time.Time {
	return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
}

func TestPartyContactsContactCRUD(t *testing.T) {
	repo := newFakePartyContactRepo()
	uc := NewPartyContacts(repo, contactTestNow)
	ctx := context.Background()
	partyID := uuid.New()

	contact, err := uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: testTenantID(), PartyID: partyID, Type: domain.ContactTypeEmail,
		Value: "user@example.com", IsPrimary: true,
	})
	require.NoError(t, err)

	got, err := uc.GetContactMechanism(ctx, contact.ID)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Value)

	list, err := uc.ListContactMechanisms(ctx, partyID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateContactMechanism(ctx, UpdateContactMechanismInput{
		ID: contact.ID, Type: domain.ContactTypeEmail, Value: "other@example.com", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "other@example.com", updated.Value)

	require.NoError(t, uc.DeleteContactMechanism(ctx, contact.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestPartyContactsAddressCRUD(t *testing.T) {
	repo := newFakePartyContactRepo()
	uc := NewPartyContacts(repo, contactTestNow)
	ctx := context.Background()
	partyID := uuid.New()

	address, err := uc.CreateAddress(ctx, CreateAddressInput{
		TenantID: testTenantID(), PartyID: partyID, Type: domain.AddressTypeLegal,
		Line1: "Av. Principal", City: "Caracas", Country: "ve", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "VE", address.Country)

	got, err := uc.GetAddress(ctx, address.ID)
	require.NoError(t, err)
	assert.Equal(t, "Caracas", got.City)

	list, err := uc.ListAddresses(ctx, partyID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateAddress(ctx, UpdateAddressInput{
		ID: address.ID, Type: domain.AddressTypeFiscal, Line1: "Calle 2",
		City: "Valencia", Country: "VE", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Valencia", updated.City)

	require.NoError(t, uc.DeleteAddress(ctx, address.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestPartyContactsValidation(t *testing.T) {
	uc := NewPartyContacts(newFakePartyContactRepo(), contactTestNow)

	_, err := uc.CreateContactMechanism(context.Background(), CreateContactMechanismInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "not-an-email",
	})
	require.ErrorIs(t, err, domain.ErrInvalidContactMechanism)

	_, err = uc.CreateAddress(context.Background(), CreateAddressInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.AddressTypeLegal,
		Line1: "Av. Principal", City: "Caracas", Country: "USA",
	})
	require.ErrorIs(t, err, domain.ErrInvalidAddress)
}

func TestPartyContactsPropagatesRepositoryError(t *testing.T) {
	repo := newFakePartyContactRepo()
	repo.createCErr = errors.New("db down")
	uc := NewPartyContacts(repo, contactTestNow)

	_, err := uc.CreateContactMechanism(context.Background(), CreateContactMechanismInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "user@example.com",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create contact mechanism")
}
