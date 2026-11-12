package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AddressValidity is a time window for a party address.
type AddressValidity struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	PartyID   uuid.UUID
	AddressID uuid.UUID
	Type      AddressType
	IsPrimary bool
	StartDate time.Time
	EndDate   *time.Time
	CreatedAt time.Time
	DeletedAt *time.Time
}

func NewAddressValidity(
	tenantID, partyID, addressID uuid.UUID,
	addressType AddressType,
	isPrimary bool,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*AddressValidity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate address validity id: %w", err)
	}
	now = now.UTC()
	validity := &AddressValidity{
		ID: id, TenantID: tenantID, PartyID: partyID, AddressID: addressID,
		Type: addressType, IsPrimary: isPrimary,
		StartDate: startDate.UTC(), CreatedAt: now,
	}
	if endDate != nil {
		end := endDate.UTC()
		validity.EndDate = &end
	}
	if err := validity.Validate(); err != nil {
		return nil, err
	}
	return validity, nil
}

func (v *AddressValidity) Validate() error {
	if v == nil {
		return fmt.Errorf("%w: nil validity", ErrInvalidAddress)
	}
	if v.ID == uuid.Nil || v.TenantID == uuid.Nil || v.PartyID == uuid.Nil || v.AddressID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidAddress)
	}
	if !v.Type.Valid() {
		return fmt.Errorf("%w: invalid type", ErrInvalidAddress)
	}
	if v.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidAddress)
	}
	if v.EndDate != nil && !v.EndDate.After(v.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidAddress)
	}
	return nil
}

func (v *AddressValidity) Close(at time.Time) error {
	if v.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(v.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidAddress)
	}
	v.EndDate = &at
	return nil
}

// DepartmentContactMechanism links a department with a global contact.
type DepartmentContactMechanism struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	DepartmentID          uuid.UUID
	ContactMechanismID    uuid.UUID
	ContactType           ContactType
	ContactMechanismOrder int
	IsPrimary             bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

func NewDepartmentContactMechanism(
	tenantID, departmentID, contactMechanismID uuid.UUID,
	contactType ContactType,
	order int,
	isPrimary bool,
	now time.Time,
) (*DepartmentContactMechanism, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate department contact id: %w", err)
	}
	now = now.UTC()
	relation := &DepartmentContactMechanism{
		ID: id, TenantID: tenantID, DepartmentID: departmentID,
		ContactMechanismID: contactMechanismID, ContactType: contactType,
		ContactMechanismOrder: order, IsPrimary: isPrimary,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := relation.Validate(); err != nil {
		return nil, err
	}
	return relation, nil
}

func (d *DepartmentContactMechanism) Validate() error {
	if d == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidDepartmentContact)
	}
	if d.ID == uuid.Nil || d.TenantID == uuid.Nil ||
		d.DepartmentID == uuid.Nil || d.ContactMechanismID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidDepartmentContact)
	}
	if !d.ContactType.Valid() {
		return fmt.Errorf("%w: invalid contact_type", ErrInvalidDepartmentContact)
	}
	if d.ContactMechanismOrder < 0 {
		return fmt.Errorf("%w: order cannot be negative", ErrInvalidDepartmentContact)
	}
	return nil
}

type DepartmentContactMechanismValidity struct {
	ID                           uuid.UUID
	TenantID                     uuid.UUID
	DepartmentContactMechanismID uuid.UUID
	DepartmentID                 uuid.UUID
	ContactType                  ContactType
	IsPrimary                    bool
	StartDate                    time.Time
	EndDate                      *time.Time
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func NewDepartmentContactMechanismValidity(
	tenantID, contactID, departmentID uuid.UUID,
	contactType ContactType,
	isPrimary bool,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*DepartmentContactMechanismValidity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate department contact validity id: %w", err)
	}
	now = now.UTC()
	validity := &DepartmentContactMechanismValidity{
		ID: id, TenantID: tenantID, DepartmentContactMechanismID: contactID,
		DepartmentID: departmentID, ContactType: contactType, IsPrimary: isPrimary,
		StartDate: startDate.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if endDate != nil {
		end := endDate.UTC()
		validity.EndDate = &end
	}
	if err := validity.Validate(); err != nil {
		return nil, err
	}
	return validity, nil
}

func (v *DepartmentContactMechanismValidity) Validate() error {
	if v == nil {
		return fmt.Errorf("%w: nil validity", ErrInvalidDepartmentContact)
	}
	if v.ID == uuid.Nil || v.TenantID == uuid.Nil ||
		v.DepartmentContactMechanismID == uuid.Nil || v.DepartmentID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidDepartmentContact)
	}
	if !v.ContactType.Valid() {
		return fmt.Errorf("%w: invalid contact_type", ErrInvalidDepartmentContact)
	}
	if v.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidDepartmentContact)
	}
	if v.EndDate != nil && !v.EndDate.After(v.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidDepartmentContact)
	}
	return nil
}

func (v *DepartmentContactMechanismValidity) Close(at time.Time) error {
	if v.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(v.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidDepartmentContact)
	}
	v.EndDate = &at
	v.UpdatedAt = at
	return nil
}

// DepartmentAddress is an address owned by a department.
type DepartmentAddress struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	DepartmentID   uuid.UUID
	CityLocationID uuid.UUID
	Type           AddressType
	Line1          string
	Line2          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NewDepartmentAddress(
	tenantID, departmentID, cityLocationID uuid.UUID,
	addressType AddressType,
	line1, line2 string,
	now time.Time,
) (*DepartmentAddress, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate department address id: %w", err)
	}
	now = now.UTC()
	address := &DepartmentAddress{
		ID: id, TenantID: tenantID, DepartmentID: departmentID,
		CityLocationID: cityLocationID, Type: addressType,
		Line1: strings.TrimSpace(line1), Line2: strings.TrimSpace(line2),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := address.Validate(); err != nil {
		return nil, err
	}
	return address, nil
}

func (a *DepartmentAddress) Validate() error {
	if a == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidDepartmentAddress)
	}
	if a.ID == uuid.Nil || a.TenantID == uuid.Nil ||
		a.DepartmentID == uuid.Nil || a.CityLocationID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidDepartmentAddress)
	}
	if !a.Type.Valid() {
		return fmt.Errorf("%w: invalid type", ErrInvalidDepartmentAddress)
	}
	if strings.TrimSpace(a.Line1) == "" {
		return fmt.Errorf("%w: line1 is required", ErrInvalidDepartmentAddress)
	}
	return nil
}

type DepartmentAddressValidity struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	DepartmentAddressID uuid.UUID
	DepartmentID        uuid.UUID
	Type                AddressType
	IsPrimary           bool
	StartDate           time.Time
	EndDate             *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func NewDepartmentAddressValidity(
	tenantID, addressID, departmentID uuid.UUID,
	addressType AddressType,
	isPrimary bool,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*DepartmentAddressValidity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate department address validity id: %w", err)
	}
	now = now.UTC()
	validity := &DepartmentAddressValidity{
		ID: id, TenantID: tenantID, DepartmentAddressID: addressID,
		DepartmentID: departmentID, Type: addressType, IsPrimary: isPrimary,
		StartDate: startDate.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if endDate != nil {
		end := endDate.UTC()
		validity.EndDate = &end
	}
	if err := validity.Validate(); err != nil {
		return nil, err
	}
	return validity, nil
}

func (v *DepartmentAddressValidity) Validate() error {
	if v == nil {
		return fmt.Errorf("%w: nil validity", ErrInvalidDepartmentAddress)
	}
	if v.ID == uuid.Nil || v.TenantID == uuid.Nil ||
		v.DepartmentAddressID == uuid.Nil || v.DepartmentID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidDepartmentAddress)
	}
	if !v.Type.Valid() {
		return fmt.Errorf("%w: invalid type", ErrInvalidDepartmentAddress)
	}
	if v.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidDepartmentAddress)
	}
	if v.EndDate != nil && !v.EndDate.After(v.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidDepartmentAddress)
	}
	return nil
}

func (v *DepartmentAddressValidity) Close(at time.Time) error {
	if v.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(v.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidDepartmentAddress)
	}
	v.EndDate = &at
	v.UpdatedAt = at
	return nil
}

// DepartmentMDMRepository persists department contacts, addresses and their
// independent validity windows.
type DepartmentMDMRepository interface {
	FindOrCreateContactMechanism(ctx context.Context, contact *ContactMechanism) (*ContactMechanism, error)

	CreateDepartmentContact(ctx context.Context, contact *DepartmentContactMechanism) error
	GetDepartmentContact(ctx context.Context, tenantID, id uuid.UUID) (*DepartmentContactMechanism, error)
	GetOpenDepartmentContactValidity(ctx context.Context, tenantID, contactID uuid.UUID) (*DepartmentContactMechanismValidity, error)
	GetOpenPrimaryDepartmentContactValidity(ctx context.Context, tenantID, departmentID uuid.UUID, contactType ContactType) (*DepartmentContactMechanismValidity, error)
	CreateDepartmentContactValidity(ctx context.Context, validity *DepartmentContactMechanismValidity) error
	CloseDepartmentContactValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error

	CreateDepartmentAddress(ctx context.Context, address *DepartmentAddress) error
	GetDepartmentAddress(ctx context.Context, tenantID, id uuid.UUID) (*DepartmentAddress, error)
	GetOpenDepartmentAddressValidity(ctx context.Context, tenantID, addressID uuid.UUID) (*DepartmentAddressValidity, error)
	GetOpenPrimaryDepartmentAddressValidity(ctx context.Context, tenantID, departmentID uuid.UUID, addressType AddressType) (*DepartmentAddressValidity, error)
	CreateDepartmentAddressValidity(ctx context.Context, validity *DepartmentAddressValidity) error
	CloseDepartmentAddressValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error
}
