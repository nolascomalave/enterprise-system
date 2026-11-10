package repository

import (
	"context"
	"errors"
	"fmt"

	"enterprise-system/ent"
	"enterprise-system/ent/address"
	"enterprise-system/ent/contactmechanism"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.PartyContactRepository = (*PartyContactRepository)(nil)

type PartyContactRepository struct {
	client *ent.Client
}

func NewPartyContactRepository(client *ent.Client) *PartyContactRepository {
	return &PartyContactRepository{client: client}
}

func (r *PartyContactRepository) withTx(ctx context.Context, fn func(*ent.Tx) error) error {
	if r == nil || r.client == nil {
		return errors.New("repository: nil ent client")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit transaction: %w", err)
	}
	return nil
}

func (r *PartyContactRepository) CreateContactMechanism(ctx context.Context, e *domain.ContactMechanism) error {
	return r.withTx(ctx, func(tx *ent.Tx) error {
		_, err := tx.ContactMechanism.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetPartyID(e.PartyID).
			SetType(contactmechanism.Type(e.Type)).
			SetValue(e.Value).
			SetIsPrimary(e.IsPrimary).
			SetIsVerified(e.IsVerified).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt).
			Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) GetContactMechanism(ctx context.Context, id uuid.UUID) (*domain.ContactMechanism, error) {
	var result *domain.ContactMechanism
	err := r.withTx(ctx, func(tx *ent.Tx) error {
		e, err := tx.ContactMechanism.Query().
			Where(contactmechanism.ID(id), contactmechanism.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toDomainContactMechanism(e)
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) ListContactMechanisms(ctx context.Context, partyID uuid.UUID) ([]*domain.ContactMechanism, error) {
	var result []*domain.ContactMechanism
	err := r.withTx(ctx, func(tx *ent.Tx) error {
		rows, err := tx.ContactMechanism.Query().
			Where(contactmechanism.PartyID(partyID), contactmechanism.DeletedAtIsNil()).
			All(ctx)
		if err != nil {
			return err
		}
		result = make([]*domain.ContactMechanism, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainContactMechanism(e))
		}
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) UpdateContactMechanism(ctx context.Context, e *domain.ContactMechanism) error {
	return r.withTx(ctx, func(tx *ent.Tx) error {
		update := tx.ContactMechanism.UpdateOneID(e.ID).
			SetType(contactmechanism.Type(e.Type)).
			SetValue(e.Value).
			SetIsPrimary(e.IsPrimary).
			SetIsVerified(e.IsVerified).
			SetUpdatedAt(e.UpdatedAt)
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) CreateAddress(ctx context.Context, e *domain.Address) error {
	return r.withTx(ctx, func(tx *ent.Tx) error {
		_, err := tx.Address.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetPartyID(e.PartyID).
			SetType(address.Type(e.Type)).
			SetLine1(e.Line1).
			SetLine2(e.Line2).
			SetCity(e.City).
			SetState(e.State).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetIsPrimary(e.IsPrimary).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt).
			Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	var result *domain.Address
	err := r.withTx(ctx, func(tx *ent.Tx) error {
		e, err := tx.Address.Query().
			Where(address.ID(id), address.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toDomainAddress(e)
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*domain.Address, error) {
	var result []*domain.Address
	err := r.withTx(ctx, func(tx *ent.Tx) error {
		rows, err := tx.Address.Query().
			Where(address.PartyID(partyID), address.DeletedAtIsNil()).
			All(ctx)
		if err != nil {
			return err
		}
		result = make([]*domain.Address, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainAddress(e))
		}
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) UpdateAddress(ctx context.Context, e *domain.Address) error {
	return r.withTx(ctx, func(tx *ent.Tx) error {
		update := tx.Address.UpdateOneID(e.ID).
			SetType(address.Type(e.Type)).
			SetLine1(e.Line1).
			SetLine2(e.Line2).
			SetCity(e.City).
			SetState(e.State).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetIsPrimary(e.IsPrimary).
			SetUpdatedAt(e.UpdatedAt)
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return err
	})
}

func toDomainContactMechanism(e *ent.ContactMechanism) *domain.ContactMechanism {
	return &domain.ContactMechanism{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		Type: domain.ContactType(e.Type), Value: e.Value,
		IsPrimary: e.IsPrimary, IsVerified: e.IsVerified,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainAddress(e *ent.Address) *domain.Address {
	return &domain.Address{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		Type: domain.AddressType(e.Type), Line1: e.Line1, Line2: e.Line2,
		City: e.City, State: e.State, PostalCode: e.PostalCode, Country: e.Country,
		IsPrimary: e.IsPrimary, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}
