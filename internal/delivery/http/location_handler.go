package http

import (
	"context"
	"net/http"
	"strings"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type LocationUseCase interface {
	List(ctx context.Context, countryCode string, parentID *uuid.UUID) ([]*domain.Location, error)
}

type LocationHandler struct {
	uc LocationUseCase
}

func NewLocationHandler(uc LocationUseCase) *LocationHandler {
	return &LocationHandler{uc: uc}
}

// List godoc
// @Summary      List locations
// @Description  Returns the children of parent_id (or country roots) from the global geographic hierarchy.
// @Tags         Locations
// @Produce      json
// @Param        country query string true "ISO 3166-1 alpha-2 country code"
// @Param        parent_id query string false "Parent location UUID"
// @Success      200 {object} dto.LocationListEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /locations [get]
func (h *LocationHandler) List(c *gin.Context) {
	country := strings.ToUpper(strings.TrimSpace(c.Query("country")))
	if country == "" {
		c.Error(apperror.NewValidation("invalid_country", "country is required", nil))
		return
	}
	var parentID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("parent_id")); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil || parsed == uuid.Nil {
			c.Error(apperror.NewValidation("invalid_parent_id", "invalid parent_id", err))
			return
		}
		parentID = &parsed
	}

	locations, err := h.uc.List(c.Request.Context(), country, parentID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.LocationListEnvelope{Data: dto.NewLocationListResponse(locations)})
}
