package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuthUseCase struct {
	loginResult   *usecase.LoginResult
	verifyResult  *usecase.LoginResult
	refreshResult *usecase.RefreshResult
	err           error
	loginInput    usecase.LoginInput
	verifyInput   usecase.Verify2FAInput
	refreshToken  string
	logoutToken   string
	logoutCalled  bool
}

func (f *fakeAuthUseCase) Login(ctx context.Context, in usecase.LoginInput) (*usecase.LoginResult, error) {
	f.loginInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.loginResult, nil
}

func (f *fakeAuthUseCase) Verify2FA(ctx context.Context, in usecase.Verify2FAInput) (*usecase.LoginResult, error) {
	f.verifyInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.verifyResult, nil
}

func (f *fakeAuthUseCase) Refresh(ctx context.Context, refreshToken string, metadata map[string]any) (*usecase.RefreshResult, error) {
	f.refreshToken = refreshToken
	if f.err != nil {
		return nil, f.err
	}
	return f.refreshResult, nil
}

func (f *fakeAuthUseCase) Logout(ctx context.Context, refreshToken string) error {
	f.logoutCalled = true
	f.logoutToken = refreshToken
	return f.err
}

func authRouter(t *testing.T, uc *fakeAuthUseCase, limit int) *gin.Engine {
	t.Helper()
	manager := testAuthManager(t)
	return NewRouter(Dependencies{
		AuthUseCase: uc, TokenManager: manager, CookieSecure: false,
		AuthRateLimit: limit, AuthRateWindow: time.Minute,
	})
}

func TestLoginSetsRefreshCookie(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeAuthUseCase{loginResult: &usecase.LoginResult{
		AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 900,
	}}
	router := authRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"tenant_id":"`+tenantID.String()+`","email":"user@example.com","password":"password123"}`), "")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "access-token", data["access_token"])
	assert.Equal(t, "Bearer", data["token_type"])

	cookies := recorder.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "refresh_token", cookies[0].Name)
	assert.True(t, cookies[0].HttpOnly)
	assert.Equal(t, "refresh-token", cookies[0].Value)
	assert.Equal(t, tenantID, uc.loginInput.TenantID)
}

func TestLoginRequires2FA(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeAuthUseCase{loginResult: &usecase.LoginResult{
		Requires2FA: true, TempToken: "temp-token",
	}}
	router := authRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"tenant_id":"`+tenantID.String()+`","email":"user@example.com","password":"password123"}`), "")

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["requires_2fa"])
	assert.Equal(t, "temp-token", data["temp_token"])
	assert.Empty(t, recorder.Result().Cookies())
}

func TestLoginInvalidCredentials(t *testing.T) {
	uc := &fakeAuthUseCase{err: auth.ErrInvalidCredentials}
	router := authRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"tenant_id":"`+uuid.New().String()+`","email":"user@example.com","password":"wrong"}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "unauthorized", body["code"])
}

func TestVerify2FASetsRefreshCookie(t *testing.T) {
	uc := &fakeAuthUseCase{verifyResult: &usecase.LoginResult{
		AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 900,
	}}
	router := authRouter(t, uc, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/2fa/verify",
		[]byte(`{"temp_token":"temp-token","code":"123456"}`), "")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotEmpty(t, recorder.Result().Cookies())
	assert.Equal(t, "temp-token", uc.verifyInput.TempToken)
	assert.Equal(t, "123456", uc.verifyInput.Code)
}

func TestRefreshRotatesCookie(t *testing.T) {
	uc := &fakeAuthUseCase{refreshResult: &usecase.RefreshResult{
		AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 900,
	}}
	router := authRouter(t, uc, 5)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "old-refresh"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "old-refresh", uc.refreshToken)
	cookies := recorder.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "new-refresh", cookies[0].Value)
}

func TestRefreshWithoutCookie(t *testing.T) {
	router := authRouter(t, &fakeAuthUseCase{}, 5)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/refresh", []byte(`{}`), "")

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestLogoutClearsCookie(t *testing.T) {
	uc := &fakeAuthUseCase{}
	router := authRouter(t, uc, 5)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "refresh-token"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.True(t, uc.logoutCalled)
	assert.Equal(t, "refresh-token", uc.logoutToken)
	cookies := recorder.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "refresh_token", cookies[0].Name)
	assert.LessOrEqual(t, cookies[0].MaxAge, 0)
}

func TestAuthRateLimit(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeAuthUseCase{loginResult: &usecase.LoginResult{
		AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresIn: 900,
	}}
	router := authRouter(t, uc, 2)
	body := []byte(`{"tenant_id":"` + tenantID.String() + `","email":"user@example.com","password":"password123"}`)

	for i := 0; i < 2; i++ {
		recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/login", body, "")
		require.Equal(t, http.StatusOK, recorder.Code)
	}

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/auth/login", body, "")
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
	bodyJSON := decodeBody(t, recorder)
	assert.Equal(t, "rate_limited", bodyJSON["code"])
	assert.NotEmpty(t, recorder.Header().Get("Retry-After"))
}
