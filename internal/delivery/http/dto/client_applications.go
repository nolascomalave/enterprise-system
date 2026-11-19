package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type ClientApplicationListItemResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ClientID  string    `json:"client_id"`
	Scopes    []string  `json:"scopes"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type ClientApplicationListEnvelope struct {
	Data []ClientApplicationListItemResponse `json:"data"`
}

func NewClientApplicationListItemResponse(application *domain.ClientApplication) ClientApplicationListItemResponse {
	if application == nil {
		return ClientApplicationListItemResponse{}
	}
	return ClientApplicationListItemResponse{
		ID: application.ID.String(), Name: application.Name,
		ClientID: application.ClientID, Scopes: application.Scopes,
		IsActive: application.IsActive, CreatedAt: application.CreatedAt,
	}
}
