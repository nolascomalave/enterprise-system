package dto

import (
	"enterprise-system/internal/domain"
)

type LocalizedCatalogEntryResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Locale      string `json:"locale,omitempty"`
}

type CatalogListEnvelope struct {
	Data []LocalizedCatalogEntryResponse `json:"data"`
}

func NewLocalizedCatalogListResponse(entries []*domain.LocalizedCatalogEntry) []LocalizedCatalogEntryResponse {
	result := make([]LocalizedCatalogEntryResponse, 0, len(entries))
	for _, entry := range entries {
		result = append(result, LocalizedCatalogEntryResponse{
			ID: entry.ID.String(), Code: entry.Code, Name: entry.Name,
			Description: entry.Description, Locale: entry.Locale,
		})
	}
	return result
}

type LocalizedDepartmentResponse struct {
	DepartmentResponse
	Locale string `json:"locale,omitempty"`
}

func NewLocalizedDepartmentListResponse(departments []*domain.LocalizedDepartment) []LocalizedDepartmentResponse {
	result := make([]LocalizedDepartmentResponse, 0, len(departments))
	for _, department := range departments {
		response := NewDepartmentResponse(department.Department)
		response.Name = department.Name
		result = append(result, LocalizedDepartmentResponse{
			DepartmentResponse: response,
			Locale:             department.Locale,
		})
	}
	return result
}
