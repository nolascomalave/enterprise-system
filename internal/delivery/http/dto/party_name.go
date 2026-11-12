package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type AddPartyNameRequest struct {
	NameType  string `json:"name_type" binding:"required"`
	Value     string `json:"value" binding:"required"`
	IsPrimary bool   `json:"is_primary"`
	Ordering  int    `json:"ordering"`
}

type UpdatePartyNameValidityRequest struct {
	NewValue  string `json:"new_value,omitempty"`
	IsPrimary bool   `json:"is_primary"`
	Ordering  int    `json:"ordering"`
}

type PartyNameValidityResponse struct {
	ID        string     `json:"id"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	IsPrimary bool       `json:"is_primary"`
}

type PartyNameResponse struct {
	ID         string                     `json:"id"`
	TenantID   string                     `json:"tenant_id"`
	PartyID    string                     `json:"party_id"`
	NameTypeID string                     `json:"name_type_id"`
	Value      string                     `json:"value"`
	IsPrimary  bool                       `json:"is_primary"`
	Ordering   int                        `json:"ordering"`
	CreatedAt  time.Time                  `json:"created_at"`
	UpdatedAt  time.Time                  `json:"updated_at"`
	Validity   *PartyNameValidityResponse `json:"validity,omitempty"`
}

type PartyNameEnvelope struct {
	Data PartyNameResponse `json:"data"`
}

func NewPartyNameResponse(name *domain.PartyName, validity *domain.PartyNameValidity) PartyNameResponse {
	if name == nil {
		return PartyNameResponse{}
	}
	response := PartyNameResponse{
		ID: name.ID.String(), TenantID: name.TenantID.String(), PartyID: name.PartyID.String(),
		NameTypeID: name.NameTypeID.String(), Value: name.Value,
		IsPrimary: name.IsPrimary, Ordering: name.Ordering,
		CreatedAt: name.CreatedAt, UpdatedAt: name.UpdatedAt,
	}
	if validity != nil {
		response.Validity = &PartyNameValidityResponse{
			ID: validity.ID.String(), StartDate: validity.StartDate,
			EndDate: validity.EndDate, IsPrimary: validity.IsPrimary,
		}
	}
	return response
}
