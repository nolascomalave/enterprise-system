package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AddressType string

const (
	AddressTypeLegal    AddressType = "LEGAL"
	AddressTypeFiscal   AddressType = "FISCAL"
	AddressTypePostal   AddressType = "POSTAL"
	AddressTypeDelivery AddressType = "DELIVERY"
)

func (t AddressType) Valid() bool {
	switch t {
	case AddressTypeLegal, AddressTypeFiscal, AddressTypePostal, AddressTypeDelivery:
		return true
	default:
		return false
	}
}

var countryAlpha2Pattern = regexp.MustCompile(`^[A-Z]{2}$`)

type Address struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	PartyID        uuid.UUID
	Type           AddressType
	Line1          string
	Line2          string
	CityLocationID uuid.UUID
	PostalCode     string
	Country        string
	IsPrimary      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NewAddress(
	tenantID, partyID uuid.UUID,
	addressType AddressType,
	line1, line2 string,
	cityLocationID uuid.UUID,
	postalCode, country string,
	isPrimary bool,
	now time.Time,
) (*Address, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate address id: %w", err)
	}

	now = now.UTC()
	address := &Address{
		ID:             id,
		TenantID:       tenantID,
		PartyID:        partyID,
		Type:           addressType,
		Line1:          strings.TrimSpace(line1),
		Line2:          strings.TrimSpace(line2),
		CityLocationID: cityLocationID,
		PostalCode:     strings.TrimSpace(postalCode),
		Country:        strings.ToUpper(strings.TrimSpace(country)),
		IsPrimary:      isPrimary,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := address.Validate(); err != nil {
		return nil, err
	}
	return address, nil
}

func (a *Address) Validate() error {
	if a == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidAddress)
	}
	if a.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidAddress)
	}
	if a.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidAddress)
	}
	if a.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidAddress)
	}
	if !a.Type.Valid() {
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidAddress, a.Type)
	}
	if strings.TrimSpace(a.Line1) == "" {
		return fmt.Errorf("%w: line1 is required", ErrInvalidAddress)
	}
	if a.CityLocationID == uuid.Nil {
		return fmt.Errorf("%w: city_location_id is required", ErrInvalidAddress)
	}
	if !countryAlpha2Pattern.MatchString(a.Country) {
		return fmt.Errorf("%w: country must be ISO 3166-1 alpha-2", ErrInvalidAddress)
	}
	if a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidAddress)
	}
	if a.UpdatedAt.Before(a.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidAddress)
	}
	return nil
}

func (a *Address) Update(
	addressType AddressType,
	line1, line2 string,
	cityLocationID uuid.UUID,
	postalCode, country string,
	isPrimary bool,
	now time.Time,
) error {
	if a.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	a.Type = addressType
	a.Line1 = strings.TrimSpace(line1)
	a.Line2 = strings.TrimSpace(line2)
	a.CityLocationID = cityLocationID
	a.PostalCode = strings.TrimSpace(postalCode)
	a.Country = strings.ToUpper(strings.TrimSpace(country))
	a.IsPrimary = isPrimary
	a.UpdatedAt = now.UTC()
	return a.Validate()
}

func (a *Address) SetPrimary(isPrimary bool, now time.Time) error {
	if a.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	a.IsPrimary = isPrimary
	a.UpdatedAt = now.UTC()
	return nil
}

func (a *Address) SoftDelete(now time.Time) error {
	if a.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	a.DeletedAt = &now
	a.UpdatedAt = now
	return nil
}
