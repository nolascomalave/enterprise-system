package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type AddPartyNationalityRequest struct {
	CountryCode string `json:"country_code" binding:"required"`
	IsPrimary   bool   `json:"is_primary"`
}

type PartyNationalityResponse struct {
	ID          string    `json:"id"`
	PartyID     string    `json:"party_id"`
	CountryCode string    `json:"country_code"`
	IsPrimary   bool      `json:"is_primary"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PartyNationalityEnvelope struct {
	Data PartyNationalityResponse `json:"data"`
}

func NewPartyNationalityResponse(nationality *domain.PartyNationality) PartyNationalityResponse {
	if nationality == nil {
		return PartyNationalityResponse{}
	}
	return PartyNationalityResponse{
		ID: nationality.ID.String(), PartyID: nationality.PartyID.String(),
		CountryCode: nationality.CountryCode, IsPrimary: nationality.IsPrimary,
		CreatedAt: nationality.CreatedAt, UpdatedAt: nationality.UpdatedAt,
	}
}

type AddPartyIdentityDocumentRequest struct {
	DocumentType          string  `json:"document_type" binding:"required"`
	IssuingCountryCode    string  `json:"issuing_country_code" binding:"required"`
	IssuingJurisdictionID *string `json:"issuing_jurisdiction_id,omitempty"`
	DocumentNumber        string  `json:"document_number" binding:"required"`
	IsPrimary             bool    `json:"is_primary"`
}

type PartyIdentityDocumentValidityResponse struct {
	ID        string     `json:"id"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	IsPrimary bool       `json:"is_primary"`
}

type PartyIdentityDocumentResponse struct {
	ID                    string                                 `json:"id"`
	PartyID               string                                 `json:"party_id"`
	DocumentType          string                                 `json:"document_type"`
	IssuingCountryCode    string                                 `json:"issuing_country_code"`
	IssuingJurisdictionID *string                                `json:"issuing_jurisdiction_id,omitempty"`
	DocumentNumber        string                                 `json:"document_number"`
	CreatedAt             time.Time                              `json:"created_at"`
	UpdatedAt             time.Time                              `json:"updated_at"`
	Validity              *PartyIdentityDocumentValidityResponse `json:"validity,omitempty"`
}

type PartyIdentityDocumentEnvelope struct {
	Data PartyIdentityDocumentResponse `json:"data"`
}

func NewPartyIdentityDocumentResponse(
	document *domain.PartyIdentityDocument,
	validity *domain.PartyIdentityDocumentValidity,
	typeCode string,
) PartyIdentityDocumentResponse {
	if document == nil {
		return PartyIdentityDocumentResponse{}
	}
	response := PartyIdentityDocumentResponse{
		ID: document.ID.String(), PartyID: document.PartyID.String(),
		DocumentType: typeCode, IssuingCountryCode: document.IssuingCountryCode,
		DocumentNumber: document.DocumentNumber,
		CreatedAt:      document.CreatedAt, UpdatedAt: document.UpdatedAt,
	}
	if document.IssuingJurisdictionID != nil {
		jurisdictionID := document.IssuingJurisdictionID.String()
		response.IssuingJurisdictionID = &jurisdictionID
	}
	if validity != nil {
		response.Validity = &PartyIdentityDocumentValidityResponse{
			ID: validity.ID.String(), StartDate: validity.StartDate,
			EndDate: validity.EndDate, IsPrimary: validity.IsPrimary,
		}
	}
	return response
}
