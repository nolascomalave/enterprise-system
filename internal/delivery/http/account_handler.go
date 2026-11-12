package http

import (
	"context"
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

// ChangePassword godoc
// @Summary      Change the authenticated user's password
// @Description  Requires the current password. Revokes all other sessions and pending reset tokens.
// @Tags         Account
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.ChangePasswordRequest true "Current and new password"
// @Success      204 "No Content"
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /auth/change-password [post]
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	var request dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
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

// ForgotPassword godoc
// @Summary      Request a password reset
// @Description  Always returns the same 202 response. If the email exists, a single-use reset token is emailed.
// @Tags         Account
// @Accept       json
// @Produce      json
// @Param        request body dto.ForgotPasswordRequest true "Tenant and email"
// @Success      202 {object} dto.MessageEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      429 {object} dto.ErrorResponse
// @Router       /auth/forgot-password [post]
func (h *AccountHandler) ForgotPassword(c *gin.Context) {
	var request dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil {
		c.Error(badRequestError(errInvalidTenantID))
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

// ResetPassword godoc
// @Summary      Reset a password with a one-time token
// @Description  Consumes the reset token, updates the password and revokes all sessions.
// @Tags         Account
// @Accept       json
// @Produce      json
// @Param        request body dto.ResetPasswordRequest true "Reset token and new password"
// @Success      204 "No Content"
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      429 {object} dto.ErrorResponse
// @Router       /auth/reset-password [post]
func (h *AccountHandler) ResetPassword(c *gin.Context) {
	var request dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
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
