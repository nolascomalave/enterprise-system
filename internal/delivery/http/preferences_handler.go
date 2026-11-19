package http

import (
	"context"
	"net/http"
	"strings"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PreferencesUseCase interface {
	GetUserPreferences(ctx context.Context, tenantID, userID, appClientID uuid.UUID) (*domain.UserPreferences, error)
	UpdateUserPreferences(ctx context.Context, tenantID, userID, appClientID uuid.UUID, preferences map[string]any) error
}

type PreferencesHandler struct {
	uc PreferencesUseCase
}

func NewPreferencesHandler(uc PreferencesUseCase) *PreferencesHandler {
	return &PreferencesHandler{uc: uc}
}

func (h *PreferencesHandler) session(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sc.TenantID == uuid.Nil || sc.UserID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}

func appClientIDFromRequest(c *gin.Context) (uuid.UUID, error) {
	if id, ok := AppClientIDFromContext(c.Request.Context()); ok && id != uuid.Nil {
		return id, nil
	}
	raw := strings.TrimSpace(c.GetHeader("X-App-Client-ID"))
	if raw == "" {
		return uuid.Nil, apperror.NewValidation("missing_app_client", "X-App-Client-ID header is required", nil)
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.NewValidation("invalid_app_client", "X-App-Client-ID must be a valid UUID", err)
	}
	return id, nil
}

// Get godoc
// @Summary      Get the current user's UI preferences
// @Description  Returns the preferences for the authenticated user and the X-App-Client-ID application. Defaults are returned without persistence when none exist.
// @Tags         Preferences
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Success      200 {object} dto.PreferencesEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /users/me/preferences [get]
func (h *PreferencesHandler) Get(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	appClientID, err := appClientIDFromRequest(c)
	if err != nil {
		c.Error(err)
		return
	}
	prefs, err := h.uc.GetUserPreferences(c.Request.Context(), sc.TenantID, sc.UserID, appClientID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.PreferencesEnvelope{
		Data: dto.PreferencesResponse{Preferences: prefs.Preferences},
	})
}

// Update godoc
// @Summary      Update the current user's UI preferences
// @Description  Replaces the preferences JSON object for the authenticated user and the X-App-Client-ID application.
// @Tags         Preferences
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        request body dto.UpdatePreferencesRequest true "Preferences object"
// @Success      204
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /users/me/preferences [put]
func (h *PreferencesHandler) Update(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	appClientID, err := appClientIDFromRequest(c)
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	if err := h.uc.UpdateUserPreferences(c.Request.Context(), sc.TenantID, sc.UserID, appClientID, request.Preferences); err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
