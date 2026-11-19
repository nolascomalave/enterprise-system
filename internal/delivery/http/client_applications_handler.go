package http

import (
	"context"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ClientApplicationsUseCase interface {
	ListClientApplications(ctx context.Context, tenantID uuid.UUID) ([]*domain.ClientApplication, error)
}

type ClientApplicationsHandler struct {
	uc ClientApplicationsUseCase
}

func NewClientApplicationsHandler(uc ClientApplicationsUseCase) *ClientApplicationsHandler {
	return &ClientApplicationsHandler{uc: uc}
}

// List godoc
// @Summary      List tenant client applications
// @Description  Returns the client applications owned by the current tenant. Inactive applications are included; deleted ones are excluded.
// @Tags         OAuth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.ClientApplicationListEnvelope
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /client-applications [get]
func (h *ClientApplicationsHandler) List(c *gin.Context) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sc.TenantID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	applications, err := h.uc.ListClientApplications(c.Request.Context(), sc.TenantID)
	if err != nil {
		c.Error(err)
		return
	}

	response := make([]dto.ClientApplicationListItemResponse, 0, len(applications))
	for _, application := range applications {
		response = append(response, dto.NewClientApplicationListItemResponse(application))
	}
	c.JSON(http.StatusOK, dto.ClientApplicationListEnvelope{Data: response})
}
