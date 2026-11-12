package http

import (
	"context"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MergePartiesUseCase interface {
	Execute(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.MergeResult, error)
}

type MergeHandler struct {
	uc MergePartiesUseCase
}

func NewMergeHandler(uc MergePartiesUseCase) *MergeHandler {
	return &MergeHandler{uc: uc}
}

// Merge godoc
// @Summary      Merge a party into a Golden Record
// @Description  Marks the source party as MERGED, points it to the target and compresses dependent paths.
// @Tags         Parties
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.MergePartiesRequest true "Source and target party IDs"
// @Success      200 {object} dto.MergePartiesEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /parties/merge [post]
func (h *MergeHandler) Merge(c *gin.Context) {
	var request dto.MergePartiesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	sourceID, err := parseRequiredUUID(request.SourcePartyID, "source_party_id")
	if err != nil {
		c.Error(err)
		return
	}
	targetID, err := parseRequiredUUID(request.TargetPartyID, "target_party_id")
	if err != nil {
		c.Error(err)
		return
	}
	result, err := h.uc.Execute(c.Request.Context(), sourceID, targetID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewMergePartiesResponse(result)})
}
