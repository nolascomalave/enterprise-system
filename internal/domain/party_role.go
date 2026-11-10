package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PartyRole links a tenant-scoped Party with a global RoleType.
type PartyRole struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	RoleTypeID uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func NewPartyRole(tenantID, partyID, roleTypeID uuid.UUID, now time.Time) (*PartyRole, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party role id: %w", err)
	}

	now = now.UTC()
	role := &PartyRole{
		ID:         id,
		TenantID:   tenantID,
		PartyID:    partyID,
		RoleTypeID: roleTypeID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := role.Validate(); err != nil {
		return nil, err
	}
	return role, nil
}

func (p *PartyRole) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyRole)
	}
	if p.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidPartyRole)
	}
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidPartyRole)
	}
	if p.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidPartyRole)
	}
	if p.RoleTypeID == uuid.Nil {
		return fmt.Errorf("%w: role_type_id is required", ErrInvalidPartyRole)
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidPartyRole)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidPartyRole)
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidPartyRole)
	}
	if p.DeletedAt != nil && p.DeletedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidPartyRole)
	}
	return nil
}

func (p *PartyRole) SoftDelete(now time.Time) error {
	if p.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	p.DeletedAt = &now
	p.UpdatedAt = now
	return nil
}
