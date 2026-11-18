package http

import (
	"context"
	"errors"
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

type MFAUseCase interface {
	UpdateMFAPreferences(ctx context.Context, in usecase.UpdateMFAPreferencesInput) error
	RequestSecurityCode(ctx context.Context, in usecase.RequestSecurityCodeInput) error
	SetupTOTP(ctx context.Context, tenantID, partyID uuid.UUID) (*usecase.SetupTOTPResult, error)
	VerifyTOTP(ctx context.Context, tenantID, partyID uuid.UUID, code string) (*usecase.VerifyTOTPResult, error)
}

type MFAHandler struct {
	uc MFAUseCase
}

func NewMFAHandler(uc MFAUseCase) *MFAHandler {
	return &MFAHandler{uc: uc}
}

// UpdatePreferences godoc
// @Summary      Update MFA preferences
// @Description  Enables or disables MFA. Changing or disabling an active MFA configuration requires otp_code, requested with action DISABLE_MFA (to disable) or ENABLE_MFA (to change method). Enabling requires a contact that can receive security codes.
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.UpdateMFAPreferencesRequest true "MFA preferences"
// @Success      204 "No Content"
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /auth/mfa-preferences [put]
func (h *MFAHandler) UpdatePreferences(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sessionContext.TenantID == uuid.Nil || sessionContext.UserID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	var request dto.UpdateMFAPreferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}

	method := domain.MFAMethod(strings.ToUpper(strings.TrimSpace(request.Method)))
	if !request.Enabled && method == "" {
		method = domain.MFAMethodNone
	}

	err := h.uc.UpdateMFAPreferences(c.Request.Context(), usecase.UpdateMFAPreferencesInput{
		TenantID: sessionContext.TenantID,
		PartyID:  sessionContext.UserID,
		Enabled:  request.Enabled,
		Method:   method,
		OTPCode:  request.OTPCode,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RequestSecurityCode godoc
// @Summary      Request a security code
// @Description  Sends a one-time security code to the authenticated user's primary contact mechanism.
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.RequestSecurityCodeRequest true "Security code action"
// @Success      202 {object} dto.MessageEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /auth/request-otp [post]
func (h *MFAHandler) RequestSecurityCode(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sessionContext.TenantID == uuid.Nil || sessionContext.UserID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	var request dto.RequestSecurityCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}

	err := h.uc.RequestSecurityCode(c.Request.Context(), usecase.RequestSecurityCodeInput{
		TenantID: sessionContext.TenantID,
		PartyID:  sessionContext.UserID,
		Action:   request.Action,
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": dto.MessageResponse{
		Message: "security code sent",
	}})
}

// SetupTOTP godoc
// @Summary      Start TOTP enrolment
// @Description  Generates a TOTP secret and returns the otpauth URL for the authenticator app. MFA is not enabled until the code is verified.
// @Tags         MFA
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.TOTPSetupEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /auth/totp/setup [post]
func (h *MFAHandler) SetupTOTP(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sessionContext.TenantID == uuid.Nil || sessionContext.UserID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	result, err := h.uc.SetupTOTP(c.Request.Context(), sessionContext.TenantID, sessionContext.UserID)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.TOTPSetupEnvelope{Data: dto.TOTPSetupResponse{
		Secret: result.Secret, OTPAuthURL: result.OTPAuthURL,
	}})
}

// VerifyTOTP godoc
// @Summary      Verify TOTP enrolment
// @Description  Validates a code against the pending TOTP secret, enables TOTP and returns 8 one-time backup codes. The plaintext backup codes are shown only in this response.
// @Tags         MFA
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.TOTPVerifyRequest true "TOTP code"
// @Success      200 {object} dto.TOTPVerifyEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Router       /auth/totp/verify [post]
func (h *MFAHandler) VerifyTOTP(c *gin.Context) {
	sessionContext, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok || sessionContext.TenantID == uuid.Nil || sessionContext.UserID == uuid.Nil {
		c.Error(unauthorizedError(ErrUnauthorized))
		return
	}

	var request dto.TOTPVerifyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}

	result, err := h.uc.VerifyTOTP(c.Request.Context(), sessionContext.TenantID, sessionContext.UserID, request.Code)
	if err != nil {
		c.Error(err)
		return
	}
	if result == nil {
		c.Error(apperror.NewInternal(errors.New("nil TOTP verify result")))
		return
	}
	c.JSON(http.StatusOK, dto.TOTPVerifyEnvelope{Data: dto.TOTPVerifyResponse{
		BackupCodes: result.BackupCodes,
	}})
}
