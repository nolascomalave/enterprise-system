package http

import (
	"context"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
)

type PartyNameUseCase interface {
	Add(ctx context.Context, in usecase.AddPartyNameInput) (*usecase.PartyNameResult, error)
	UpdateValidity(ctx context.Context, in usecase.UpdatePartyNameValidityInput) (*usecase.PartyNameResult, error)
}

type PartyNameHandler struct {
	uc PartyNameUseCase
}

func NewPartyNameHandler(uc PartyNameUseCase) *PartyNameHandler {
	return &PartyNameHandler{uc: uc}
}

// Create godoc
// @Summary      Add a party name
// @Description  Creates the name if needed and opens its initial validity. A single primary name per type is enforced.
// @Tags         Party Names
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Party UUID"
// @Param        request body dto.AddPartyNameRequest true "Name data"
// @Success      201 {object} dto.PartyNameEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /parties/{id}/names [post]
func (h *PartyNameHandler) Create(c *gin.Context) {
	sc, ok := partyNameSession(c)
	if !ok {
		return
	}
	partyID, err := parseRequiredUUID(c.Param("id"), "party_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AddPartyNameRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	result, err := h.uc.Add(c.Request.Context(), usecase.AddPartyNameInput{
		TenantID: sc.TenantID, PartyID: partyID, NameType: request.NameType,
		Value: request.Value, IsPrimary: request.IsPrimary, Ordering: request.Ordering,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.PartyNameEnvelope{
		Data: dto.NewPartyNameResponse(result.Name, result.Validity),
	})
}

// UpdateValidity godoc
// @Summary      Update a party name validity
// @Description  Closes the current validity and opens a new one for an existing or new value.
// @Tags         Party Names
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Party UUID"
// @Param        name_id path string true "Party name UUID"
// @Param        request body dto.UpdatePartyNameValidityRequest true "New validity data"
// @Success      200 {object} dto.PartyNameEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /parties/{id}/names/{name_id}/validity [put]
func (h *PartyNameHandler) UpdateValidity(c *gin.Context) {
	sc, ok := partyNameSession(c)
	if !ok {
		return
	}
	partyID, err := parseRequiredUUID(c.Param("id"), "party_id")
	if err != nil {
		c.Error(err)
		return
	}
	nameID, err := parseRequiredUUID(c.Param("name_id"), "name_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.UpdatePartyNameValidityRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	result, err := h.uc.UpdateValidity(c.Request.Context(), usecase.UpdatePartyNameValidityInput{
		TenantID: sc.TenantID, PartyID: partyID, PartyNameID: nameID,
		NewValue: request.NewValue, IsPrimary: request.IsPrimary, Ordering: request.Ordering,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.PartyNameEnvelope{
		Data: dto.NewPartyNameResponse(result.Name, result.Validity),
	})
}

func partyNameSession(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}
