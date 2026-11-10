package http

import (
	"context"
	"net/http"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAccountUseCase struct {
	changeErr   error
	forgotErr   error
	resetErr    error
	changeInput usecase.ChangePasswordInput
	forgotInput usecase.ForgotPasswordInput
	resetInput  usecase.ResetPasswordInput
}

func (f *fakeAccountUseCase) ChangePassword(ctx context.Context, in usecase.ChangePasswordInput) error {
	f.changeInput = in
	return f.changeErr
}

func (f *fakeAccountUseCase) ForgotPassword(ctx context.Context, in usecase.ForgotPasswordInput) error {
	f.forgotInput = in
	return f.forgotErr
}

func (f *fakeAccountUseCase) ResetPassword(ctx context.Context, in usecase.ResetPasswordInput) error {
	f.resetInput = in
	return f.resetErr
}

func accountRouter(t *testing.T, uc *fakeAccountUseCase, limit int) *gin.Engine {
	t.Helper()
	return NewRouter(Dependencies{
		AccountUseCase: uc, TokenManager: testAuthManager(t),
		AuthRateLimit: limit, AuthRateWindow: time.Minute,
	})
}

func TestChangePassword(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	manager := testAuthManager(t)
	uc := &fakeAccountUseCase{}
	router := NewRouter(Dependencies{AccountUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, tenantID, userID, sessionID)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/change-password",
		[]byte(`{"current_password":"current-password","new_password":"new-password-123"}`), token)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, tenantID, uc.changeInput.TenantID)
	assert.Equal(t, userID, uc.changeInput.PartyID)
	assert.Equal(t, sessionID, uc.changeInput.SessionID)
	assert.Equal(t, "current-password", uc.changeInput.CurrentPassword)
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	uc := &fakeAccountUseCase{}
	router := accountRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/change-password",
		[]byte(`{"current_password":"current-password","new_password":"new-password-123"}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestChangePasswordRejectsWrongCurrent(t *testing.T) {
	manager := testAuthManager(t)
	uc := &fakeAccountUseCase{changeErr: auth.ErrInvalidCurrentPassword}
	router := NewRouter(Dependencies{AccountUseCase: uc, TokenManager: manager})
	token := accessToken(t, manager, uuid.New(), uuid.New(), uuid.New())

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/change-password",
		[]byte(`{"current_password":"wrong","new_password":"new-password-123"}`), token)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestForgotPasswordAlwaysAccepted(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeAccountUseCase{}
	router := accountRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/forgot-password",
		[]byte(`{"tenant_id":"`+tenantID.String()+`","email":"user@example.com"}`), "")

	require.Equal(t, http.StatusAccepted, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Contains(t, data["message"], "if the email exists")
	assert.Equal(t, tenantID, uc.forgotInput.TenantID)
	assert.Equal(t, "user@example.com", uc.forgotInput.Email)
}

func TestForgotPasswordRateLimit(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeAccountUseCase{}
	router := accountRouter(t, uc, 2)
	body := []byte(`{"tenant_id":"` + tenantID.String() + `","email":"user@example.com"}`)

	for i := 0; i < 2; i++ {
		recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/forgot-password", body, "")
		require.Equal(t, http.StatusAccepted, recorder.Code)
	}
	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/forgot-password", body, "")
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
}

func TestResetPassword(t *testing.T) {
	uc := &fakeAccountUseCase{}
	router := accountRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/reset-password",
		[]byte(`{"token":"reset-token","new_password":"new-password-123"}`), "")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "reset-token", uc.resetInput.Token)
	assert.Equal(t, "new-password-123", uc.resetInput.NewPassword)
}

func TestResetPasswordInvalidToken(t *testing.T) {
	uc := &fakeAccountUseCase{resetErr: auth.ErrInvalidResetToken}
	router := accountRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/reset-password",
		[]byte(`{"token":"bad-token","new_password":"new-password-123"}`), "")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
