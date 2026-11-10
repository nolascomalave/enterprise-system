package usecase

import (
	"context"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type PartyContacts struct {
	repo domain.PartyContactRepository
	now  func() time.Time
}

func NewPartyContacts(repo domain.PartyContactRepository, now func() time.Time) *PartyContacts {
	if now == nil {
		now = time.Now
	}
	return &PartyContacts{repo: repo, now: now}
}

type CreateContactMechanismInput struct {
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	Type       domain.ContactType
	Value      string
	IsPrimary  bool
	IsVerified bool
}

type UpdateContactMechanismInput struct {
	ID         uuid.UUID
	Type       domain.ContactType
	Value      string
	IsPrimary  bool
	IsVerified bool
}

func (s *PartyContacts) CreateContactMechanism(ctx context.Context, in CreateContactMechanismInput) (*domain.ContactMechanism, error) {
	entity, err := domain.NewContactMechanism(in.TenantID, in.PartyID, in.Type, in.Value,
		in.IsPrimary, in.IsVerified, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateContactMechanism(ctx, entity); err != nil {
		return nil, fmt.Errorf("create contact mechanism: %w", err)
	}
	return entity, nil
}

func (s *PartyContacts) GetContactMechanism(ctx context.Context, id uuid.UUID) (*domain.ContactMechanism, error) {
	return s.repo.GetContactMechanism(ctx, id)
}

func (s *PartyContacts) ListContactMechanisms(ctx context.Context, partyID uuid.UUID) ([]*domain.ContactMechanism, error) {
	return s.repo.ListContactMechanisms(ctx, partyID)
}

func (s *PartyContacts) UpdateContactMechanism(ctx context.Context, in UpdateContactMechanismInput) (*domain.ContactMechanism, error) {
	entity, err := s.repo.GetContactMechanism(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.Type, in.Value, in.IsPrimary, in.IsVerified, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateContactMechanism(ctx, entity); err != nil {
		return nil, fmt.Errorf("update contact mechanism: %w", err)
	}
	return entity, nil
}

func (s *PartyContacts) DeleteContactMechanism(ctx context.Context, id uuid.UUID) error {
	entity, err := s.repo.GetContactMechanism(ctx, id)
	if err != nil {
		return err
	}
	if err := entity.SoftDelete(s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateContactMechanism(ctx, entity); err != nil {
		return fmt.Errorf("delete contact mechanism: %w", err)
	}
	return nil
}

type CreateAddressInput struct {
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	Type       domain.AddressType
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
	IsPrimary  bool
}

type UpdateAddressInput struct {
	ID         uuid.UUID
	Type       domain.AddressType
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
	IsPrimary  bool
}

func (s *PartyContacts) CreateAddress(ctx context.Context, in CreateAddressInput) (*domain.Address, error) {
	entity, err := domain.NewAddress(in.TenantID, in.PartyID, in.Type, in.Line1, in.Line2,
		in.City, in.State, in.PostalCode, in.Country, in.IsPrimary, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateAddress(ctx, entity); err != nil {
		return nil, fmt.Errorf("create address: %w", err)
	}
	return entity, nil
}

func (s *PartyContacts) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	return s.repo.GetAddress(ctx, id)
}

func (s *PartyContacts) ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*domain.Address, error) {
	return s.repo.ListAddresses(ctx, partyID)
}

func (s *PartyContacts) UpdateAddress(ctx context.Context, in UpdateAddressInput) (*domain.Address, error) {
	entity, err := s.repo.GetAddress(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.Type, in.Line1, in.Line2, in.City, in.State,
		in.PostalCode, in.Country, in.IsPrimary, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateAddress(ctx, entity); err != nil {
		return nil, fmt.Errorf("update address: %w", err)
	}
	return entity, nil
}

func (s *PartyContacts) DeleteAddress(ctx context.Context, id uuid.UUID) error {
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
}
