package usecase

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePartyContactRepo struct {
	contacts   map[uuid.UUID]*domain.ContactMechanism
	byValue    map[string]*domain.ContactMechanism
	relations  map[uuid.UUID]*domain.PartyContactMechanism
	addresses  map[uuid.UUID]*domain.Address
	createCErr error
	updateCErr error
	createAErr error
	updateAErr error
}

func newFakePartyContactRepo() *fakePartyContactRepo {
	return &fakePartyContactRepo{
		contacts:  make(map[uuid.UUID]*domain.ContactMechanism),
		byValue:   make(map[string]*domain.ContactMechanism),
		relations: make(map[uuid.UUID]*domain.PartyContactMechanism),
		addresses: make(map[uuid.UUID]*domain.Address),
	}
}

func (r *fakePartyContactRepo) FindContactMechanismByTypeValue(ctx context.Context, contactType domain.ContactType, value string) (*domain.ContactMechanism, error) {
	if c, ok := r.byValue[string(contactType)+"|"+value]; ok {
		return c, nil
	}
	return nil, domain.ErrContactMechanismNotFound
}

func (r *fakePartyContactRepo) FindOrCreateContactMechanism(ctx context.Context, contact *domain.ContactMechanism) (*domain.ContactMechanism, error) {
	if r.createCErr != nil {
		return nil, r.createCErr
	}
	if existing, ok := r.byValue[string(contact.Type)+"|"+contact.Value]; ok {
		return existing, nil
	}
	r.contacts[contact.ID] = contact
	r.byValue[string(contact.Type)+"|"+contact.Value] = contact
	return contact, nil
}

func (r *fakePartyContactRepo) CreatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if r.createCErr != nil {
		return r.createCErr
	}
	r.relations[relation.ID] = relation
	return nil
}

func (r *fakePartyContactRepo) GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*domain.PartyContact, error) {
	relation, ok := r.relations[relationID]
	if !ok || relation.TenantID != tenantID {
		return nil, domain.ErrPartyContactNotFound
	}
	contact, ok := r.contacts[relation.ContactMechanismID]
	if !ok {
		return nil, domain.ErrContactMechanismNotFound
	}
	return &domain.PartyContact{Relation: relation, Contact: contact}, nil
}

func (r *fakePartyContactRepo) ListPartyContacts(ctx context.Context, tenantID, partyID uuid.UUID) ([]*domain.PartyContact, error) {
	out := make([]*domain.PartyContact, 0)
	for _, relation := range r.relations {
		if relation.TenantID != tenantID || relation.PartyID != partyID || relation.EndDate != nil {
			continue
		}
		contact := r.contacts[relation.ContactMechanismID]
		out = append(out, &domain.PartyContact{Relation: relation, Contact: contact})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Relation.ContactMechanismOrder == out[j].Relation.ContactMechanismOrder {
			return out[i].Relation.StartDate.Before(out[j].Relation.StartDate)
		}
		return out[i].Relation.ContactMechanismOrder < out[j].Relation.ContactMechanismOrder
	})
	return out, nil
}

func (r *fakePartyContactRepo) UpdatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if r.updateCErr != nil {
		return r.updateCErr
	}
	r.relations[relation.ID] = relation
	return nil
}

func (r *fakePartyContactRepo) FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*domain.PartyContactMechanism, error) {
	for _, relation := range r.relations {
		if relation.TenantID == tenantID && relation.ContactMechanismID == contactMechanismID && relation.IsActive(at) {
			return relation, nil
		}
	}
	return nil, domain.ErrPartyContactNotFound
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
	txManager := newFakeTxManager()
	uc := NewPartyContacts(repo, txManager, contactTestNow)
	ctx := context.Background()
	partyID := uuid.New()
	tenantID := testTenantID()

	created, err := uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: partyID, Type: domain.ContactTypeEmail,
		Value: "user@example.com", IsPrimary: true, Order: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	got, err := uc.GetContactMechanism(ctx, tenantID, created.Relation.ID)
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Contact.Value)

	list, err := uc.ListContactMechanisms(ctx, tenantID, partyID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateContactMechanism(ctx, UpdateContactMechanismInput{
		ID: created.Relation.ID, TenantID: tenantID, IsPrimary: true, Order: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, updated.Relation.ContactMechanismOrder)

	require.NoError(t, uc.DeleteContactMechanism(ctx, tenantID, created.Relation.ID))
	assert.NotNil(t, updated.Relation.EndDate)
	assert.Equal(t, 5, txManager.calls, "every write and read must run inside the manager")
}

func TestPartyContactsRespectOrder(t *testing.T) {
	repo := newFakePartyContactRepo()
	uc := NewPartyContacts(repo, newFakeTxManager(), contactTestNow)
	ctx := context.Background()
	partyID := uuid.New()
	tenantID := testTenantID()

	_, err := uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: partyID, Type: domain.ContactTypePhone,
		Value: "+58 412 1234567", Order: 3,
	})
	require.NoError(t, err)
	_, err = uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: partyID, Type: domain.ContactTypeEmail,
		Value: "user@example.com", Order: 1,
	})
	require.NoError(t, err)
	_, err = uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: partyID, Type: domain.ContactTypeWebsite,
		Value: "https://example.com", Order: 2,
	})
	require.NoError(t, err)

	list, err := uc.ListContactMechanisms(ctx, tenantID, partyID)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, 1, list[0].Relation.ContactMechanismOrder)
	assert.Equal(t, 2, list[1].Relation.ContactMechanismOrder)
	assert.Equal(t, 3, list[2].Relation.ContactMechanismOrder)
}

func TestPartyContactsReusesGlobalContact(t *testing.T) {
	repo := newFakePartyContactRepo()
	uc := NewPartyContacts(repo, newFakeTxManager(), contactTestNow)
	ctx := context.Background()
	tenantID := testTenantID()

	first, err := uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "shared@example.com", Order: 1,
	})
	require.NoError(t, err)
	second, err := uc.CreateContactMechanism(ctx, CreateContactMechanismInput{
		TenantID: tenantID, PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "shared@example.com", Order: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, first.Contact.ID, second.Contact.ID)
	assert.NotEqual(t, first.Relation.ID, second.Relation.ID)
	assert.Len(t, repo.contacts, 1)
}

func TestPartyContactsAddressCRUD(t *testing.T) {
	repo := newFakePartyContactRepo()
	uc := NewPartyContacts(repo, newFakeTxManager(), contactTestNow)
	ctx := context.Background()
	partyID := uuid.New()

	cityID := uuid.New()
	address, err := uc.CreateAddress(ctx, CreateAddressInput{
		TenantID: testTenantID(), PartyID: partyID, Type: domain.AddressTypeLegal,
		Line1: "Av. Principal", CityLocationID: cityID, Country: "ve", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "VE", address.Country)

	got, err := uc.GetAddress(ctx, address.ID)
	require.NoError(t, err)
	assert.Equal(t, cityID, got.CityLocationID)

	list, err := uc.ListAddresses(ctx, partyID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	newCityID := uuid.New()
	updated, err := uc.UpdateAddress(ctx, UpdateAddressInput{
		ID: address.ID, Type: domain.AddressTypeFiscal, Line1: "Calle 2",
		CityLocationID: newCityID, Country: "VE", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, newCityID, updated.CityLocationID)

	require.NoError(t, uc.DeleteAddress(ctx, address.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestPartyContactsValidation(t *testing.T) {
	uc := NewPartyContacts(newFakePartyContactRepo(), newFakeTxManager(), contactTestNow)

	_, err := uc.CreateContactMechanism(context.Background(), CreateContactMechanismInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "not-an-email",
	})
	require.ErrorIs(t, err, domain.ErrInvalidContactMechanism)

	_, err = uc.CreateAddress(context.Background(), CreateAddressInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.AddressTypeLegal,
		Line1: "Av. Principal", CityLocationID: uuid.New(), Country: "USA",
	})
	require.ErrorIs(t, err, domain.ErrInvalidAddress)
}

func TestPartyContactsPropagatesRepositoryError(t *testing.T) {
	repo := newFakePartyContactRepo()
	repo.createCErr = errors.New("db down")
	uc := NewPartyContacts(repo, newFakeTxManager(), contactTestNow)

	_, err := uc.CreateContactMechanism(context.Background(), CreateContactMechanismInput{
		TenantID: testTenantID(), PartyID: uuid.New(), Type: domain.ContactTypeEmail,
		Value: "user@example.com",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "find or create contact mechanism")
}
