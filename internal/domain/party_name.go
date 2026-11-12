package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PartyNameTypeCode identifies the kind of a party name.
type PartyNameTypeCode string

const (
	PartyNameTypeFirst  PartyNameTypeCode = "FIRST"
	PartyNameTypeLast   PartyNameTypeCode = "LAST"
	PartyNameTypeLegal  PartyNameTypeCode = "LEGAL"
	PartyNameTypeTrade  PartyNameTypeCode = "TRADE"
	PartyNameTypeAlias  PartyNameTypeCode = "ALIAS"
	PartyNameTypeFormer PartyNameTypeCode = "FORMER"
)

func (c PartyNameTypeCode) Valid() bool {
	switch c {
	case PartyNameTypeFirst, PartyNameTypeLast, PartyNameTypeLegal,
		PartyNameTypeTrade, PartyNameTypeAlias, PartyNameTypeFormer:
		return true
	default:
		return false
	}
}

// NameType is a row of the global.name_types catalog.
type NameType struct {
	ID     uuid.UUID
	Code   string
	Name   string
	System bool
}

// PartyName is the MDM value. Its dates live in PartyNameValidity.
type PartyName struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	NameTypeID uuid.UUID
	Value      string
	IsPrimary  bool
	Ordering   int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func NewPartyName(
	tenantID, partyID, nameTypeID uuid.UUID,
	value string,
	isPrimary bool,
	ordering int,
	now time.Time,
) (*PartyName, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party name id: %w", err)
	}
	now = now.UTC()
	name := &PartyName{
		ID: id, TenantID: tenantID, PartyID: partyID, NameTypeID: nameTypeID,
		Value: strings.TrimSpace(value), IsPrimary: isPrimary, Ordering: ordering,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := name.Validate(); err != nil {
		return nil, err
	}
	return name, nil
}

func (n *PartyName) Validate() error {
	if n == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyName)
	}
	if n.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidPartyName)
	}
	if n.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidPartyName)
	}
	if n.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidPartyName)
	}
	if n.NameTypeID == uuid.Nil {
		return fmt.Errorf("%w: name_type_id is required", ErrInvalidPartyName)
	}
	if strings.TrimSpace(n.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidPartyName)
	}
	if n.Ordering < 0 {
		return fmt.Errorf("%w: ordering cannot be negative", ErrInvalidPartyName)
	}
	if n.CreatedAt.IsZero() || n.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidPartyName)
	}
	return nil
}

// Update refreshes the mutable attributes and reactivates a soft-deleted name.
func (n *PartyName) Update(value string, isPrimary bool, ordering int, now time.Time) error {
	if n == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyName)
	}
	n.Value = strings.TrimSpace(value)
	n.IsPrimary = isPrimary
	n.Ordering = ordering
	n.DeletedAt = nil
	n.UpdatedAt = now.UTC()
	return n.Validate()
}

// PartyNameValidity is a time window in which a name applies. IsPrimary is a
// snapshot used by the partial unique index "one primary open per party/type".
type PartyNameValidity struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	PartyNameID uuid.UUID
	PartyID     uuid.UUID
	NameTypeID  uuid.UUID
	IsPrimary   bool
	StartDate   time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewPartyNameValidity(
	tenantID, partyNameID, partyID, nameTypeID uuid.UUID,
	isPrimary bool,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*PartyNameValidity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party name validity id: %w", err)
	}
	now = now.UTC()
	validity := &PartyNameValidity{
		ID: id, TenantID: tenantID, PartyNameID: partyNameID,
		PartyID: partyID, NameTypeID: nameTypeID, IsPrimary: isPrimary,
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

func (v *PartyNameValidity) Validate() error {
	if v == nil {
		return fmt.Errorf("%w: nil validity", ErrInvalidPartyName)
	}
	if v.ID == uuid.Nil || v.TenantID == uuid.Nil || v.PartyNameID == uuid.Nil ||
		v.PartyID == uuid.Nil || v.NameTypeID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidPartyName)
	}
	if v.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidPartyName)
	}
	if v.EndDate != nil && !v.EndDate.After(v.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidPartyName)
	}
	return nil
}

func (v *PartyNameValidity) IsActive(at time.Time) bool {
	if v == nil {
		return false
	}
	at = at.UTC()
	return !v.StartDate.After(at) && (v.EndDate == nil || v.EndDate.After(at))
}

func (v *PartyNameValidity) Close(at time.Time) error {
	if v.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(v.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidPartyName)
	}
	v.EndDate = &at
	v.UpdatedAt = at
	return nil
}

// PartyNameRepository persists names and their independent validity windows.
type PartyNameRepository interface {
	FindNameTypeByCode(ctx context.Context, code string) (*NameType, error)
	FindPartyName(ctx context.Context, tenantID, partyID, nameTypeID uuid.UUID, value string) (*PartyName, error)
	GetPartyName(ctx context.Context, tenantID, nameID uuid.UUID) (*PartyName, error)
	CreatePartyName(ctx context.Context, name *PartyName) error
	UpdatePartyName(ctx context.Context, name *PartyName) error

	GetOpenValidity(ctx context.Context, tenantID, partyNameID uuid.UUID) (*PartyNameValidity, error)
	GetOpenPrimaryValidity(ctx context.Context, tenantID, partyID, nameTypeID uuid.UUID) (*PartyNameValidity, error)
	CreateValidity(ctx context.Context, validity *PartyNameValidity) error
	CloseValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error
}
