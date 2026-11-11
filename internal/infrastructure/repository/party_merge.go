package repository

import (
	"context"
	"errors"
	"fmt"

	"enterprise-system/ent"
	"enterprise-system/ent/party"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.PartyMergeRepository = (*PartyMergeRepository)(nil)

// PartyMergeRepository is the Ent implementation of the merge persistence
// port. It always uses the ambient transaction opened by TransactionManager.
type PartyMergeRepository struct {
	client *ent.Client
}

func NewPartyMergeRepository(client *ent.Client) *PartyMergeRepository {
	return &PartyMergeRepository{client: client}
}

func (r *PartyMergeRepository) GetParty(ctx context.Context, partyID uuid.UUID) (*domain.Party, error) {
	var result *domain.Party
	err := execute(ctx, r.client, func(client *ent.Client) error {
		p, err := client.Party.Query().Where(party.ID(partyID)).Only(ctx)
		if err != nil {
			return fmt.Errorf("repository: get party %s: %w", partyID, err)
		}
		result = toDomainParty(p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PartyMergeRepository) FindPartiesByCanonicalID(ctx context.Context, canonicalPartyID uuid.UUID) ([]*domain.Party, error) {
	var result []*domain.Party
	err := execute(ctx, r.client, func(client *ent.Client) error {
		parties, err := client.Party.Query().Where(party.CanonicalPartyID(canonicalPartyID)).All(ctx)
		if err != nil {
			return fmt.Errorf("repository: find parties by canonical %s: %w", canonicalPartyID, err)
		}
		result = make([]*domain.Party, 0, len(parties))
		for _, p := range parties {
			result = append(result, toDomainParty(p))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *PartyMergeRepository) UpdateParty(ctx context.Context, p *domain.Party) error {
	if p == nil {
		return errors.New("repository: nil party")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		update := client.Party.UpdateOneID(p.ID).
			SetRecordStatus(party.RecordStatus(p.RecordStatus)).
			SetUpdatedAt(p.UpdatedAt)

		if p.CanonicalPartyID != nil {
			update.SetCanonicalPartyID(*p.CanonicalPartyID)
		} else {
			update.ClearCanonicalPartyID()
		}
		if p.DeletedAt != nil {
			update.SetDeletedAt(*p.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}

		if _, err := update.Save(ctx); err != nil {
			return fmt.Errorf("repository: update party %s: %w", p.ID, err)
		}
		return nil
	})
}

func (r *PartyMergeRepository) CreatePartyMergeLog(ctx context.Context, entry *domain.PartyMergeLog) error {
	if entry == nil {
		return errors.New("repository: nil party merge log")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.PartyMergeLog.Create().
			SetID(entry.ID).
			SetTenantID(entry.TenantID).
			SetSourcePartyID(entry.SourcePartyID).
			SetTargetPartyID(entry.TargetPartyID).
			SetMergedAt(entry.MergedAt)

		if entry.MergedBy != nil {
			create.SetMergedBy(*entry.MergedBy)
		}

		if _, err := create.Save(ctx); err != nil {
			return fmt.Errorf("repository: create party merge log: %w", err)
		}
		return nil
	})
}

func toDomainParty(p *ent.Party) *domain.Party {
	return &domain.Party{
		ID:               p.ID,
		TenantID:         p.TenantID,
		PartyType:        domain.PartyType(p.PartyType),
		DisplayName:      p.DisplayName,
		CanonicalPartyID: p.CanonicalPartyID,
		RecordStatus:     domain.PartyRecordStatus(p.RecordStatus),
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		DeletedAt:        p.DeletedAt,
	}
}
