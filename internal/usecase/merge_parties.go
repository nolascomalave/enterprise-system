package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// MergeParties is the application use case that merges a source party into a
// target Golden Record inside a single transaction.
type MergeParties struct {
	tx      TransactionManager
	repo    domain.PartyMergeRepository
	now     func() time.Time
	actorID *uuid.UUID
}

func NewMergeParties(
	tx TransactionManager,
	repo domain.PartyMergeRepository,
	now func() time.Time,
	actorID *uuid.UUID,
) *MergeParties {
	if now == nil {
		now = time.Now
	}
	return &MergeParties{tx: tx, repo: repo, now: now, actorID: actorID}
}

func (uc *MergeParties) Execute(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.MergeResult, error) {
	if uc == nil || uc.tx == nil || uc.repo == nil {
		return nil, internalError(fmt.Errorf("merge parties: nil dependencies: %w", domain.ErrInvalidPartyMerge))
	}

	var result *domain.MergeResult
	err := uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		service := domain.NewPartyMergeService(uc.repo, uc.now, uc.actorID)
		var err error
		result, err = service.MergeParties(ctx, sourceID, targetID)
		return err
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPartyMerge) {
			return nil, validationError("invalid_party_merge", "invalid party merge", err)
		}
		return nil, fmt.Errorf("merge parties: %w", err)
	}
	return result, nil
}
