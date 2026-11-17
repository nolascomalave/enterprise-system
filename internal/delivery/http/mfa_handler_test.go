package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMFAUseCase struct {
	updateInput  usecase.UpdateMFAPreferencesInput
	requestInput usecase.RequestSecurityCodeInput
	updateErr    error
	requestErr   error
	updateCalls  int
	requestCalls int
}

func (f *fakeMFAUseCase) UpdateMFAPreferences(ctx context.Context, in usecase.UpdateMFAPreferencesInput) error {
	f.updateCalls++
	f.updateInput = in
	return f.updateErr
}

func (f *fakeMFAUseCase) RequestSecurityCode(ctx context.Context, in usecase.RequestSecurityCodeInput) error {
	f.requestCalls++
	f.requestInput = in
	return f.requestErr
}

func mfaTestRouter(uc MFAUseCase, sc enttenant.SessionContext) *gin.Engine {
	router := gin.New()
	router.Use(ErrorMiddleware())
	handler := NewMFAHandler(uc)
	group := router.Group("")
	group.Use(sessionContextMiddleware(sc))
	group.PUT("/api/v1/auth/mfa-preferences", handler.UpdatePreferences)
	group.POST("/api/v1/auth/request-otp", handler.RequestSecurityCode)
	return router
}

func TestUpdateMFAPreferencesEndpoint(t *testing.T) {
	uc := &fakeMFAUseCase{}
	tenantID, partyID := uuid.New(), uuid.New()
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: partyID})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/mfa-preferences",
		bytes.NewBufferString(`{"enabled":true,"method":"EMAIL_OTP"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, 1, uc.updateCalls)
	assert.Equal(t, tenantID, uc.updateInput.TenantID)
	assert.Equal(t, partyID, uc.updateInput.PartyID)
	assert.True(t, uc.updateInput.Enabled)
	assert.Equal(t, domain.MFAMethodEmailOTP, uc.updateInput.Method)
}

func TestUpdateMFAPreferencesEndpointNormalizesMethod(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/mfa-preferences",
		bytes.NewBufferString(`{"enabled":false}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.False(t, uc.updateInput.Enabled)
	assert.Equal(t, domain.MFAMethodNone, uc.updateInput.Method)
}

func TestUpdateMFAPreferencesEndpointRejectsInvalidBody(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/mfa-preferences",
		bytes.NewBufferString(`{"enabled":`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, uc.updateCalls)
}

func TestUpdateMFAPreferencesEndpointRequiresSession(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/mfa-preferences",
		bytes.NewBufferString(`{"enabled":true,"method":"EMAIL_OTP"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Equal(t, 0, uc.updateCalls)
}

func TestRequestSecurityCodeEndpoint(t *testing.T) {
	uc := &fakeMFAUseCase{}
	tenantID, partyID := uuid.New(), uuid.New()
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: partyID})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-otp",
		bytes.NewBufferString(`{"action":"CHANGE_PASSWORD"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, 1, uc.requestCalls)
	assert.Equal(t, tenantID, uc.requestInput.TenantID)
	assert.Equal(t, partyID, uc.requestInput.PartyID)
	assert.Equal(t, "CHANGE_PASSWORD", uc.requestInput.Action)
}

func TestRequestSecurityCodeEndpointMapsCooldown(t *testing.T) {
	uc := &fakeMFAUseCase{requestErr: apperror.NewValidation("otp_cooldown", "wait", domain.ErrOTPCooldown)}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-otp",
		bytes.NewBufferString(`{"action":"CHANGE_PASSWORD"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRequestSecurityCodeEndpointRequiresAction(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/request-otp",
		bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, uc.requestCalls)
}
