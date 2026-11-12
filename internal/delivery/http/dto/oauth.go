package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type CreateClientApplicationRequest struct {
	Name   string   `json:"name" binding:"required"`
	Scopes []string `json:"scopes" binding:"required"`
}

type ClientApplicationResponse struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	ClientID  string    `json:"client_id"`
	Scopes    []string  `json:"scopes"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type CreatedClientApplicationResponse struct {
	ClientApplicationResponse
	ClientSecret string `json:"client_secret"`
}

type ClientApplicationEnvelope struct {
	Data CreatedClientApplicationResponse `json:"data"`
}

func NewClientApplicationResponse(application *domain.ClientApplication) ClientApplicationResponse {
	if application == nil {
		return ClientApplicationResponse{}
	}
	return ClientApplicationResponse{
		ID: application.ID.String(), TenantID: application.TenantID.String(),
		Name: application.Name, ClientID: application.ClientID,
		Scopes: application.Scopes, IsActive: application.IsActive,
		CreatedAt: application.CreatedAt,
	}
}

type ClientCredentialsRequest struct {
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
}

type ClientTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}
