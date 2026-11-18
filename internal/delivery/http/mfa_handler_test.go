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

	setupResult *usecase.SetupTOTPResult
	setupErr    error
	setupCalls  int
	setupTenant uuid.UUID
	setupParty  uuid.UUID

	verifyResult *usecase.VerifyTOTPResult
	verifyErr    error
	verifyCalls  int
	verifyTenant uuid.UUID
	verifyParty  uuid.UUID
	verifyCode   string
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

func (f *fakeMFAUseCase) SetupTOTP(ctx context.Context, tenantID, partyID uuid.UUID) (*usecase.SetupTOTPResult, error) {
	f.setupCalls++
	f.setupTenant, f.setupParty = tenantID, partyID
	return f.setupResult, f.setupErr
}

func (f *fakeMFAUseCase) VerifyTOTP(ctx context.Context, tenantID, partyID uuid.UUID, code string) (*usecase.VerifyTOTPResult, error) {
	f.verifyCalls++
	f.verifyTenant, f.verifyParty, f.verifyCode = tenantID, partyID, code
	return f.verifyResult, f.verifyErr
}

func mfaTestRouter(uc MFAUseCase, sc enttenant.SessionContext) *gin.Engine {
	router := gin.New()
	router.Use(ErrorMiddleware())
	handler := NewMFAHandler(uc)
	group := router.Group("")
	group.Use(sessionContextMiddleware(sc))
	group.PUT("/api/v1/auth/mfa-preferences", handler.UpdatePreferences)
	group.POST("/api/v1/auth/request-otp", handler.RequestSecurityCode)
	group.POST("/api/v1/auth/totp/setup", handler.SetupTOTP)
	group.POST("/api/v1/auth/totp/verify", handler.VerifyTOTP)
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

func TestSetupTOTPEndpoint(t *testing.T) {
	uc := &fakeMFAUseCase{setupResult: &usecase.SetupTOTPResult{
		Secret: "JBSWY3DPEHPK3PXP", OTPAuthURL: "otpauth://totp/enterprise-system:user@example.com?secret=JBSWY3DPEHPK3PXP",
	}}
	tenantID, partyID := uuid.New(), uuid.New()
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: partyID})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/setup", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", data["secret"])
	assert.Contains(t, data["otpauth_url"], "otpauth://")
	require.Equal(t, 1, uc.setupCalls)
	assert.Equal(t, tenantID, uc.setupTenant)
	assert.Equal(t, partyID, uc.setupParty)
}

func TestVerifyTOTPEndpoint(t *testing.T) {
	uc := &fakeMFAUseCase{verifyResult: &usecase.VerifyTOTPResult{BackupCodes: []string{
		"A1B2-C3D4", "E5F6-G7H8", "J9K2-L3M4", "N5P6-Q7R8",
		"S9T2-U3V4", "W5X6-Y7Z8", "A2B3-C4D5", "E6F7-G8H9",
	}}}
	tenantID, partyID := uuid.New(), uuid.New()
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: tenantID, UserID: partyID})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/verify",
		bytes.NewBufferString(`{"code":"123456"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, uc.verifyCalls)
	assert.Equal(t, tenantID, uc.verifyTenant)
	assert.Equal(t, partyID, uc.verifyParty)
	assert.Equal(t, "123456", uc.verifyCode)

	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	codes := data["backup_codes"].([]any)
	assert.Len(t, codes, 8)
	assert.Equal(t, "A1B2-C3D4", codes[0])
}

func TestUpdateMFAPreferencesEndpointForwardsOTP(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPut, "/api/v1/auth/mfa-preferences",
		bytes.NewBufferString(`{"enabled":false,"otp_code":"123456"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "123456", uc.updateInput.OTPCode)
}

func TestVerifyTOTPEndpointRequiresCode(t *testing.T) {
	uc := &fakeMFAUseCase{}
	router := mfaTestRouter(uc, enttenant.SessionContext{TenantID: uuid.New(), UserID: uuid.New()})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/verify",
		bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 0, uc.verifyCalls)
}
