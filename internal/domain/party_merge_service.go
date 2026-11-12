package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PartyMergeRepository is the persistence port required by the merge service.
// Implementations must run under the tenant RLS context.
type PartyMergeRepository interface {
	GetParty(ctx context.Context, partyID uuid.UUID) (*Party, error)
	FindPartiesByCanonicalID(ctx context.Context, canonicalPartyID uuid.UUID) ([]*Party, error)
	UpdateParty(ctx context.Context, party *Party) error
	CreatePartyMergeLog(ctx context.Context, log *PartyMergeLog) error
}

type MergeResult struct {
	SourceID           uuid.UUID
	TargetID           uuid.UUID
	CompressedPartyIDs []uuid.UUID
	MergedAt           time.Time
}

type PartyMergeService struct {
	repo    PartyMergeRepository
	now     func() time.Time
	actorID *uuid.UUID
}

func NewPartyMergeService(repo PartyMergeRepository, now func() time.Time, actorID *uuid.UUID) *PartyMergeService {
	if now == nil {
		now = time.Now
	}
	return &PartyMergeService{repo: repo, now: now, actorID: actorID}
}

// MergeParties performs the Golden Record merge and path compression:
//  1. source becomes MERGED, receives canonical_party_id = target and deleted_at.
//  2. every party that pointed to source is re-pointed to target (transitively).
//  3. the merge is recorded in party_merge_log.
//
// Callers must run this inside a transaction; the usecase provides it.
func (s *PartyMergeService) MergeParties(ctx context.Context, sourceID, targetID uuid.UUID) (*MergeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: nil repository", ErrInvalidPartyMerge)
	}
	if sourceID == uuid.Nil || targetID == uuid.Nil {
		return nil, fmt.Errorf("%w: source and target are required", ErrInvalidPartyMerge)
	}
	if sourceID == targetID {
		return nil, fmt.Errorf("%w: source and target must differ", ErrInvalidPartyMerge)
	}

	source, err := s.repo.GetParty(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("get source party: %w", err)
	}
	target, err := s.repo.GetParty(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("get target party: %w", err)
	}

	if err := source.Validate(); err != nil {
		return nil, fmt.Errorf("invalid source party: %w", err)
	}
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("invalid target party: %w", err)
	}
	if err := validateMergeEndpoints(source, target); err != nil {
		return nil, err
	}

	now := s.now().UTC()
	compressed, err := s.compressPath(ctx, source, targetID, now)
	if err != nil {
		return nil, err
	}

	if err := source.MergeInto(targetID, now); err != nil {
		return nil, fmt.Errorf("merge source party: %w", err)
	}
	if err := s.repo.UpdateParty(ctx, source); err != nil {
		return nil, fmt.Errorf("update merged source party: %w", err)
	}

	entry, err := NewPartyMergeLog(source.TenantID, sourceID, targetID, s.actorID, now)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreatePartyMergeLog(ctx, entry); err != nil {
		return nil, fmt.Errorf("create party merge log: %w", err)
	}

	return &MergeResult{
		SourceID:           sourceID,
		TargetID:           targetID,
		CompressedPartyIDs: compressed,
		MergedAt:           now,
	}, nil
}

func validateMergeEndpoints(source, target *Party) error {
	if source == nil || target == nil {
		return fmt.Errorf("%w: source and target are required", ErrInvalidPartyMerge)
	}
	if source.TenantID == uuid.Nil || target.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant is required", ErrInvalidPartyMerge)
	}
	if source.TenantID != target.TenantID {
		return fmt.Errorf("%w: source tenant %s, target tenant %s", ErrPartyTenantMismatch, source.TenantID, target.TenantID)
	}
	if source.RecordStatus != PartyRecordStatusActive || source.DeletedAt != nil {
		return fmt.Errorf("%w: source must be ACTIVE and not deleted", ErrPartyNotMergeable)
	}
	if source.CanonicalPartyID != nil {
		return fmt.Errorf("%w: source already has canonical_party_id", ErrPartyNotMergeable)
	}
	if target.RecordStatus != PartyRecordStatusActive || target.DeletedAt != nil {
		return fmt.Errorf("%w: target must be ACTIVE and not deleted", ErrPartyMergeTargetNotActive)
	}
	if target.CanonicalPartyID != nil {
		return fmt.Errorf("%w: target cannot have canonical_party_id", ErrPartyMergeTargetNotActive)
	}
	return nil
}

func (s *PartyMergeService) compressPath(ctx context.Context, source *Party, targetID uuid.UUID, now time.Time) ([]uuid.UUID, error) {
	visited := map[uuid.UUID]struct{}{source.ID: {}}
	queue := []uuid.UUID{source.ID}
	compressed := make([]uuid.UUID, 0)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		dependents, err := s.repo.FindPartiesByCanonicalID(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("find parties pointing to %s: %w", current, err)
		}

		for _, dependent := range dependents {
			if dependent == nil {
				continue
			}
			if _, seen := visited[dependent.ID]; seen {
				continue
			}
			visited[dependent.ID] = struct{}{}

			if dependent.TenantID != source.TenantID {
				return nil, fmt.Errorf("%w: dependent %s belongs to another tenant", ErrPartyTenantMismatch, dependent.ID)
			}

			// Enqueue before mutating so descendants of this node are also
			// compressed to the final Golden Record.
			queue = append(queue, dependent.ID)

			if err := dependent.CompressTo(targetID, now); err != nil {
				return nil, fmt.Errorf("compress party %s: %w", dependent.ID, err)
			}
			if err := s.repo.UpdateParty(ctx, dependent); err != nil {
				return nil, fmt.Errorf("update compressed party %s: %w", dependent.ID, err)
			}
			compressed = append(compressed, dependent.ID)
		}
	}

	return compressed, nil
}
