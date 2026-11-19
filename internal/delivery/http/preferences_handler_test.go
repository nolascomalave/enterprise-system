package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePreferencesUseCase struct {
	getResult   *domain.UserPreferences
	getErr      error
	updateErr   error
	getInput    [3]uuid.UUID
	updatePrefs map[string]any
	updateInput [3]uuid.UUID
	getCalls    int
	updateCalls int
}

func (f *fakePreferencesUseCase) GetUserPreferences(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*domain.UserPreferences, error) {
	f.getCalls++
	f.getInput = [3]uuid.UUID{tenantID, userID, appClientID}
	return f.getResult, f.getErr
}

func (f *fakePreferencesUseCase) UpdateUserPreferences(ctx context.Context, tenantID, userID, appClientID uuid.UUID, prefs map[string]any) error {
	f.updateCalls++
	f.updateInput = [3]uuid.UUID{tenantID, userID, appClientID}
	f.updatePrefs = prefs
	return f.updateErr
}

func preferencesTestRouter(uc PreferencesUseCase, sc enttenant.SessionContext) *gin.Engine {
	router := gin.New()
	router.Use(ErrorMiddleware())
	handler := NewPreferencesHandler(uc)
	group := router.Group("")
	group.Use(sessionContextMiddleware(sc))
	group.GET("/api/v1/users/me/preferences", handler.Get)
	group.PUT("/api/v1/users/me/preferences", handler.Update)
	return router
}

func TestGetPreferencesEndpoint(t *testing.T) {
	tenantID, userID, appID := uuid.New(), uuid.New(), uuid.New()
	uc := &fakePreferencesUseCase{getResult: &domain.UserPreferences{
		TenantID: tenantID, ClientApplicationID: appID,
		Preferences: map[string]any{"theme": "dark"},
	}}
	router := preferencesTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: userID})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/preferences", nil)
	req.Header.Set("X-App-Client-ID", appID.String())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	prefs := data["preferences"].(map[string]any)
	assert.Equal(t, "dark", prefs["theme"])
	assert.Equal(t, [3]uuid.UUID{tenantID, userID, appID}, uc.getInput)
}

func TestUpdatePreferencesEndpoint(t *testing.T) {
	tenantID, userID, appID := uuid.New(), uuid.New(), uuid.New()
	uc := &fakePreferencesUseCase{}
	router := preferencesTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: userID})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/me/preferences",
		bytes.NewBufferString(`{"preferences":{"theme":"dark","sidebar_collapsed":true}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Client-ID", appID.String())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, 1, uc.updateCalls)
	assert.Equal(t, [3]uuid.UUID{tenantID, userID, appID}, uc.updateInput)
	assert.Equal(t, "dark", uc.updatePrefs["theme"])
	assert.Equal(t, true, uc.updatePrefs["sidebar_collapsed"])
}

func TestPreferencesEndpointRequiresAppClientID(t *testing.T) {
	uc := &fakePreferencesUseCase{}
	router := preferencesTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/preferences", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, uc.getCalls)
}

func TestPreferencesEndpointRequiresSession(t *testing.T) {
	uc := &fakePreferencesUseCase{}
	router := preferencesTestRouter(uc, enttenant.SessionContext{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/preferences", nil)
	req.Header.Set("X-App-Client-ID", uuid.New().String())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, 0, uc.getCalls)
}
