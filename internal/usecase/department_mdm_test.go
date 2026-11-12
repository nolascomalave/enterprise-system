package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDepartmentMDMRepo struct {
	contacts          map[uuid.UUID]*domain.ContactMechanism
	byValue           map[string]*domain.ContactMechanism
	relations         map[uuid.UUID]*domain.DepartmentContactMechanism
	contactValidities map[uuid.UUID]*domain.DepartmentContactMechanismValidity
	addresses         map[uuid.UUID]*domain.DepartmentAddress
	addressValidities map[uuid.UUID]*domain.DepartmentAddressValidity
}

func newFakeDepartmentMDMRepo() *fakeDepartmentMDMRepo {
	return &fakeDepartmentMDMRepo{
		contacts:          make(map[uuid.UUID]*domain.ContactMechanism),
		byValue:           make(map[string]*domain.ContactMechanism),
		relations:         make(map[uuid.UUID]*domain.DepartmentContactMechanism),
		contactValidities: make(map[uuid.UUID]*domain.DepartmentContactMechanismValidity),
		addresses:         make(map[uuid.UUID]*domain.DepartmentAddress),
		addressValidities: make(map[uuid.UUID]*domain.DepartmentAddressValidity),
	}
}

func (r *fakeDepartmentMDMRepo) FindOrCreateContactMechanism(ctx context.Context, contact *domain.ContactMechanism) (*domain.ContactMechanism, error) {
	key := string(contact.Type) + "|" + strings.ToLower(contact.Value)
	if existing, ok := r.byValue[key]; ok {
		return existing, nil
	}
	r.contacts[contact.ID] = contact
	r.byValue[key] = contact
	return contact, nil
}

func (r *fakeDepartmentMDMRepo) CreateDepartmentContact(ctx context.Context, contact *domain.DepartmentContactMechanism) error {
	r.relations[contact.ID] = contact
	return nil
}

func (r *fakeDepartmentMDMRepo) GetDepartmentContact(ctx context.Context, tenantID, id uuid.UUID) (*domain.DepartmentContactMechanism, error) {
	contact, ok := r.relations[id]
	if !ok || contact.TenantID != tenantID {
		return nil, domain.ErrDepartmentContactNotFound
	}
	return contact, nil
}

func (r *fakeDepartmentMDMRepo) GetOpenDepartmentContactValidity(ctx context.Context, tenantID, contactID uuid.UUID) (*domain.DepartmentContactMechanismValidity, error) {
	for _, validity := range r.contactValidities {
		if validity.TenantID == tenantID && validity.DepartmentContactMechanismID == contactID && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrDepartmentContactNotFound
}

func (r *fakeDepartmentMDMRepo) GetOpenPrimaryDepartmentContactValidity(ctx context.Context, tenantID, departmentID uuid.UUID, contactType domain.ContactType) (*domain.DepartmentContactMechanismValidity, error) {
	for _, validity := range r.contactValidities {
		if validity.TenantID == tenantID && validity.DepartmentID == departmentID &&
			validity.ContactType == contactType && validity.IsPrimary && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrDepartmentContactNotFound
}

func (r *fakeDepartmentMDMRepo) CreateDepartmentContactValidity(ctx context.Context, validity *domain.DepartmentContactMechanismValidity) error {
	r.contactValidities[validity.ID] = validity
	return nil
}

func (r *fakeDepartmentMDMRepo) CloseDepartmentContactValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error {
	validity, ok := r.contactValidities[validityID]
	if !ok || validity.TenantID != tenantID {
		return domain.ErrDepartmentContactNotFound
	}
	return validity.Close(at)
}

func (r *fakeDepartmentMDMRepo) CreateDepartmentAddress(ctx context.Context, address *domain.DepartmentAddress) error {
	r.addresses[address.ID] = address
	return nil
}

func (r *fakeDepartmentMDMRepo) GetDepartmentAddress(ctx context.Context, tenantID, id uuid.UUID) (*domain.DepartmentAddress, error) {
	address, ok := r.addresses[id]
	if !ok || address.TenantID != tenantID {
		return nil, domain.ErrDepartmentAddressNotFound
	}
	return address, nil
}

func (r *fakeDepartmentMDMRepo) GetOpenDepartmentAddressValidity(ctx context.Context, tenantID, addressID uuid.UUID) (*domain.DepartmentAddressValidity, error) {
	for _, validity := range r.addressValidities {
		if validity.TenantID == tenantID && validity.DepartmentAddressID == addressID && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrDepartmentAddressNotFound
}

func (r *fakeDepartmentMDMRepo) GetOpenPrimaryDepartmentAddressValidity(ctx context.Context, tenantID, departmentID uuid.UUID, addressType domain.AddressType) (*domain.DepartmentAddressValidity, error) {
	for _, validity := range r.addressValidities {
		if validity.TenantID == tenantID && validity.DepartmentID == departmentID &&
			validity.Type == addressType && validity.IsPrimary && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrDepartmentAddressNotFound
}

func (r *fakeDepartmentMDMRepo) CreateDepartmentAddressValidity(ctx context.Context, validity *domain.DepartmentAddressValidity) error {
	r.addressValidities[validity.ID] = validity
	return nil
}

func (r *fakeDepartmentMDMRepo) CloseDepartmentAddressValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error {
	validity, ok := r.addressValidities[validityID]
	if !ok || validity.TenantID != tenantID {
		return domain.ErrDepartmentAddressNotFound
	}
	return validity.Close(at)
}

func departmentMDMService(t *testing.T) (*DepartmentMDMService, *fakeDepartmentMDMRepo) {
	t.Helper()
	repo := newFakeDepartmentMDMRepo()
	service := NewDepartmentMDMService(repo, newFakeTxManager(),
		func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) })
	return service, repo
}

func TestAddDepartmentContactCreatesValidity(t *testing.T) {
	service, repo := departmentMDMService(t)
	tenantID, departmentID := uuid.New(), uuid.New()

	result, err := service.AddContact(context.Background(), AddDepartmentContactInput{
		TenantID: tenantID, DepartmentID: departmentID, Type: domain.ContactTypePhone,
		Value: "+58 412 1234567", Order: 1, IsPrimary: true,
	})

	require.NoError(t, err)
	assert.Equal(t, domain.ContactTypePhone, result.Contact.Type)
	assert.True(t, result.Validity.IsPrimary)
	assert.Nil(t, result.Validity.EndDate)
	assert.Len(t, repo.contactValidities, 1)
}

func TestDepartmentOnlyOnePrimaryContactValidity(t *testing.T) {
	service, repo := departmentMDMService(t)
	tenantID, departmentID := uuid.New(), uuid.New()

	first, err := service.AddContact(context.Background(), AddDepartmentContactInput{
		TenantID: tenantID, DepartmentID: departmentID, Type: domain.ContactTypePhone,
		Value: "+58 412 1111111", IsPrimary: true,
	})
	require.NoError(t, err)
	second, err := service.AddContact(context.Background(), AddDepartmentContactInput{
		TenantID: tenantID, DepartmentID: departmentID, Type: domain.ContactTypePhone,
		Value: "+58 412 2222222", IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, first.Validity.EndDate, "the previous primary phone must be closed")
	assert.Nil(t, second.Validity.EndDate)
	openPrimary := 0
	for _, validity := range repo.contactValidities {
		if validity.ContactType == domain.ContactTypePhone && validity.IsPrimary && validity.EndDate == nil {
			openPrimary++
		}
	}
	assert.Equal(t, 1, openPrimary, "only one primary phone validity can be open")
}

func TestUpdateDepartmentContactValidityKeepsHistory(t *testing.T) {
	service, repo := departmentMDMService(t)
	tenantID, departmentID := uuid.New(), uuid.New()
	created, err := service.AddContact(context.Background(), AddDepartmentContactInput{
		TenantID: tenantID, DepartmentID: departmentID, Type: domain.ContactTypeEmail,
		Value: "dept@example.com", IsPrimary: true,
	})
	require.NoError(t, err)

	updated, err := service.UpdateContactValidity(context.Background(), UpdateDepartmentContactValidityInput{
		TenantID: tenantID, DepartmentID: departmentID,
		DepartmentContactID: created.Relation.ID, IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, created.Validity.EndDate)
	assert.NotEqual(t, created.Validity.ID, updated.Validity.ID)
	assert.Nil(t, updated.Validity.EndDate)
	assert.Len(t, repo.contactValidities, 2)
}

func TestAddDepartmentAddressAndValidity(t *testing.T) {
	service, repo := departmentMDMService(t)
	tenantID, departmentID, cityID := uuid.New(), uuid.New(), uuid.New()

	first, err := service.AddAddress(context.Background(), AddDepartmentAddressInput{
		TenantID: tenantID, DepartmentID: departmentID, CityLocationID: cityID,
		Type: domain.AddressTypeLegal, Line1: "Av. Principal", IsPrimary: true,
	})
	require.NoError(t, err)
	second, err := service.AddAddress(context.Background(), AddDepartmentAddressInput{
		TenantID: tenantID, DepartmentID: departmentID, CityLocationID: uuid.New(),
		Type: domain.AddressTypeLegal, Line1: "Calle 2", IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, first.Validity.EndDate)
	assert.Nil(t, second.Validity.EndDate)
	assert.Len(t, repo.addressValidities, 2)

	updated, err := service.UpdateAddressValidity(context.Background(), UpdateDepartmentAddressValidityInput{
		TenantID: tenantID, DepartmentID: departmentID,
		DepartmentAddressID: second.Address.ID, IsPrimary: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, second.Validity.EndDate)
	assert.Nil(t, updated.Validity.EndDate)
	assert.Len(t, repo.addressValidities, 3)
}
