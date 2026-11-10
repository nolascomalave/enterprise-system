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

var (
	_ domain.PartyMergeRepository = (*PartyMergeRepository)(nil)
	_ domain.Transactor           = (*PartyMergeRepository)(nil)
)

// PartyMergeRepository is the Ent implementation of the merge persistence port.
// The active transaction is carried in the tx field; a root repository uses the
// client directly and WithTx returns a tx-scoped repository.
type PartyMergeRepository struct {
	client *ent.Client
	tx     *ent.Tx
}

func NewPartyMergeRepository(client *ent.Client) *PartyMergeRepository {
	return &PartyMergeRepository{client: client}
}

func (r *PartyMergeRepository) WithTx(ctx context.Context, fn func(domain.PartyMergeRepository) error) error {
	if r == nil || r.client == nil {
		return errors.New("repository: nil ent client")
	}
	if r.tx != nil {
		return errors.New("repository: WithTx cannot be nested")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin transaction: %w", err)
	}

	txRepo := &PartyMergeRepository{client: r.client, tx: tx}
	if err := fn(txRepo); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("repository: rollback: %w", rollbackErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit transaction: %w", err)
	}
	return nil
}

func (r *PartyMergeRepository) GetParty(ctx context.Context, partyID uuid.UUID) (*domain.Party, error) {
	p, err := r.partyQuery().Where(party.ID(partyID)).Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: get party %s: %w", partyID, err)
	}
	return toDomainParty(p), nil
}

func (r *PartyMergeRepository) FindPartiesByCanonicalID(ctx context.Context, canonicalPartyID uuid.UUID) ([]*domain.Party, error) {
	parties, err := r.partyQuery().Where(party.CanonicalPartyID(canonicalPartyID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: find parties by canonical %s: %w", canonicalPartyID, err)
	}

	result := make([]*domain.Party, 0, len(parties))
	for _, p := range parties {
		result = append(result, toDomainParty(p))
	}
	return result, nil
}

func (r *PartyMergeRepository) UpdateParty(ctx context.Context, p *domain.Party) error {
	if p == nil {
		return errors.New("repository: nil party")
	}

	update := r.partyUpdateOne(p.ID).
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
}

func (r *PartyMergeRepository) CreatePartyMergeLog(ctx context.Context, entry *domain.PartyMergeLog) error {
	if entry == nil {
		return errors.New("repository: nil party merge log")
	}

	create := r.partyMergeLogCreate().
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
}

func (r *PartyMergeRepository) partyQuery() *ent.PartyQuery {
	if r.tx != nil {
		return r.tx.Party.Query()
	}
	return r.client.Party.Query()
}

func (r *PartyMergeRepository) partyUpdateOne(id uuid.UUID) *ent.PartyUpdateOne {
	if r.tx != nil {
		return r.tx.Party.UpdateOneID(id)
	}
	return r.client.Party.UpdateOneID(id)
}

func (r *PartyMergeRepository) partyMergeLogCreate() *ent.PartyMergeLogCreate {
	if r.tx != nil {
		return r.tx.PartyMergeLog.Create()
	}
	return r.client.PartyMergeLog.Create()
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
