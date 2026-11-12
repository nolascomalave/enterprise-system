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
	DefaultName  string    `json:"default_name"`
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
		CountryCode: location.CountryCode, DefaultName: location.DefaultName,
		Path: location.Path, CreatedAt: location.CreatedAt, UpdatedAt: location.UpdatedAt,
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
