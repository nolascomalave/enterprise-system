package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PartyMergeLog records a source -> target merge. It is append-only and keeps
// merged_by as a value, without an FK, so history survives actor deletion.
type PartyMergeLog struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	SourcePartyID uuid.UUID
	TargetPartyID uuid.UUID
	MergedAt      time.Time
	MergedBy      *uuid.UUID
}

func NewPartyMergeLog(
	tenantID uuid.UUID,
	sourcePartyID uuid.UUID,
	targetPartyID uuid.UUID,
	mergedBy *uuid.UUID,
	now time.Time,
) (*PartyMergeLog, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate party merge log id: %w", err)
	}

	entry := &PartyMergeLog{
		ID:            id,
		TenantID:      tenantID,
		SourcePartyID: sourcePartyID,
		TargetPartyID: targetPartyID,
		MergedAt:      now.UTC(),
		MergedBy:      mergedBy,
	}
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return entry, nil
}

func (l *PartyMergeLog) Validate() error {
	if l == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPartyMerge)
	}
	if l.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidPartyMerge)
	}
	if l.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidPartyMerge)
	}
	if l.SourcePartyID == uuid.Nil {
		return fmt.Errorf("%w: source_party_id is required", ErrInvalidPartyMerge)
	}
	if l.TargetPartyID == uuid.Nil {
		return fmt.Errorf("%w: target_party_id is required", ErrInvalidPartyMerge)
	}
	if l.SourcePartyID == l.TargetPartyID {
		return fmt.Errorf("%w: source and target must differ", ErrInvalidPartyMerge)
	}
	if l.MergedAt.IsZero() {
		return fmt.Errorf("%w: merged_at is required", ErrInvalidPartyMerge)
	}
	if l.MergedBy != nil && *l.MergedBy == uuid.Nil {
		return fmt.Errorf("%w: merged_by cannot be nil uuid", ErrInvalidPartyMerge)
	}
	return nil
}
