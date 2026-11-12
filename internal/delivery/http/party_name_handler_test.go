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

type fakePartyNameUseCase struct {
	addInput    usecase.AddPartyNameInput
	updateInput usecase.UpdatePartyNameValidityInput
	result      *usecase.PartyNameResult
	err         error
}

func (f *fakePartyNameUseCase) Add(ctx context.Context, in usecase.AddPartyNameInput) (*usecase.PartyNameResult, error) {
	f.addInput = in
	return f.result, f.err
}

func (f *fakePartyNameUseCase) UpdateValidity(ctx context.Context, in usecase.UpdatePartyNameValidityInput) (*usecase.PartyNameResult, error) {
	f.updateInput = in
	return f.result, f.err
}

func partyNameResult(tenantID, partyID uuid.UUID) *usecase.PartyNameResult {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	name := &domain.PartyName{
		ID: uuid.New(), TenantID: tenantID, PartyID: partyID, NameTypeID: uuid.New(),
		Value: "Acme", IsPrimary: true, Ordering: 1, CreatedAt: now, UpdatedAt: now,
	}
	validity := &domain.PartyNameValidity{
		ID: uuid.New(), TenantID: tenantID, PartyNameID: name.ID, PartyID: partyID,
		NameTypeID: name.NameTypeID, IsPrimary: true, StartDate: now,
	}
	return &usecase.PartyNameResult{Name: name, Validity: validity}
}

func TestAddPartyNameHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	partyID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	uc := &fakePartyNameUseCase{result: partyNameResult(tenantID, partyID)}
	router := NewRouter(Dependencies{PartyNameUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+partyID.String()+"/names",
		[]byte(`{"name_type":"LEGAL","value":"Acme","is_primary":true,"ordering":1}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.addInput.TenantID)
	assert.Equal(t, partyID, uc.addInput.PartyID)
	assert.Equal(t, "LEGAL", uc.addInput.NameType)
	assert.Equal(t, "Acme", uc.addInput.Value)
	assert.True(t, uc.addInput.IsPrimary)
	body := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, "Acme", body["value"])
	assert.Equal(t, true, body["is_primary"])
}

func TestUpdatePartyNameValidityHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	partyID := uuid.New()
	nameID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	uc := &fakePartyNameUseCase{result: partyNameResult(tenantID, partyID)}
	router := NewRouter(Dependencies{PartyNameUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPut,
		"/api/v1/parties/"+partyID.String()+"/names/"+nameID.String()+"/validity",
		[]byte(`{"new_value":"Acme Holdings","is_primary":true,"ordering":2}`), token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.updateInput.TenantID)
	assert.Equal(t, partyID, uc.updateInput.PartyID)
	assert.Equal(t, nameID, uc.updateInput.PartyNameID)
	assert.Equal(t, "Acme Holdings", uc.updateInput.NewValue)
	assert.True(t, uc.updateInput.IsPrimary)
	assert.Equal(t, 2, uc.updateInput.Ordering)
}

func TestPartyNameHandlerRequiresSession(t *testing.T) {
	manager := testAuthManager(t)
	uc := &fakePartyNameUseCase{}
	router := NewRouter(Dependencies{PartyNameUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+uuid.New().String()+"/names", []byte(`{"name_type":"LEGAL","value":"Acme"}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
