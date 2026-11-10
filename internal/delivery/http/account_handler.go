package http

import (
	"context"
	"fmt"
	"net/http"

	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AccountUseCase interface {
	ChangePassword(ctx context.Context, in usecase.ChangePasswordInput) error
	ForgotPassword(ctx context.Context, in usecase.ForgotPasswordInput) error
	ResetPassword(ctx context.Context, in usecase.ResetPasswordInput) error
}

type AccountHandler struct {
	uc AccountUseCase
}

func NewAccountHandler(uc AccountUseCase) *AccountHandler {
	return &AccountHandler{uc: uc}
}

func (h *AccountHandler) ChangePassword(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(ErrUnauthorized)
		return
	}

	var request dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}

	err := h.uc.ChangePassword(c.Request.Context(), usecase.ChangePasswordInput{
		TenantID: sessionContext.TenantID, PartyID: sessionContext.UserID,
		SessionID:       sessionContext.SessionID,
		CurrentPassword: request.CurrentPassword, NewPassword: request.NewPassword,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AccountHandler) ForgotPassword(c *gin.Context) {
	var request dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil {
		c.Error(fmt.Errorf("%w: invalid tenant_id", ErrBadRequest))
		return
	}

	if err := h.uc.ForgotPassword(c.Request.Context(), usecase.ForgotPasswordInput{
		TenantID: tenantID, Email: request.Email, Metadata: requestMetadata(c),
	}); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": dto.MessageResponse{
		Message: "if the email exists, a password reset link has been sent",
	}})
}

func (h *AccountHandler) ResetPassword(c *gin.Context) {
	var request dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}

	if err := h.uc.ResetPassword(c.Request.Context(), usecase.ResetPasswordInput{
		Token: request.Token, NewPassword: request.NewPassword,
	}); err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
