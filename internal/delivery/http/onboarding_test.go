package http

import (
	"context"
	"net/http"
	"testing"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOnboardingUseCase struct {
	result *usecase.RegisterTenantResult
	err    error
	input  usecase.RegisterTenantInput
	calls  int
}

func (f *fakeOnboardingUseCase) RegisterTenant(ctx context.Context, in usecase.RegisterTenantInput) (*usecase.RegisterTenantResult, error) {
	f.calls++
	f.input = in
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func onboardingRouter(t *testing.T, uc *fakeOnboardingUseCase) *gin.Engine {
	t.Helper()
	return NewRouter(Dependencies{
		OnboardingUseCase: uc, TokenManager: testAuthManager(t),
		AuthRateLimit: 5,
	})
}

func registerBody() []byte {
	return []byte(`{"company_name":"Acme Corp","tax_id":"J-123","slug":"acme-corp","admin_first_name":"Jane","admin_last_name":"Doe","admin_email":"jane@example.com","admin_password":"password123"}`)
}

func TestRegisterTenant(t *testing.T) {
	uc := &fakeOnboardingUseCase{result: &usecase.RegisterTenantResult{
		TenantID: uuid.New(), OrganizationPartyID: uuid.New(), AdminPartyID: uuid.New(), Slug: "acme-corp",
	}}
	router := onboardingRouter(t, uc)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", registerBody(), "")

	require.Equal(t, http.StatusCreated, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "acme-corp", data["slug"])
	assert.Equal(t, uc.result.TenantID.String(), data["tenant_id"])
	assert.Equal(t, "Acme Corp", uc.input.CompanyName)
	assert.Equal(t, "jane@example.com", uc.input.AdminEmail)
}

func TestRegisterTenantRateLimit(t *testing.T) {
	uc := &fakeOnboardingUseCase{result: &usecase.RegisterTenantResult{TenantID: uuid.New()}}
	router := onboardingRouter(t, uc)

	for i := 0; i < 3; i++ {
		recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", registerBody(), "")
		require.Equal(t, http.StatusCreated, recorder.Code)
	}
	recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", registerBody(), "")
	assert.Equal(t, http.StatusTooManyRequests, recorder.Code)
}

func TestRegisterTenantInvalidBody(t *testing.T) {
	router := onboardingRouter(t, &fakeOnboardingUseCase{})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", []byte(`{}`), "")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRegisterTenantDuplicateSlug(t *testing.T) {
	uc := &fakeOnboardingUseCase{err: domain.ErrTenantSlugTaken}
	router := onboardingRouter(t, uc)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", registerBody(), "")

	assert.Equal(t, http.StatusConflict, recorder.Code)
	body := decodeBody(t, recorder)
	assert.Equal(t, "conflict", body["code"])
}

func TestRegisterTenantDuplicateEmail(t *testing.T) {
	uc := &fakeOnboardingUseCase{err: domain.ErrTenantAdminEmailTaken}
	router := onboardingRouter(t, uc)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/onboarding/register", registerBody(), "")

	assert.Equal(t, http.StatusConflict, recorder.Code)
}
