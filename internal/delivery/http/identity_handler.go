package http

import (
	"context"
	"net/http"
	"strings"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type IdentityUseCase interface {
	AddNationality(ctx context.Context, in usecase.AddPartyNationalityInput) (*domain.PartyNationality, error)
	AddIdentityDocument(ctx context.Context, in usecase.AddPartyIdentityDocumentInput) (*usecase.IdentityDocumentResult, error)
}

type IdentityHandler struct {
	uc IdentityUseCase
}

func NewIdentityHandler(uc IdentityUseCase) *IdentityHandler {
	return &IdentityHandler{uc: uc}
}

// AddNationality godoc
// @Summary      Add a party nationality
// @Description  Adds a nationality for the party. Only one nationality can be primary.
// @Tags         Party Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Party UUID"
// @Param        request body dto.AddPartyNationalityRequest true "Nationality data"
// @Success      201 {object} dto.PartyNationalityEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /parties/{id}/nationalities [post]
func (h *IdentityHandler) AddNationality(c *gin.Context) {
	sc, ok := identitySession(c)
	if !ok {
		return
	}
	partyID, err := parseRequiredUUID(c.Param("id"), "party_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AddPartyNationalityRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	nationality, err := h.uc.AddNationality(c.Request.Context(), usecase.AddPartyNationalityInput{
		TenantID: sc.TenantID, PartyID: partyID,
		CountryCode: request.CountryCode, IsPrimary: request.IsPrimary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.PartyNationalityEnvelope{
		Data: dto.NewPartyNationalityResponse(nationality),
	})
}

// AddIdentityDocument godoc
// @Summary      Add a party identity document
// @Description  Registers an identity document and opens its initial validity. The jurisdiction, if provided, must belong to the issuing country.
// @Tags         Party Identity
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Party UUID"
// @Param        request body dto.AddPartyIdentityDocumentRequest true "Identity document data"
// @Success      201 {object} dto.PartyIdentityDocumentEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /parties/{id}/identity-documents [post]
func (h *IdentityHandler) AddIdentityDocument(c *gin.Context) {
	sc, ok := identitySession(c)
	if !ok {
		return
	}
	partyID, err := parseRequiredUUID(c.Param("id"), "party_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AddPartyIdentityDocumentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	var jurisdictionID *uuid.UUID
	if request.IssuingJurisdictionID != nil && strings.TrimSpace(*request.IssuingJurisdictionID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.IssuingJurisdictionID))
		if err != nil || parsed == uuid.Nil {
			c.Error(apperror.NewValidation("invalid_jurisdiction", "invalid issuing_jurisdiction_id", err))
			return
		}
		jurisdictionID = &parsed
	}
	result, err := h.uc.AddIdentityDocument(c.Request.Context(), usecase.AddPartyIdentityDocumentInput{
		TenantID: sc.TenantID, PartyID: partyID,
		DocumentType: request.DocumentType, IssuingCountryCode: request.IssuingCountryCode,
		IssuingJurisdictionID: jurisdictionID, DocumentNumber: request.DocumentNumber,
		IsPrimary: request.IsPrimary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.PartyIdentityDocumentEnvelope{
		Data: dto.NewPartyIdentityDocumentResponse(result.Document, result.Validity, result.TypeCode),
	})
}

func identitySession(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}
