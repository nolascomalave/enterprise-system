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

type fakeIdentityUseCase struct {
	nationality   *domain.PartyNationality
	nationalityIn usecase.AddPartyNationalityInput
	document      *usecase.IdentityDocumentResult
	documentIn    usecase.AddPartyIdentityDocumentInput
	err           error
}

func (f *fakeIdentityUseCase) AddNationality(ctx context.Context, in usecase.AddPartyNationalityInput) (*domain.PartyNationality, error) {
	f.nationalityIn = in
	return f.nationality, f.err
}

func (f *fakeIdentityUseCase) AddIdentityDocument(ctx context.Context, in usecase.AddPartyIdentityDocumentInput) (*usecase.IdentityDocumentResult, error) {
	f.documentIn = in
	return f.document, f.err
}

func TestAddPartyNationalityHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	partyID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	uc := &fakeIdentityUseCase{nationality: &domain.PartyNationality{
		ID: uuid.New(), TenantID: tenantID, PartyID: partyID,
		CountryCode: "VE", IsPrimary: true, CreatedAt: now, UpdatedAt: now,
	}}
	router := NewRouter(Dependencies{IdentityUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+partyID.String()+"/nationalities",
		[]byte(`{"country_code":"VE","is_primary":true}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.nationalityIn.TenantID)
	assert.Equal(t, partyID, uc.nationalityIn.PartyID)
	assert.Equal(t, "VE", uc.nationalityIn.CountryCode)
	assert.True(t, uc.nationalityIn.IsPrimary)
}

func TestAddPartyIdentityDocumentHandler(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	partyID := uuid.New()
	jurisdictionID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	document := &domain.PartyIdentityDocument{
		ID: uuid.New(), TenantID: tenantID, PartyID: partyID, DocumentTypeID: uuid.New(),
		IssuingCountryCode: "VE", IssuingJurisdictionID: &jurisdictionID,
		DocumentNumber: "12345678", CreatedAt: now, UpdatedAt: now,
	}
	validity := &domain.PartyIdentityDocumentValidity{
		ID: uuid.New(), TenantID: tenantID, PartyIdentityDocumentID: document.ID,
		PartyID: partyID, IsPrimary: true, StartDate: now,
	}
	uc := &fakeIdentityUseCase{document: &usecase.IdentityDocumentResult{
		Document: document, Validity: validity, TypeCode: "PASSPORT",
	}}
	router := NewRouter(Dependencies{IdentityUseCase: uc, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+partyID.String()+"/identity-documents",
		[]byte(`{"document_type":"PASSPORT","issuing_country_code":"VE","issuing_jurisdiction_id":"`+
			jurisdictionID.String()+`","document_number":"12345678","is_primary":true}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.documentIn.TenantID)
	assert.Equal(t, partyID, uc.documentIn.PartyID)
	assert.Equal(t, "PASSPORT", uc.documentIn.DocumentType)
	require.NotNil(t, uc.documentIn.IssuingJurisdictionID)
	assert.Equal(t, jurisdictionID, *uc.documentIn.IssuingJurisdictionID)
	body := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, "PASSPORT", body["document_type"])
	assert.Equal(t, "12345678", body["document_number"])
}

func TestAddIdentityDocumentHandlerRejectsInvalidJurisdiction(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	partyID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	router := NewRouter(Dependencies{IdentityUseCase: &fakeIdentityUseCase{}, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+partyID.String()+"/identity-documents",
		[]byte(`{"document_type":"PASSPORT","issuing_country_code":"VE","issuing_jurisdiction_id":"bad","document_number":"123"}`), token)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestIdentityHandlerRequiresSession(t *testing.T) {
	manager := testAuthManager(t)
	router := NewRouter(Dependencies{IdentityUseCase: &fakeIdentityUseCase{}, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/parties/"+uuid.New().String()+"/nationalities",
		[]byte(`{"country_code":"VE"}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}
