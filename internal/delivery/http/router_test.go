package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type fakePartyUseCase struct {
	detail       *domain.PartyDetail
	list         []*domain.Party
	createErr    error
	getErr       error
	listErr      error
	createInput  usecase.CreatePartyInput
	gotTenant    uuid.UUID
	gotPartyID   uuid.UUID
	gotSession   enttenant.SessionContext
	gotSessionOK bool
}

func (f *fakePartyUseCase) Create(ctx context.Context, in usecase.CreatePartyInput) (*domain.PartyDetail, error) {
	f.createInput = in
	f.gotSession, f.gotSessionOK = enttenant.SessionContextFrom(ctx)
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.detail, nil
}

func (f *fakePartyUseCase) Get(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyDetail, error) {
	f.gotTenant = tenantID
	f.gotPartyID = partyID
	f.gotSession, f.gotSessionOK = enttenant.SessionContextFrom(ctx)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.detail, nil
}

func (f *fakePartyUseCase) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Party, error) {
	f.gotTenant = tenantID
	f.gotSession, f.gotSessionOK = enttenant.SessionContextFrom(ctx)
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func testAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	manager, err := auth.NewManager(auth.Config{
		Secret:     []byte("01234567890123456789012345678901"),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		TempTTL:    5 * time.Minute,
		BcryptCost: 4,
		Now:        time.Now,
	})
	require.NoError(t, err)
	return manager
}

func accessToken(t *testing.T, manager *auth.Manager, tenantID, userID, sessionID uuid.UUID) string {
	t.Helper()
	token, err := manager.IssueAccessToken(tenantID, userID, sessionID)
	require.NoError(t, err)
	return token
}

func testPartyDetail(tenantID uuid.UUID) *domain.PartyDetail {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	party := &domain.Party{
		ID: uuid.New(), TenantID: tenantID, PartyType: domain.PartyTypeIndividual,
		DisplayName: "Jane Doe", RecordStatus: domain.PartyRecordStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	person := &domain.PersonProfile{
		ID: uuid.New(), TenantID: tenantID, PartyID: party.ID,
		FirstName: "Jane", LastName: "Doe", CreatedAt: now, UpdatedAt: now,
	}
	return &domain.PartyDetail{Party: party, PersonProfile: person}
}

func requestWithToken(router *gin.Engine, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

func TestCreateParty(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	manager := testAuthManager(t)
	uc := &fakePartyUseCase{detail: testPartyDetail(tenantID)}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, userID, sessionID)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties",
		[]byte(`{"party_type":"INDIVIDUAL","display_name":"Jane Doe","person_profile":{"first_name":"Jane","last_name":"Doe"}}`),
		token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "Jane Doe", data["display_name"])
	assert.Equal(t, "INDIVIDUAL", data["party_type"])
	person := data["person_profile"].(map[string]any)
	assert.Equal(t, "Jane", person["first_name"])

	assert.Equal(t, tenantID, uc.createInput.TenantID)
	assert.True(t, uc.gotSessionOK)
	assert.Equal(t, tenantID, uc.gotSession.TenantID)
	assert.Equal(t, userID, uc.gotSession.UserID)
	assert.Equal(t, sessionID, uc.gotSession.SessionID)
}

func TestCreatePartyRequiresBearerToken(t *testing.T) {
	manager := testAuthManager(t)
	router := NewRouter(Dependencies{PartyUseCase: &fakePartyUseCase{detail: testPartyDetail(uuid.New())}, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties", []byte(`{}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "unauthorized", body["code"])
}

func TestCreatePartyRejectsInvalidToken(t *testing.T) {
	manager := testAuthManager(t)
	router := NewRouter(Dependencies{PartyUseCase: &fakePartyUseCase{detail: testPartyDetail(uuid.New())}, TokenManager: manager})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties", []byte(`{}`), "invalid-token")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestCreatePartyInvalidBody(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	router := NewRouter(Dependencies{PartyUseCase: &fakePartyUseCase{detail: testPartyDetail(tenantID)}, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties",
		[]byte(`{"party_type":"OTHER","display_name":"Jane"}`), token)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "bad_request", body["code"])
}

func TestGetParty(t *testing.T) {
	tenantID := uuid.New()
	detail := testPartyDetail(tenantID)
	manager := testAuthManager(t)
	uc := &fakePartyUseCase{detail: detail}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/parties/"+detail.Party.ID.String(), nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.gotTenant)
	assert.Equal(t, detail.Party.ID, uc.gotPartyID)
	assert.True(t, uc.gotSessionOK)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, detail.Party.ID.String(), data["id"])
}

func TestGetPartyInvalidID(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	router := NewRouter(Dependencies{PartyUseCase: &fakePartyUseCase{detail: testPartyDetail(tenantID)}, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/parties/not-a-uuid", nil, token)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetPartyNotFound(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	uc := &fakePartyUseCase{getErr: domain.ErrPartyNotFound}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/parties/"+uuid.New().String(), nil, token)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "not_found", body["code"])
}

func TestListParties(t *testing.T) {
	tenantID := uuid.New()
	party := testPartyDetail(tenantID).Party
	manager := testAuthManager(t)
	uc := &fakePartyUseCase{list: []*domain.Party{party}}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/parties", nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, tenantID, uc.gotTenant)
	body := decodeBody(t, recorder)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, party.ID.String(), data[0].(map[string]any)["id"])
}

func TestErrorMappingConflict(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	uc := &fakePartyUseCase{createErr: domain.ErrAlreadyDeleted}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties",
		[]byte(`{"party_type":"INDIVIDUAL","display_name":"Jane","person_profile":{"first_name":"Jane","last_name":"Doe"}}`),
		token)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "conflict", body["code"])
}

func TestErrorMappingInternal(t *testing.T) {
	manager := testAuthManager(t)
	tenantID := uuid.New()
	uc := &fakePartyUseCase{createErr: errors.New("database exploded")}
	router := NewRouter(Dependencies{PartyUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties",
		[]byte(`{"party_type":"INDIVIDUAL","display_name":"Jane","person_profile":{"first_name":"Jane","last_name":"Doe"}}`),
		token)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "internal_error", body["code"])
	assert.Equal(t, "internal server error", body["error"])
}
