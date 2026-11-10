package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type CreatePartyRequest struct {
	PartyType           string                            `json:"party_type" binding:"required,oneof=INDIVIDUAL ORGANIZATION"`
	DisplayName         string                            `json:"display_name" binding:"required"`
	PersonProfile       *CreatePersonProfileRequest       `json:"person_profile,omitempty"`
	OrganizationProfile *CreateOrganizationProfileRequest `json:"organization_profile,omitempty"`
}

type CreatePersonProfileRequest struct {
	FirstName      string     `json:"first_name"`
	MiddleName     string     `json:"middle_name,omitempty"`
	LastName       string     `json:"last_name"`
	SecondLastName string     `json:"second_last_name,omitempty"`
	BirthDate      *time.Time `json:"birth_date,omitempty"`
}

type CreateOrganizationProfileRequest struct {
	LegalName string `json:"legal_name"`
	TradeName string `json:"trade_name,omitempty"`
	TaxID     string `json:"tax_id"`
}

type PartyResponse struct {
	ID                  string                       `json:"id"`
	TenantID            string                       `json:"tenant_id"`
	PartyType           string                       `json:"party_type"`
	DisplayName         string                       `json:"display_name"`
	RecordStatus        string                       `json:"record_status"`
	CanonicalPartyID    *string                      `json:"canonical_party_id,omitempty"`
	PersonProfile       *PersonProfileResponse       `json:"person_profile,omitempty"`
	OrganizationProfile *OrganizationProfileResponse `json:"organization_profile,omitempty"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
	DeletedAt           *time.Time                   `json:"deleted_at,omitempty"`
}

type PersonProfileResponse struct {
	ID             string     `json:"id"`
	PartyID        string     `json:"party_id"`
	FirstName      string     `json:"first_name"`
	MiddleName     string     `json:"middle_name,omitempty"`
	LastName       string     `json:"last_name"`
	SecondLastName string     `json:"second_last_name,omitempty"`
	BirthDate      *time.Time `json:"birth_date,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type OrganizationProfileResponse struct {
	ID        string    `json:"id"`
	PartyID   string    `json:"party_id"`
	LegalName string    `json:"legal_name"`
	TradeName string    `json:"trade_name,omitempty"`
	TaxID     string    `json:"tax_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewPartyResponse(detail *domain.PartyDetail) PartyResponse {
	if detail == nil || detail.Party == nil {
		return PartyResponse{}
	}
	p := detail.Party
	resp := PartyResponse{
		ID:           p.ID.String(),
		TenantID:     p.TenantID.String(),
		PartyType:    string(p.PartyType),
		DisplayName:  p.DisplayName,
		RecordStatus: string(p.RecordStatus),
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		DeletedAt:    p.DeletedAt,
	}
	if p.CanonicalPartyID != nil {
		value := p.CanonicalPartyID.String()
		resp.CanonicalPartyID = &value
	}
	if detail.PersonProfile != nil {
		resp.PersonProfile = newPersonProfileResponse(detail.PersonProfile)
	}
	if detail.OrganizationProfile != nil {
		resp.OrganizationProfile = newOrganizationProfileResponse(detail.OrganizationProfile)
	}
	return resp
}

func NewPartyListResponse(parties []*domain.Party) []PartyResponse {
	result := make([]PartyResponse, 0, len(parties))
	for _, p := range parties {
		result = append(result, NewPartyResponse(&domain.PartyDetail{Party: p}))
	}
	return result
}

func newPersonProfileResponse(p *domain.PersonProfile) *PersonProfileResponse {
	return &PersonProfileResponse{
		ID:             p.ID.String(),
		PartyID:        p.PartyID.String(),
		FirstName:      p.FirstName,
		MiddleName:     p.MiddleName,
		LastName:       p.LastName,
		SecondLastName: p.SecondLastName,
		BirthDate:      p.BirthDate,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func newOrganizationProfileResponse(p *domain.OrganizationProfile) *OrganizationProfileResponse {
	return &OrganizationProfileResponse{
		ID:        p.ID.String(),
		PartyID:   p.PartyID.String(),
		LegalName: p.LegalName,
		TradeName: p.TradeName,
		TaxID:     p.TaxID,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
