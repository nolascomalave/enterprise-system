package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type LocationResponse struct {
	ID           string    `json:"id"`
	LocationType string    `json:"location_type"`
	ParentID     *string   `json:"parent_id,omitempty"`
	CountryCode  string    `json:"country_code"`
	ISOCode      *string   `json:"iso_code,omitempty"`
	ExternalID   *string   `json:"external_id,omitempty"`
	Name         string    `json:"name"`
	DefaultName  string    `json:"default_name"`
	Locale       string    `json:"locale,omitempty"`
	Path         string    `json:"path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LocationListEnvelope struct {
	Data []LocationResponse `json:"data"`
}

func NewLocationResponse(location *domain.Location) LocationResponse {
	if location == nil {
		return LocationResponse{}
	}
	response := LocationResponse{
		ID: location.ID.String(), LocationType: string(location.LocationType),
		CountryCode: location.CountryCode, Name: location.DefaultName,
		DefaultName: location.DefaultName,
		Path:        location.Path, CreatedAt: location.CreatedAt, UpdatedAt: location.UpdatedAt,
	}
	if location.ParentID != nil {
		parentID := location.ParentID.String()
		response.ParentID = &parentID
	}
	if location.ISOCode != "" {
		isoCode := location.ISOCode
		response.ISOCode = &isoCode
	}
	if location.ExternalID != "" {
		externalID := location.ExternalID
		response.ExternalID = &externalID
	}
	return response
}

func NewLocationListResponse(locations []*domain.Location) []LocationResponse {
	result := make([]LocationResponse, 0, len(locations))
	for _, location := range locations {
		result = append(result, NewLocationResponse(location))
	}
	return result
}

func NewLocalizedLocationResponse(translated *domain.TranslatedLocation) LocationResponse {
	if translated == nil || translated.Location == nil {
		return LocationResponse{}
	}
	response := NewLocationResponse(translated.Location)
	response.Name = translated.Name
	response.Locale = translated.Locale
	return response
}

func NewLocalizedLocationListResponse(locations []*domain.TranslatedLocation) []LocationResponse {
	result := make([]LocationResponse, 0, len(locations))
	for _, location := range locations {
		result = append(result, NewLocalizedLocationResponse(location))
	}
	return result
}
