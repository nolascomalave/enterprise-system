package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDepartmentMDMUseCase struct {
	contactInput usecase.AddDepartmentContactInput
	addressInput usecase.AddDepartmentAddressInput
	contact      *usecase.DepartmentContactResult
	address      *usecase.DepartmentAddressResult
	err          error
}

func (f *fakeDepartmentMDMUseCase) AddContact(ctx context.Context, in usecase.AddDepartmentContactInput) (*usecase.DepartmentContactResult, error) {
	f.contactInput = in
	return f.contact, f.err
}

func (f *fakeDepartmentMDMUseCase) AddAddress(ctx context.Context, in usecase.AddDepartmentAddressInput) (*usecase.DepartmentAddressResult, error) {
	f.addressInput = in
	return f.address, f.err
}

func TestAddDepartmentContactHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID, departmentID := uuid.New(), uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	contact := &domain.ContactMechanism{ID: uuid.New(), Type: domain.ContactTypePhone, Value: "+58 412 1234567"}
	relation := &domain.DepartmentContactMechanism{
		ID: uuid.New(), TenantID: tenantID, DepartmentID: departmentID,
		ContactMechanismID: contact.ID, ContactType: contact.Type,
		ContactMechanismOrder: 1, IsPrimary: true, CreatedAt: now, UpdatedAt: now,
	}
	validity := &domain.DepartmentContactMechanismValidity{
		ID: uuid.New(), TenantID: tenantID, DepartmentContactMechanismID: relation.ID,
		DepartmentID: departmentID, ContactType: contact.Type, IsPrimary: true, StartDate: now,
	}
	uc := &fakeDepartmentMDMUseCase{contact: &usecase.DepartmentContactResult{
		Contact: contact, Relation: relation, Validity: validity,
	}}
	router := NewRouter(Dependencies{DepartmentMDMUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/departments/"+departmentID.String()+"/contacts",
		[]byte(`{"type":"PHONE","value":"+58 412 1234567","order":1,"is_primary":true}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.contactInput.TenantID)
	assert.Equal(t, departmentID, uc.contactInput.DepartmentID)
	assert.Equal(t, domain.ContactTypePhone, uc.contactInput.Type)
	assert.True(t, uc.contactInput.IsPrimary)
	body := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, "+58 412 1234567", body["value"])
}

func TestAddDepartmentAddressHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID, departmentID, cityID := uuid.New(), uuid.New(), uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	address := &domain.DepartmentAddress{
		ID: uuid.New(), TenantID: tenantID, DepartmentID: departmentID,
		CityLocationID: cityID, Type: domain.AddressTypeLegal, Line1: "Av. Principal",
		CreatedAt: now, UpdatedAt: now,
	}
	validity := &domain.DepartmentAddressValidity{
		ID: uuid.New(), TenantID: tenantID, DepartmentAddressID: address.ID,
		DepartmentID: departmentID, Type: address.Type, IsPrimary: true, StartDate: now,
	}
	uc := &fakeDepartmentMDMUseCase{address: &usecase.DepartmentAddressResult{Address: address, Validity: validity}}
	router := NewRouter(Dependencies{DepartmentMDMUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/departments/"+departmentID.String()+"/addresses",
		[]byte(`{"type":"LEGAL","line1":"Av. Principal","city_location_id":"`+cityID.String()+`","is_primary":true}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.addressInput.TenantID)
	assert.Equal(t, departmentID, uc.addressInput.DepartmentID)
	assert.Equal(t, cityID, uc.addressInput.CityLocationID)
	assert.True(t, uc.addressInput.IsPrimary)
	body := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, "Av. Principal", body["line1"])
}

func TestDepartmentMDMHandlerRequiresSession(t *testing.T) {
	manager := testAuthManager(t)
	router := NewRouter(Dependencies{DepartmentMDMUseCase: &fakeDepartmentMDMUseCase{}, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/departments/"+uuid.New().String()+"/contacts",
		[]byte(`{"type":"PHONE","value":"123"}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
