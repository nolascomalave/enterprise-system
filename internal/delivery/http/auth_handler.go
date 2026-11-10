package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const refreshTokenCookie = "refresh_token"

type AuthUseCase interface {
	Login(ctx context.Context, in usecase.LoginInput) (*usecase.LoginResult, error)
	Verify2FA(ctx context.Context, in usecase.Verify2FAInput) (*usecase.LoginResult, error)
	Refresh(ctx context.Context, refreshToken string, metadata map[string]any) (*usecase.RefreshResult, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthHandler struct {
	uc           AuthUseCase
	secureCookie bool
	refreshTTL   time.Duration
}

func NewAuthHandler(uc AuthUseCase, secureCookie bool, refreshTTL time.Duration) *AuthHandler {
	if refreshTTL <= 0 {
		refreshTTL = 30 * 24 * time.Hour
	}
	return &AuthHandler{uc: uc, secureCookie: secureCookie, refreshTTL: refreshTTL}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request dto.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}
	tenantID, err := uuid.Parse(request.TenantID)
	if err != nil {
		c.Error(fmt.Errorf("%w: invalid tenant_id", ErrBadRequest))
		return
	}

	result, err := h.uc.Login(c.Request.Context(), usecase.LoginInput{
		TenantID: tenantID, Email: request.Email, Password: request.Password,
		Metadata: requestMetadata(c),
	})
	if err != nil {
		c.Error(err)
		return
	}

	c.Header("Cache-Control", "no-store")
	if result.Requires2FA {
		c.JSON(http.StatusOK, gin.H{"data": dto.AuthTokenResponse{
			Requires2FA: true,
			TempToken:   result.TempToken,
		}})
		return
	}

	h.setRefreshCookie(c, result.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"data": dto.AuthTokenResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn,
	}})
}

func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var request dto.Verify2FARequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(fmt.Errorf("%w: %v", ErrBadRequest, err))
		return
	}

	result, err := h.uc.Verify2FA(c.Request.Context(), usecase.Verify2FAInput{
		TempToken: request.TempToken, Code: request.Code, Metadata: requestMetadata(c),
	})
	if err != nil {
		c.Error(err)
		return
	}

	h.setRefreshCookie(c, result.RefreshToken)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": dto.AuthTokenResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn,
	}})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshTokenCookie)
	if err != nil || refreshToken == "" {
		c.Error(auth.ErrInvalidToken)
		return
	}

	result, err := h.uc.Refresh(c.Request.Context(), refreshToken, requestMetadata(c))
	if err != nil {
		c.Error(err)
		return
	}

	h.setRefreshCookie(c, result.RefreshToken)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"data": dto.AuthTokenResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn,
	}})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshTokenCookie)
	if err == nil && refreshToken != "" {
		if logoutErr := h.uc.Logout(c.Request.Context(), refreshToken); logoutErr != nil && !errors.Is(logoutErr, auth.ErrInvalidToken) {
			c.Error(logoutErr)
			return
		}
	}
	h.clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
		Expires:  time.Now().Add(h.refreshTTL),
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
}

func requestMetadata(c *gin.Context) map[string]any {
	return map[string]any{
		"ip":         c.ClientIP(),
		"user_agent": c.Request.UserAgent(),
	}
}
