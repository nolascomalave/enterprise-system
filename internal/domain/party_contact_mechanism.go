package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PartyContactMechanism is the tenant-scoped association between a Party and a
// global ContactMechanism, with validity and ordering.
type PartyContactMechanism struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	PartyID               uuid.UUID
	ContactMechanismID    uuid.UUID
	IsPrimary             bool
	IsVerified            bool
	ContactMechanismOrder int
	StartDate             time.Time
	EndDate               *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type PartyContact struct {
	Relation *PartyContactMechanism
	Contact  *ContactMechanism
}

func NewPartyContactMechanism(
	tenantID, partyID, contactMechanismID uuid.UUID,
	isPrimary, isVerified bool,
	order int,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*PartyContactMechanism, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party contact mechanism id: %w", err)
	}
	now = now.UTC()
	startDate = startDate.UTC()
	entity := &PartyContactMechanism{
		ID: id, TenantID: tenantID, PartyID: partyID, ContactMechanismID: contactMechanismID,
		IsPrimary: isPrimary, IsVerified: isVerified, ContactMechanismOrder: order,
		StartDate: startDate, CreatedAt: now, UpdatedAt: now,
	}
	if endDate != nil {
		end := endDate.UTC()
		entity.EndDate = &end
	}
	if err := entity.Validate(); err != nil {
		return nil, err
	}
	return entity, nil
}

func (p *PartyContactMechanism) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyContactMechanism)
	}
	if p.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidPartyContactMechanism)
	}
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidPartyContactMechanism)
	}
	if p.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidPartyContactMechanism)
	}
	if p.ContactMechanismID == uuid.Nil {
		return fmt.Errorf("%w: contact_mechanism_id is required", ErrInvalidPartyContactMechanism)
	}
	if p.ContactMechanismOrder < 0 {
		return fmt.Errorf("%w: contact_mechanism_order cannot be negative", ErrInvalidPartyContactMechanism)
	}
	if p.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidPartyContactMechanism)
	}
	if p.EndDate != nil && !p.EndDate.After(p.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidPartyContactMechanism)
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidPartyContactMechanism)
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidPartyContactMechanism)
	}
	return nil
}

func (p *PartyContactMechanism) IsActive(at time.Time) bool {
	if p == nil {
		return false
	}
	at = at.UTC()
	return !p.StartDate.After(at) && (p.EndDate == nil || p.EndDate.After(at))
}

func (p *PartyContactMechanism) Close(at time.Time) error {
	if p.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(p.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidPartyContactMechanism)
	}
	p.EndDate = &at
	p.UpdatedAt = at
	return nil
}

func (p *PartyContactMechanism) Update(isPrimary, isVerified bool, order int, endDate *time.Time, now time.Time) error {
	if p == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyContactMechanism)
	}
	if p.EndDate != nil {
		return ErrAlreadyDeleted
	}
	if order < 0 {
		return fmt.Errorf("%w: contact_mechanism_order cannot be negative", ErrInvalidPartyContactMechanism)
	}
	p.IsPrimary = isPrimary
	p.IsVerified = isVerified
	p.ContactMechanismOrder = order
	if endDate != nil {
		end := endDate.UTC()
		if !end.After(p.StartDate) {
			return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidPartyContactMechanism)
		}
		p.EndDate = &end
	}
	p.UpdatedAt = now.UTC()
	return p.Validate()
}
