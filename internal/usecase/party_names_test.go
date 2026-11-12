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

type fakePartyNameRepo struct {
	nameTypes map[string]*domain.NameType
	names     map[uuid.UUID]*domain.PartyName
	validity  map[uuid.UUID]*domain.PartyNameValidity
	closed    int
}

func newFakePartyNameRepo() *fakePartyNameRepo {
	repo := &fakePartyNameRepo{
		nameTypes: make(map[string]*domain.NameType),
		names:     make(map[uuid.UUID]*domain.PartyName),
		validity:  make(map[uuid.UUID]*domain.PartyNameValidity),
	}
	for _, code := range []string{"FIRST", "LAST", "LEGAL", "TRADE", "ALIAS", "FORMER"} {
		repo.nameTypes[code] = &domain.NameType{ID: uuid.New(), Code: code, Name: code}
	}
	return repo
}

func (r *fakePartyNameRepo) FindNameTypeByCode(ctx context.Context, code string) (*domain.NameType, error) {
	nameType, ok := r.nameTypes[code]
	if !ok {
		return nil, domain.ErrNameTypeNotFound
	}
	return nameType, nil
}

func (r *fakePartyNameRepo) FindPartyName(ctx context.Context, tenantID, partyID, nameTypeID uuid.UUID, value string) (*domain.PartyName, error) {
	for _, name := range r.names {
		if name.TenantID == tenantID && name.PartyID == partyID &&
			name.NameTypeID == nameTypeID && strings.EqualFold(name.Value, value) {
			return name, nil
		}
	}
	return nil, domain.ErrPartyNameNotFound
}

func (r *fakePartyNameRepo) GetPartyName(ctx context.Context, tenantID, nameID uuid.UUID) (*domain.PartyName, error) {
	name, ok := r.names[nameID]
	if !ok || name.TenantID != tenantID {
		return nil, domain.ErrPartyNameNotFound
	}
	return name, nil
}

func (r *fakePartyNameRepo) CreatePartyName(ctx context.Context, name *domain.PartyName) error {
	r.names[name.ID] = name
	return nil
}

func (r *fakePartyNameRepo) UpdatePartyName(ctx context.Context, name *domain.PartyName) error {
	r.names[name.ID] = name
	return nil
}

func (r *fakePartyNameRepo) GetOpenValidity(ctx context.Context, tenantID, partyNameID uuid.UUID) (*domain.PartyNameValidity, error) {
	for _, validity := range r.validity {
		if validity.TenantID == tenantID && validity.PartyNameID == partyNameID && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrPartyNameValidityNotFound
}

func (r *fakePartyNameRepo) GetOpenPrimaryValidity(ctx context.Context, tenantID, partyID, nameTypeID uuid.UUID) (*domain.PartyNameValidity, error) {
	for _, validity := range r.validity {
		if validity.TenantID == tenantID && validity.PartyID == partyID &&
			validity.NameTypeID == nameTypeID && validity.IsPrimary && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrPartyNameValidityNotFound
}

func (r *fakePartyNameRepo) CreateValidity(ctx context.Context, validity *domain.PartyNameValidity) error {
	r.validity[validity.ID] = validity
	return nil
}

func (r *fakePartyNameRepo) CloseValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error {
	validity, ok := r.validity[validityID]
	if !ok || validity.TenantID != tenantID {
		return domain.ErrPartyNameValidityNotFound
	}
	if err := validity.Close(at); err != nil {
		return err
	}
	r.closed++
	return nil
}

func partyNameTestService(t *testing.T) (*PartyNameService, *fakePartyNameRepo) {
	t.Helper()
	repo := newFakePartyNameRepo()
	service := NewPartyNameService(repo, newFakeTxManager(),
		func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) })
	return service, repo
}

func TestAddPartyNameCreatesNameAndInitialValidity(t *testing.T) {
	service, repo := partyNameTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()

	result, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: partyID, NameType: "LEGAL",
		Value: "  Acme Corp  ", IsPrimary: true, Ordering: 1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Acme Corp", result.Name.Value)
	assert.True(t, result.Name.IsPrimary)
	assert.Equal(t, tenantID, result.Name.TenantID)
	assert.Equal(t, repo.nameTypes["LEGAL"].ID, result.Name.NameTypeID)
	assert.True(t, result.Validity.IsPrimary)
	assert.Nil(t, result.Validity.EndDate)
	assert.Len(t, repo.names, 1)
	assert.Len(t, repo.validity, 1)
}

func TestAddPartyNameRejectsInvalidType(t *testing.T) {
	service, repo := partyNameTestService(t)

	_, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: uuid.New(), PartyID: uuid.New(), NameType: "INVALID", Value: "Acme",
	})

	require.ErrorIs(t, err, domain.ErrInvalidPartyName)
	assert.Empty(t, repo.names)
	assert.Empty(t, repo.validity)
}

func TestAddPartyNameDoesNotDuplicateOpenValidity(t *testing.T) {
	service, repo := partyNameTestService(t)
	input := AddPartyNameInput{
		TenantID: uuid.New(), PartyID: uuid.New(), NameType: "FIRST",
		Value: "Jane", IsPrimary: true,
	}

	first, err := service.Add(context.Background(), input)
	require.NoError(t, err)
	second, err := service.Add(context.Background(), input)
	require.NoError(t, err)

	assert.Equal(t, first.Name.ID, second.Name.ID)
	assert.Equal(t, first.Validity.ID, second.Validity.ID)
	assert.Len(t, repo.names, 1)
	assert.Len(t, repo.validity, 1)
}

func TestAddPartyNameClosesPreviousPrimary(t *testing.T) {
	service, repo := partyNameTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()

	first, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: partyID, NameType: "LAST", Value: "Doe", IsPrimary: true,
	})
	require.NoError(t, err)
	second, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: partyID, NameType: "LAST", Value: "Smith", IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, first.Validity.EndDate)
	assert.Nil(t, second.Validity.EndDate)
	assert.Equal(t, 1, repo.closed)
}

func TestUpdatePartyNameValidityClosesCurrentAndOpensNew(t *testing.T) {
	service, repo := partyNameTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()

	created, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: partyID, NameType: "LEGAL", Value: "Acme", IsPrimary: true,
	})
	require.NoError(t, err)

	updated, err := service.UpdateValidity(context.Background(), UpdatePartyNameValidityInput{
		TenantID: tenantID, PartyID: partyID, PartyNameID: created.Name.ID,
		IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, created.Validity.EndDate)
	assert.NotEqual(t, created.Validity.ID, updated.Validity.ID)
	assert.Equal(t, created.Name.ID, updated.Name.ID)
	assert.Nil(t, updated.Validity.EndDate)
	assert.Equal(t, 1, repo.closed)
}

func TestUpdatePartyNameValidityCanSwitchToNewValue(t *testing.T) {
	service, repo := partyNameTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()

	created, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: partyID, NameType: "LEGAL", Value: "Acme", IsPrimary: false,
	})
	require.NoError(t, err)

	updated, err := service.UpdateValidity(context.Background(), UpdatePartyNameValidityInput{
		TenantID: tenantID, PartyID: partyID, PartyNameID: created.Name.ID,
		NewValue: "Acme Holdings", IsPrimary: true, Ordering: 2,
	})
	require.NoError(t, err)

	assert.Equal(t, "Acme Holdings", updated.Name.Value)
	assert.NotEqual(t, created.Name.ID, updated.Name.ID)
	assert.NotNil(t, created.Validity.EndDate)
	assert.True(t, updated.Validity.IsPrimary)
	assert.Equal(t, 2, updated.Name.Ordering)
	assert.Len(t, repo.names, 2)
	assert.Len(t, repo.validity, 2)
}

func TestUpdatePartyNameValidityRejectsForeignParty(t *testing.T) {
	service, _ := partyNameTestService(t)
	tenantID := uuid.New()

	created, err := service.Add(context.Background(), AddPartyNameInput{
		TenantID: tenantID, PartyID: uuid.New(), NameType: "FIRST", Value: "Jane",
	})
	require.NoError(t, err)

	_, err = service.UpdateValidity(context.Background(), UpdatePartyNameValidityInput{
		TenantID: tenantID, PartyID: uuid.New(), PartyNameID: created.Name.ID,
	})

	require.ErrorIs(t, err, domain.ErrPartyNameNotFound)
}

func TestUpdatePartyNameValidityRejectsMissingName(t *testing.T) {
	service, _ := partyNameTestService(t)

	_, err := service.UpdateValidity(context.Background(), UpdatePartyNameValidityInput{
		TenantID: uuid.New(), PartyID: uuid.New(), PartyNameID: uuid.New(),
	})

	require.ErrorIs(t, err, domain.ErrPartyNameNotFound)
}
