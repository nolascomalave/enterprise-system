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

type OAuthUseCase interface {
	Create(ctx context.Context, in usecase.CreateClientApplicationInput) (*usecase.CreatedClientApplication, error)
	IssueToken(ctx context.Context, clientID, clientSecret string) (*usecase.ClientToken, error)
}

type OAuthHandler struct {
	uc OAuthUseCase
}

func NewOAuthHandler(uc OAuthUseCase) *OAuthHandler {
	return &OAuthHandler{uc: uc}
}

// CreateClientApplication godoc
// @Summary      Create a client application
// @Description  Creates an OAuth2 M2M client. The client_secret is returned only once.
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateClientApplicationRequest true "Client application data"
// @Success      201 {object} dto.ClientApplicationEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /client-applications [post]
func (h *OAuthHandler) CreateClientApplication(c *gin.Context) {
	sc, ok := oauthSession(c)
	if !ok {
		return
	}
	var request dto.CreateClientApplicationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	created, err := h.uc.Create(c.Request.Context(), usecase.CreateClientApplicationInput{
		TenantID: &sc.TenantID, Name: request.Name, Scopes: request.Scopes,
		ClientType: domain.ClientTypeThirdParty, ApplicationScope: domain.ApplicationScopeTenantSpecific,
	})
	if err != nil {
		c.Error(err)
		return
	}
	response := dto.CreatedClientApplicationResponse{
		ClientApplicationResponse: dto.NewClientApplicationResponse(created.Application),
		ClientSecret:              created.ClientSecret,
	}
	c.JSON(http.StatusCreated, dto.ClientApplicationEnvelope{Data: response})
}

// CreateFirstPartyApplication godoc
// @Summary      Create a first-party client application
// @Description  Platform-admin only. Creates an official FIRST_PARTY app, either PLATFORM (no tenant) or TENANT_SPECIFIC.
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Param        X-Platform-Admin-Key header string true "Platform administrator key"
// @Param        request body dto.CreateFirstPartyApplicationRequest true "First-party application data"
// @Success      201 {object} dto.ClientApplicationEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /client-applications/first-party [post]
func (h *OAuthHandler) CreateFirstPartyApplication(c *gin.Context) {
	var request dto.CreateFirstPartyApplicationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	var tenantID *uuid.UUID
	if request.TenantID != nil && strings.TrimSpace(*request.TenantID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*request.TenantID))
		if err != nil || parsed == uuid.Nil {
			c.Error(apperror.NewValidation("invalid_tenant", "invalid tenant_id", err))
			return
		}
		tenantID = &parsed
	}
	created, err := h.uc.Create(c.Request.Context(), usecase.CreateClientApplicationInput{
		TenantID: tenantID, Name: request.Name, Scopes: request.Scopes,
		ClientType:       domain.ClientTypeFirstParty,
		ApplicationScope: domain.ApplicationScope(request.ApplicationScope),
	})
	if err != nil {
		c.Error(err)
		return
	}
	response := dto.CreatedClientApplicationResponse{
		ClientApplicationResponse: dto.NewClientApplicationResponse(created.Application),
		ClientSecret:              created.ClientSecret,
	}
	c.JSON(http.StatusCreated, dto.ClientApplicationEnvelope{Data: response})
}

// PlatformStatus godoc
// @Summary      Platform status
// @Description  Minimal endpoint protected by the FIRST_PARTY/PLATFORM scope platform:admin.
// @Tags         Platform
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]string
// @Failure      401 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse
// @Router       /platform/status [get]
func (h *OAuthHandler) PlatformStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Token godoc
// @Summary      OAuth2 client credentials token
// @Description  Exchanges client_id/client_secret for a short-lived JWT with tenant_id, client_id and scopes.
// @Tags         OAuth
// @Accept       json
// @Produce      json
// @Param        request body dto.ClientCredentialsRequest true "Client credentials"
// @Success      200 {object} dto.ClientTokenResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /oauth/token [post]
func (h *OAuthHandler) Token(c *gin.Context) {
	var request dto.ClientCredentialsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	token, err := h.uc.IssueToken(c.Request.Context(),
		strings.TrimSpace(request.ClientID), request.ClientSecret)
	if err != nil {
		c.Error(err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.ClientTokenResponse{
		AccessToken: token.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   token.ExpiresIn,
		Scope:       strings.Join(token.Scopes, " "),
	})
}

func oauthSession(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}
