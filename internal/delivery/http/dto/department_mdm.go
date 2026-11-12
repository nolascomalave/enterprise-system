package dto

import (
	"time"

	"enterprise-system/internal/domain"
)

type AddDepartmentContactRequest struct {
	Type      string `json:"type" binding:"required"`
	Value     string `json:"value" binding:"required"`
	Order     int    `json:"order"`
	IsPrimary bool   `json:"is_primary"`
}

type DepartmentContactValidityResponse struct {
	ID        string     `json:"id"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	IsPrimary bool       `json:"is_primary"`
}

type DepartmentContactResponse struct {
	ID                 string                             `json:"id"`
	DepartmentID       string                             `json:"department_id"`
	ContactMechanismID string                             `json:"contact_mechanism_id"`
	ContactType        string                             `json:"contact_type"`
	Value              string                             `json:"value"`
	Order              int                                `json:"order"`
	IsPrimary          bool                               `json:"is_primary"`
	Validity           *DepartmentContactValidityResponse `json:"validity,omitempty"`
}

type DepartmentContactEnvelope struct {
	Data DepartmentContactResponse `json:"data"`
}

func NewDepartmentContactResponse(result *domain.ContactMechanism, relation *domain.DepartmentContactMechanism, validity *domain.DepartmentContactMechanismValidity) DepartmentContactResponse {
	if relation == nil {
		return DepartmentContactResponse{}
	}
	response := DepartmentContactResponse{
		ID: relation.ID.String(), DepartmentID: relation.DepartmentID.String(),
		ContactMechanismID: relation.ContactMechanismID.String(),
		ContactType:        string(relation.ContactType),
		Order:              relation.ContactMechanismOrder,
		IsPrimary:          relation.IsPrimary,
	}
	if result != nil {
		response.Value = result.Value
	}
	if validity != nil {
		response.Validity = &DepartmentContactValidityResponse{
			ID: validity.ID.String(), StartDate: validity.StartDate,
			EndDate: validity.EndDate, IsPrimary: validity.IsPrimary,
		}
	}
	return response
}

type AddDepartmentAddressRequest struct {
	Type           string `json:"type" binding:"required"`
	Line1          string `json:"line1" binding:"required"`
	Line2          string `json:"line2,omitempty"`
	CityLocationID string `json:"city_location_id" binding:"required"`
	IsPrimary      bool   `json:"is_primary"`
}

type DepartmentAddressValidityResponse struct {
	ID        string     `json:"id"`
	StartDate time.Time  `json:"start_date"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	IsPrimary bool       `json:"is_primary"`
}

type DepartmentAddressResponse struct {
	ID             string                             `json:"id"`
	DepartmentID   string                             `json:"department_id"`
	CityLocationID string                             `json:"city_location_id"`
	Type           string                             `json:"type"`
	Line1          string                             `json:"line1"`
	Line2          string                             `json:"line2,omitempty"`
	IsPrimary      bool                               `json:"is_primary"`
	Validity       *DepartmentAddressValidityResponse `json:"validity,omitempty"`
}

type DepartmentAddressEnvelope struct {
	Data DepartmentAddressResponse `json:"data"`
}

func NewDepartmentAddressResponse(address *domain.DepartmentAddress, validity *domain.DepartmentAddressValidity) DepartmentAddressResponse {
	if address == nil {
		return DepartmentAddressResponse{}
	}
	response := DepartmentAddressResponse{
		ID: address.ID.String(), DepartmentID: address.DepartmentID.String(),
		CityLocationID: address.CityLocationID.String(), Type: string(address.Type),
		Line1: address.Line1, Line2: address.Line2,
	}
	if validity != nil {
		response.IsPrimary = validity.IsPrimary
		response.Validity = &DepartmentAddressValidityResponse{
			ID: validity.ID.String(), StartDate: validity.StartDate,
			EndDate: validity.EndDate, IsPrimary: validity.IsPrimary,
		}
	}
	return response
}
