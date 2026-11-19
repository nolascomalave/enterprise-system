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

type DashboardUseCase interface {
	GetDashboardSummary(ctx context.Context, tenantID uuid.UUID) (*domain.DashboardSummary, error)
}

type DashboardHandler struct {
	uc DashboardUseCase
}

func NewDashboardHandler(uc DashboardUseCase) *DashboardHandler {
	return &DashboardHandler{uc: uc}
}

// GetSummary godoc
// @Summary      Get the tenant dashboard summary
// @Description  Returns active party, employee and department counts plus the five most recent active parties.
// @Tags         Dashboard
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.DashboardSummaryEnvelope
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /dashboard/summary [get]
func (h *DashboardHandler) GetSummary(c *gin.Context) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sc.TenantID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	summary, err := h.uc.GetDashboardSummary(c.Request.Context(), sc.TenantID)
	if err != nil {
		c.Error(err)
		return
	}

	recent := make([]dto.DashboardRecentPartyResponse, 0, len(summary.RecentParties))
	for _, party := range summary.RecentParties {
		recent = append(recent, dto.DashboardRecentPartyResponse{
			ID:        party.ID.String(),
			Name:      party.Name,
			PartyType: string(party.PartyType),
			CreatedAt: party.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.DashboardSummaryEnvelope{Data: dto.DashboardSummaryResponse{
		TotalParties:     summary.TotalParties,
		TotalEmployees:   summary.TotalEmployees,
		TotalDepartments: summary.TotalDepartments,
		RecentParties:    recent,
	}})
}
