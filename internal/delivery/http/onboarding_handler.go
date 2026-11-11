package http

import (
	"context"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
)

type OnboardingUseCase interface {
	RegisterTenant(ctx context.Context, in usecase.RegisterTenantInput) (*usecase.RegisterTenantResult, error)
}

type OnboardingHandler struct {
	uc OnboardingUseCase
}

func NewOnboardingHandler(uc OnboardingUseCase) *OnboardingHandler {
	return &OnboardingHandler{uc: uc}
}

// Register godoc
// @Summary      Register a tenant and its first administrator
// @Description  Creates the subscription, organization party/profile, TENANT role, admin party/profile, ADMIN role and credentials in one transaction.
// @Tags         Onboarding
// @Accept       json
// @Produce      json
// @Param        request body dto.RegisterTenantRequest true "Tenant and administrator data"
// @Success      201 {object} dto.RegisterTenantEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      429 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /onboarding/register [post]
func (h *OnboardingHandler) Register(c *gin.Context) {
	var request dto.RegisterTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}

	result, err := h.uc.RegisterTenant(c.Request.Context(), usecase.RegisterTenantInput{
		CompanyName: request.CompanyName, TaxID: request.TaxID, Slug: request.Slug,
		AdminFirstName: request.AdminFirstName, AdminLastName: request.AdminLastName,
		AdminEmail: request.AdminEmail, AdminPassword: request.AdminPassword,
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": dto.RegisterTenantResponse{
		TenantID: result.TenantID.String(), OrganizationPartyID: result.OrganizationPartyID.String(),
		AdminPartyID: result.AdminPartyID.String(), Slug: result.Slug,
	}})
}
