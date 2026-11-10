package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PartyRecordStatus is the identity lifecycle of a party.
type PartyRecordStatus string

const (
	PartyRecordStatusActive   PartyRecordStatus = "ACTIVE"
	PartyRecordStatusMerged   PartyRecordStatus = "MERGED"
	PartyRecordStatusArchived PartyRecordStatus = "ARCHIVED"
)

func (s PartyRecordStatus) Valid() bool {
	switch s {
	case PartyRecordStatusActive, PartyRecordStatusMerged, PartyRecordStatusArchived:
		return true
	default:
		return false
	}
}

// Party is the tenant-scoped identity root. It lives in tenants.parties and is
// protected by RLS.
type Party struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	PartyType        PartyType
	DisplayName      string
	CanonicalPartyID *uuid.UUID
	RecordStatus     PartyRecordStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func NewParty(tenantID uuid.UUID, partyType PartyType, displayName string, now time.Time) (*Party, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party id: %w", err)
	}

	now = now.UTC()
	party := &Party{
		ID:           id,
		TenantID:     tenantID,
		PartyType:    partyType,
		DisplayName:  strings.TrimSpace(displayName),
		RecordStatus: PartyRecordStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := party.Validate(); err != nil {
		return nil, err
	}
	return party, nil
}

func (p *Party) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidParty)
	}
	if p.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidParty)
	}
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidParty)
	}
	if !p.PartyType.Valid() {
		return fmt.Errorf("%w: unsupported party_type %q", ErrInvalidParty, p.PartyType)
	}
	if strings.TrimSpace(p.DisplayName) == "" {
		return fmt.Errorf("%w: display_name is required", ErrInvalidParty)
	}
	if !p.RecordStatus.Valid() {
		return fmt.Errorf("%w: unsupported record_status %q", ErrInvalidParty, p.RecordStatus)
	}
	if p.CanonicalPartyID != nil {
		if *p.CanonicalPartyID == uuid.Nil {
			return fmt.Errorf("%w: canonical_party_id cannot be nil uuid", ErrInvalidParty)
		}
		if *p.CanonicalPartyID == p.ID {
			return fmt.Errorf("%w: canonical_party_id cannot point to itself", ErrInvalidParty)
		}
	}
	if p.RecordStatus == PartyRecordStatusActive && p.CanonicalPartyID != nil {
		return fmt.Errorf("%w: ACTIVE party cannot have canonical_party_id", ErrInvalidParty)
	}
	if p.RecordStatus == PartyRecordStatusMerged {
		if p.CanonicalPartyID == nil {
			return fmt.Errorf("%w: MERGED party requires canonical_party_id", ErrInvalidParty)
		}
		if p.DeletedAt == nil {
			return fmt.Errorf("%w: MERGED party requires deleted_at", ErrInvalidParty)
		}
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidParty)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidParty)
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidParty)
	}
	if p.DeletedAt != nil && p.DeletedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidParty)
	}
	return nil
}

func (p *Party) Rename(displayName string, now time.Time) error {
	if p.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return fmt.Errorf("%w: display_name is required", ErrInvalidParty)
	}
	p.DisplayName = displayName
	p.UpdatedAt = now.UTC()
	return nil
}

func (p *Party) SoftDelete(now time.Time) error {
	if p.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	p.DeletedAt = &now
	p.UpdatedAt = now
	return nil
}

// MergeInto marks the party as MERGED and points it at the Golden Record.
func (p *Party) MergeInto(targetID uuid.UUID, now time.Time) error {
	if p.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	if p.RecordStatus != PartyRecordStatusActive {
		return fmt.Errorf("%w: only ACTIVE parties can be merged", ErrInvalidParty)
	}
	if targetID == uuid.Nil || targetID == p.ID {
		return fmt.Errorf("%w: invalid merge target", ErrInvalidParty)
	}

	now = now.UTC()
	p.RecordStatus = PartyRecordStatusMerged
	p.CanonicalPartyID = &targetID
	p.DeletedAt = &now
	p.UpdatedAt = now
	return p.Validate()
}

// CompressTo re-points an already MERGED party directly at the final Golden
// Record. This is the path-compression step of a merge.
func (p *Party) CompressTo(targetID uuid.UUID, now time.Time) error {
	if p.RecordStatus == PartyRecordStatusActive || p.CanonicalPartyID == nil {
		return fmt.Errorf("%w: only already-merged/archived parties can be path-compressed", ErrInvalidParty)
	}
	if targetID == uuid.Nil || targetID == p.ID {
		return fmt.Errorf("%w: invalid compression target", ErrInvalidParty)
	}

	now = now.UTC()
	p.CanonicalPartyID = &targetID
	p.UpdatedAt = now
	return p.Validate()
}
