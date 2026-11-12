package http

import (
	"context"
	"net/http"
	"strings"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DepartmentMDMUseCase interface {
	AddContact(ctx context.Context, in usecase.AddDepartmentContactInput) (*usecase.DepartmentContactResult, error)
	AddAddress(ctx context.Context, in usecase.AddDepartmentAddressInput) (*usecase.DepartmentAddressResult, error)
}

type DepartmentMDMHandler struct {
	uc DepartmentMDMUseCase
}

func NewDepartmentMDMHandler(uc DepartmentMDMUseCase) *DepartmentMDMHandler {
	return &DepartmentMDMHandler{uc: uc}
}

// AddContact godoc
// @Summary      Add a department contact
// @Description  Creates a department contact and opens its initial validity. Only one primary contact per type is allowed.
// @Tags         Department MDM
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Department UUID"
// @Param        request body dto.AddDepartmentContactRequest true "Contact data"
// @Success      201 {object} dto.DepartmentContactEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /departments/{id}/contacts [post]
func (h *DepartmentMDMHandler) AddContact(c *gin.Context) {
	sc, ok := departmentMDMSession(c)
	if !ok {
		return
	}
	departmentID, err := parseRequiredUUID(c.Param("id"), "department_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AddDepartmentContactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	result, err := h.uc.AddContact(c.Request.Context(), usecase.AddDepartmentContactInput{
		TenantID: sc.TenantID, DepartmentID: departmentID,
		Type:  domain.ContactType(strings.ToUpper(strings.TrimSpace(request.Type))),
		Value: request.Value, Order: request.Order, IsPrimary: request.IsPrimary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.DepartmentContactEnvelope{
		Data: dto.NewDepartmentContactResponse(result.Contact, result.Relation, result.Validity),
	})
}

// AddAddress godoc
// @Summary      Add a department address
// @Description  Creates a department address and opens its initial validity. Only one primary address per type is allowed.
// @Tags         Department MDM
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Department UUID"
// @Param        request body dto.AddDepartmentAddressRequest true "Address data"
// @Success      201 {object} dto.DepartmentAddressEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /departments/{id}/addresses [post]
func (h *DepartmentMDMHandler) AddAddress(c *gin.Context) {
	sc, ok := departmentMDMSession(c)
	if !ok {
		return
	}
	departmentID, err := parseRequiredUUID(c.Param("id"), "department_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AddDepartmentAddressRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	cityID, err := uuid.Parse(strings.TrimSpace(request.CityLocationID))
	if err != nil || cityID == uuid.Nil {
		c.Error(badRequestError(err))
		return
	}
	result, err := h.uc.AddAddress(c.Request.Context(), usecase.AddDepartmentAddressInput{
		TenantID: sc.TenantID, DepartmentID: departmentID, CityLocationID: cityID,
		Type:  domain.AddressType(strings.ToUpper(strings.TrimSpace(request.Type))),
		Line1: request.Line1, Line2: request.Line2, IsPrimary: request.IsPrimary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.DepartmentAddressEnvelope{
		Data: dto.NewDepartmentAddressResponse(result.Address, result.Validity),
	})
}

func departmentMDMSession(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}
