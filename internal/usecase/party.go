package usecase

import (
	"context"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type CreatePersonProfileInput struct {
	FirstName      string
	MiddleName     string
	LastName       string
	SecondLastName string
	BirthDate      *time.Time
}

type CreateOrganizationProfileInput struct {
	LegalName string
	TradeName string
	TaxID     string
}

type CreatePartyInput struct {
	TenantID     uuid.UUID
	PartyType    domain.PartyType
	DisplayName  string
	Person       *CreatePersonProfileInput
	Organization *CreateOrganizationProfileInput
}

type PartyService struct {
	repo domain.PartyRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewPartyService(repo domain.PartyRepository, tx TransactionManager, now func() time.Time) *PartyService {
	if now == nil {
		now = time.Now
	}
	return &PartyService{repo: repo, tx: tx, now: now}
}

func (s *PartyService) Create(ctx context.Context, in CreatePartyInput) (*domain.PartyDetail, error) {
	if s == nil || s.repo == nil || s.tx == nil {
		return nil, fmt.Errorf("%w: nil dependencies", domain.ErrInvalidParty)
	}
	now := s.now()
	party, err := domain.NewParty(in.TenantID, in.PartyType, in.DisplayName, now)
	if err != nil {
		return nil, err
	}

	detail := &domain.PartyDetail{Party: party}
	switch in.PartyType {
	case domain.PartyTypeIndividual:
		if in.Person == nil {
			return nil, fmt.Errorf("%w: person_profile is required for INDIVIDUAL", domain.ErrInvalidParty)
		}
		if in.Organization != nil {
			return nil, fmt.Errorf("%w: organization_profile is not valid for INDIVIDUAL", domain.ErrInvalidParty)
		}
		profile, err := domain.NewPersonProfile(
			in.TenantID, party.ID,
			in.Person.FirstName, in.Person.MiddleName,
			in.Person.LastName, in.Person.SecondLastName,
			in.Person.BirthDate, now,
		)
		if err != nil {
			return nil, err
		}
		detail.PersonProfile = profile

	case domain.PartyTypeOrganization:
		if in.Organization == nil {
			return nil, fmt.Errorf("%w: organization_profile is required for ORGANIZATION", domain.ErrInvalidParty)
		}
		if in.Person != nil {
			return nil, fmt.Errorf("%w: person_profile is not valid for ORGANIZATION", domain.ErrInvalidParty)
		}
		profile, err := domain.NewOrganizationProfile(
			in.TenantID, party.ID,
			in.Organization.LegalName, in.Organization.TradeName, in.Organization.TaxID, now,
		)
		if err != nil {
			return nil, err
		}
		detail.OrganizationProfile = profile
	}

	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.repo.CreateParty(ctx, party, detail.PersonProfile, detail.OrganizationProfile); err != nil {
			return fmt.Errorf("create party: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *PartyService) Get(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyDetail, error) {
	if s == nil || s.repo == nil || s.tx == nil {
		return nil, fmt.Errorf("%w: nil dependencies", domain.ErrInvalidParty)
	}
	var detail *domain.PartyDetail
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		detail, err = s.repo.GetPartyDetail(ctx, tenantID, partyID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *PartyService) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Party, error) {
	if s == nil || s.repo == nil || s.tx == nil {
		return nil, fmt.Errorf("%w: nil dependencies", domain.ErrInvalidParty)
	}
	var parties []*domain.Party
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		parties, err = s.repo.ListParties(ctx, tenantID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return parties, nil
}
