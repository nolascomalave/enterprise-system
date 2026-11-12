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

// RBACUseCase exposes RBAC operations to the HTTP layer.
type RBACUseCase interface {
	ResolvePermissions(ctx context.Context, input usecase.ResolvePermissionsInput) (*domain.PermissionUniverse, error)
}

// RBACHandler handles RBAC-related HTTP requests.
type RBACHandler struct {
	uc RBACUseCase
}

// NewRBACHandler constructs an RBACHandler.
func NewRBACHandler(uc RBACUseCase) *RBACHandler {
	return &RBACHandler{uc: uc}
}

// MyPermissions godoc
// @Summary      List my permissions
// @Description  Returns the resolved universe of system function codes for the authenticated user and app.
// @Tags         RBAC
// @Produce      json
// @Security     BearerAuth
// @Param        app_client_id query string true "Client application UUID"
// @Success      200 {object} dto.PermissionUniverseEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /rbac/my-permissions [get]
func (h *RBACHandler) MyPermissions(c *gin.Context) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	rawAppClientID := strings.TrimSpace(c.Query("app_client_id"))
	if rawAppClientID == "" {
		c.Error(apperror.NewValidation("invalid_app_client", "app_client_id is required", nil))
		return
	}
	appClientID, err := uuid.Parse(rawAppClientID)
	if err != nil || appClientID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_app_client", "app_client_id must be a valid UUID", err))
		return
	}

	universe, err := h.uc.ResolvePermissions(c.Request.Context(), usecase.ResolvePermissionsInput{
		TenantID:          sc.TenantID,
		UserCredentialsID: sc.UserID,
		AppClientID:       appClientID,
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.PermissionUniverseEnvelope{
		Data: dto.PermissionUniverseResponse{Functions: universe.Functions},
	})
}
