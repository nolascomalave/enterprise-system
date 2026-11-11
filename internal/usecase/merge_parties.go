package usecase

import (
	"context"
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
		return nil, fmt.Errorf("%w: nil dependencies", domain.ErrInvalidPartyMerge)
	}

	var result *domain.MergeResult
	err := uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		service := domain.NewPartyMergeService(uc.repo, uc.now, uc.actorID)
		var err error
		result, err = service.MergeParties(ctx, sourceID, targetID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
