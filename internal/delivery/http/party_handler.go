package http

import (
	"context"
	"fmt"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PartyUseCase interface {
	Create(ctx context.Context, in usecase.CreatePartyInput) (*domain.PartyDetail, error)
	Get(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyDetail, error)
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Party, error)
}

type PartyHandler struct {
	uc PartyUseCase
}

func NewPartyHandler(uc PartyUseCase) *PartyHandler {
	return &PartyHandler{uc: uc}
}

func (h *PartyHandler) Create(c *gin.Context) {
	var request dto.CreatePartyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", domain.ErrInvalidParty, err))
		return
	}

	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(ErrUnauthorized)
		return
	}

	input := usecase.CreatePartyInput{
		TenantID:    sessionContext.TenantID,
		PartyType:   domain.PartyType(request.PartyType),
		DisplayName: request.DisplayName,
	}
	if request.PersonProfile != nil {
		input.Person = &usecase.CreatePersonProfileInput{
			FirstName:      request.PersonProfile.FirstName,
			MiddleName:     request.PersonProfile.MiddleName,
			LastName:       request.PersonProfile.LastName,
			SecondLastName: request.PersonProfile.SecondLastName,
			BirthDate:      request.PersonProfile.BirthDate,
		}
	}
	if request.OrganizationProfile != nil {
		input.Organization = &usecase.CreateOrganizationProfileInput{
			LegalName: request.OrganizationProfile.LegalName,
			TradeName: request.OrganizationProfile.TradeName,
			TaxID:     request.OrganizationProfile.TaxID,
		}
	}

	detail, err := h.uc.Create(c.Request.Context(), input)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewPartyResponse(detail)})
}

func (h *PartyHandler) GetByID(c *gin.Context) {
	partyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Error(fmt.Errorf("%w: invalid party id", domain.ErrInvalidParty))
		return
	}

	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(ErrUnauthorized)
		return
	}

	detail, err := h.uc.Get(c.Request.Context(), sessionContext.TenantID, partyID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewPartyResponse(detail)})
}

func (h *PartyHandler) List(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(ErrUnauthorized)
		return
	}

	parties, err := h.uc.List(c.Request.Context(), sessionContext.TenantID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewPartyListResponse(parties)})
}
