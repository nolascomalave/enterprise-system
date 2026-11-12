package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type PartyContacts struct {
	repo domain.PartyContactRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewPartyContacts(repo domain.PartyContactRepository, tx TransactionManager, now func() time.Time) *PartyContacts {
	if now == nil {
		now = time.Now
	}
	return &PartyContacts{repo: repo, tx: tx, now: now}
}

func (s *PartyContacts) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.repo == nil || s.tx == nil {
		return errors.New("party contacts: nil dependencies")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

type CreateContactMechanismInput struct {
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	Type       domain.ContactType
	Value      string
	IsPrimary  bool
	IsVerified bool
	Order      int
	StartDate  *time.Time
	EndDate    *time.Time
}

type UpdateContactMechanismInput struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	IsPrimary  bool
	IsVerified bool
	Order      int
	EndDate    *time.Time
}

func (s *PartyContacts) CreateContactMechanism(ctx context.Context, in CreateContactMechanismInput) (*domain.PartyContact, error) {
	contact, err := domain.NewContactMechanism(in.Type, in.Value)
	if err != nil {
		return nil, err
	}

	var relation *domain.PartyContactMechanism
	err = s.withinTx(ctx, func(ctx context.Context) error {
		found, err := s.repo.FindOrCreateContactMechanism(ctx, contact)
		if err != nil {
			return fmt.Errorf("find or create contact mechanism: %w", err)
		}
		contact = found

		now := s.now().UTC()
		startDate := now
		if in.StartDate != nil {
			startDate = in.StartDate.UTC()
		}
		// The dates below are persisted by the repository in
		// tenants.party_contact_mechanism_validities; the relation table only
		// stores the current attributes.
		relation, err = domain.NewPartyContactMechanism(
			in.TenantID, in.PartyID, contact.ID, in.IsPrimary, in.IsVerified,
			in.Order, startDate, in.EndDate, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreatePartyContactMechanism(ctx, relation); err != nil {
			return fmt.Errorf("create party contact mechanism: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &domain.PartyContact{Relation: relation, Contact: contact}, nil
}

func (s *PartyContacts) GetContactMechanism(ctx context.Context, tenantID, id uuid.UUID) (*domain.PartyContact, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.PartyContact, error) {
		return s.repo.GetPartyContact(ctx, tenantID, id)
	})
}

func (s *PartyContacts) ListContactMechanisms(ctx context.Context, tenantID, partyID uuid.UUID) ([]*domain.PartyContact, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.PartyContact, error) {
		return s.repo.ListPartyContacts(ctx, tenantID, partyID)
	})
}

func (s *PartyContacts) UpdateContactMechanism(ctx context.Context, in UpdateContactMechanismInput) (*domain.PartyContact, error) {
	var entity *domain.PartyContact
	err := s.withinTx(ctx, func(ctx context.Context) error {
		var err error
		entity, err = s.repo.GetPartyContact(ctx, in.TenantID, in.ID)
		if err != nil {
			return err
		}
		if err := entity.Relation.Update(in.IsPrimary, in.IsVerified, in.Order, in.EndDate, s.now()); err != nil {
			return err
		}
		if err := s.repo.UpdatePartyContactMechanism(ctx, entity.Relation); err != nil {
			return fmt.Errorf("update party contact mechanism: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *PartyContacts) DeleteContactMechanism(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.withinTx(ctx, func(ctx context.Context) error {
		entity, err := s.repo.GetPartyContact(ctx, tenantID, id)
		if err != nil {
			return err
		}
		if err := entity.Relation.Close(s.now()); err != nil {
			return err
		}
		if err := s.repo.UpdatePartyContactMechanism(ctx, entity.Relation); err != nil {
			return fmt.Errorf("delete party contact mechanism: %w", err)
		}
		return nil
	})
}

type CreateAddressInput struct {
	TenantID       uuid.UUID
	PartyID        uuid.UUID
	Type           domain.AddressType
	Line1          string
	Line2          string
	CityLocationID uuid.UUID
	PostalCode     string
	Country        string
	IsPrimary      bool
}

type UpdateAddressInput struct {
	ID             uuid.UUID
	Type           domain.AddressType
	Line1          string
	Line2          string
	CityLocationID uuid.UUID
	PostalCode     string
	Country        string
	IsPrimary      bool
}

func (s *PartyContacts) CreateAddress(ctx context.Context, in CreateAddressInput) (*domain.Address, error) {
	entity, err := domain.NewAddress(in.TenantID, in.PartyID, in.Type, in.Line1, in.Line2,
		in.CityLocationID, in.PostalCode, in.Country, in.IsPrimary, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.withinTx(ctx, func(ctx context.Context) error {
		return s.repo.CreateAddress(ctx, entity)
	}); err != nil {
		return nil, fmt.Errorf("create address: %w", err)
	}
	return entity, nil
}

func (s *PartyContacts) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.Address, error) {
		return s.repo.GetAddress(ctx, id)
	})
}

func (s *PartyContacts) ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*domain.Address, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.Address, error) {
		return s.repo.ListAddresses(ctx, partyID)
	})
}

func (s *PartyContacts) UpdateAddress(ctx context.Context, in UpdateAddressInput) (*domain.Address, error) {
	var entity *domain.Address
	err := s.withinTx(ctx, func(ctx context.Context) error {
		var err error
		entity, err = s.repo.GetAddress(ctx, in.ID)
		if err != nil {
			return err
		}
		if err := entity.Update(in.Type, in.Line1, in.Line2, in.CityLocationID,
			in.PostalCode, in.Country, in.IsPrimary, s.now()); err != nil {
			return err
		}
		if err := s.repo.UpdateAddress(ctx, entity); err != nil {
			return fmt.Errorf("update address: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entity, nil
}

func (s *PartyContacts) DeleteAddress(ctx context.Context, id uuid.UUID) error {
	return s.withinTx(ctx, func(ctx context.Context) error {
		entity, err := s.repo.GetAddress(ctx, id)
		if err != nil {
			return err
		}
		if err := entity.SoftDelete(s.now()); err != nil {
			return err
		}
		if err := s.repo.UpdateAddress(ctx, entity); err != nil {
			return fmt.Errorf("delete address: %w", err)
		}
		return nil
	})
}
