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
	ID               string    `json:"id"`
	TenantID         *string   `json:"tenant_id,omitempty"`
	Name             string    `json:"name"`
	ClientID         string    `json:"client_id"`
	Scopes           []string  `json:"scopes"`
	ClientType       string    `json:"client_type"`
	ApplicationScope string    `json:"application_scope"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
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
	response := ClientApplicationResponse{
		ID: application.ID.String(), Name: application.Name,
		ClientID: application.ClientID, Scopes: application.Scopes,
		ClientType: string(application.ClientType), ApplicationScope: string(application.ApplicationScope),
		IsActive: application.IsActive, CreatedAt: application.CreatedAt,
	}
	if application.TenantID != nil {
		tenantID := application.TenantID.String()
		response.TenantID = &tenantID
	}
	return response
}

type CreateFirstPartyApplicationRequest struct {
	Name             string   `json:"name" binding:"required"`
	Scopes           []string `json:"scopes" binding:"required"`
	ApplicationScope string   `json:"application_scope" binding:"required"`
	TenantID         *string  `json:"tenant_id,omitempty"`
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
