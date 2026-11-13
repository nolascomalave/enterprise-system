package http

import (
	"context"
	"errors"
	"io"
	"net/http"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TenantRelationshipUseCase exposes the relationship flows to the HTTP layer.
type TenantRelationshipUseCase interface {
	StartPendingRelationship(ctx context.Context, input usecase.StartPendingRelationshipInput) (*domain.TenantRelationship, error)
	EmitInviteToken(ctx context.Context, input usecase.EmitInviteTokenInput) (*usecase.EmitInviteTokenResult, error)
	ConsumeInviteToken(ctx context.Context, input usecase.ConsumeInviteTokenInput) (*domain.TenantRelationship, error)
	RejectRelationship(ctx context.Context, input usecase.RejectRelationshipInput) (*domain.TenantRelationship, error)
	CancelPendingRelationship(ctx context.Context, input usecase.CancelPendingRelationshipInput) (*domain.TenantRelationship, error)
	RevokeApprovedRelationship(ctx context.Context, input usecase.RevokeApprovedRelationshipInput) (*domain.TenantRelationship, error)
}

type TenantRelationshipHandler struct {
	uc TenantRelationshipUseCase
}

func NewTenantRelationshipHandler(uc TenantRelationshipUseCase) *TenantRelationshipHandler {
	return &TenantRelationshipHandler{uc: uc}
}

func (h *TenantRelationshipHandler) session(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}

func relationshipResponse(r *domain.TenantRelationship) dto.TenantRelationshipResponse {
	return dto.TenantRelationshipResponse{
		ID:                 r.ID.String(),
		TenantIDHolding:    r.TenantIDHolding.String(),
		TenantIDSubsidiary: r.TenantIDSubsidiary.String(),
		RelationshipType:   r.RelationshipType,
		Status:             string(r.Status),
		RequestedAt:        r.RequestedAt,
		ApprovedAt:         r.ApprovedAt,
		RejectedAt:         r.RejectedAt,
		CanceledAt:         r.CanceledAt,
		EndsAt:             r.EndsAt,
		RejectionReason:    r.RejectionReason,
	}
}

// InitiateRelationship godoc
// @Summary      Initiate a PENDING holding/subsidiary relationship
// @Description  The authenticated subsidiary requests affiliation to a holding. The subsidiary is taken from the JWT.
// @Tags         Tenant Relationships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        request body dto.InitiateRelationshipRequest true "Holding and relationship type"
// @Success      201 {object} dto.TenantRelationshipEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /tenant-relationships/initiate [post]
func (h *TenantRelationshipHandler) Initiate(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.InitiateRelationshipRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	holdingID, err := uuid.Parse(request.HoldingTenantID)
	if err != nil || holdingID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_holding_tenant", "holding_tenant_id must be a valid UUID", err))
		return
	}
	relationship, err := h.uc.StartPendingRelationship(c.Request.Context(), usecase.StartPendingRelationshipInput{
		HoldingTenantID:    holdingID,
		SubsidiaryTenantID: sc.TenantID,
		RelationshipType:   request.RelationshipType,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.TenantRelationshipEnvelope{Data: relationshipResponse(relationship)})
}

// EmitInviteToken godoc
// @Summary      Emit an invite token
// @Description  Holding-only action. Emits the invite token once for a PENDING relationship.
// @Tags         Tenant Relationships
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        id path string true "Relationship UUID"
// @Success      200 {object} dto.EmitInviteTokenEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Router       /tenant-relationships/{id}/emit-token [post]
func (h *TenantRelationshipHandler) EmitToken(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	relationshipID, err := uuid.Parse(c.Param("id"))
	if err != nil || relationshipID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_relationship_id", "invalid relationship id", err))
		return
	}
	result, err := h.uc.EmitInviteToken(c.Request.Context(), usecase.EmitInviteTokenInput{
		RelationshipID: relationshipID,
		HoldingID:      sc.TenantID,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.EmitInviteTokenEnvelope{Data: dto.EmitInviteTokenResponse{
		InviteCode: result.InviteCode,
		ExpiresAt:  result.Invite.ExpiresAt,
	}})
}

// ConsumeInviteToken godoc
// @Summary      Consume an invite token
// @Description  Subsidiary-only action. The subsidiary is taken from the JWT; no tenant_id is accepted from the body. Approves the relationship after cycle validation.
// @Tags         Tenant Relationships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        request body dto.ConsumeInviteTokenRequest true "Invite token"
// @Success      200 {object} dto.TenantRelationshipEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /tenant-relationships/consume-token [post]
func (h *TenantRelationshipHandler) ConsumeToken(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.ConsumeInviteTokenRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	relationship, err := h.uc.ConsumeInviteToken(c.Request.Context(), usecase.ConsumeInviteTokenInput{
		InviteCode:   request.InviteCode,
		SubsidiaryID: sc.TenantID,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.TenantRelationshipEnvelope{Data: relationshipResponse(relationship)})
}

// RejectRelationship godoc
// @Summary      Reject a PENDING relationship
// @Description  Holding-only action. Rejects the relationship and invalidates its active invites.
// @Tags         Tenant Relationships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        id path string true "Relationship UUID"
// @Param        request body dto.RelationshipReasonRequest false "Reason"
// @Success      200 {object} dto.TenantRelationshipEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /tenant-relationships/{id}/reject [post]
func (h *TenantRelationshipHandler) Reject(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	relationshipID, err := uuid.Parse(c.Param("id"))
	if err != nil || relationshipID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_relationship_id", "invalid relationship id", err))
		return
	}
	var request dto.RelationshipReasonRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		c.Error(badRequestError(err))
		return
	}
	relationship, err := h.uc.RejectRelationship(c.Request.Context(), usecase.RejectRelationshipInput{
		RelationshipID: relationshipID,
		HoldingID:      sc.TenantID,
		Reason:         request.Reason,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.TenantRelationshipEnvelope{Data: relationshipResponse(relationship)})
}

// CancelRelationship godoc
// @Summary      Cancel a PENDING relationship
// @Description  Subsidiary-only action. Cancels the relationship and invalidates its active invites.
// @Tags         Tenant Relationships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        id path string true "Relationship UUID"
// @Param        request body dto.RelationshipReasonRequest false "Reason"
// @Success      200 {object} dto.TenantRelationshipEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /tenant-relationships/{id}/cancel [post]
func (h *TenantRelationshipHandler) Cancel(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	relationshipID, err := uuid.Parse(c.Param("id"))
	if err != nil || relationshipID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_relationship_id", "invalid relationship id", err))
		return
	}
	var request dto.RelationshipReasonRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		c.Error(badRequestError(err))
		return
	}
	relationship, err := h.uc.CancelPendingRelationship(c.Request.Context(), usecase.CancelPendingRelationshipInput{
		RelationshipID: relationshipID,
		SubsidiaryID:   sc.TenantID,
		Reason:         request.Reason,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.TenantRelationshipEnvelope{Data: relationshipResponse(relationship)})
}

// RevokeRelationship godoc
// @Summary      Revoke an APPROVED relationship
// @Description  Holding-only action. Revokes the relationship.
// @Tags         Tenant Relationships
// @Produce      json
// @Security     BearerAuth
// @Param        X-App-Client-ID header string true "Client application UUID"
// @Param        id path string true "Relationship UUID"
// @Success      200 {object} dto.TenantRelationshipEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /tenant-relationships/{id}/revoke [post]
func (h *TenantRelationshipHandler) Revoke(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	relationshipID, err := uuid.Parse(c.Param("id"))
	if err != nil || relationshipID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_relationship_id", "invalid relationship id", err))
		return
	}
	relationship, err := h.uc.RevokeApprovedRelationship(c.Request.Context(), usecase.RevokeApprovedRelationshipInput{
		RelationshipID: relationshipID,
		HoldingID:      sc.TenantID,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.TenantRelationshipEnvelope{Data: relationshipResponse(relationship)})
}
