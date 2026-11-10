package repository

import (
	"context"
	"errors"
	"fmt"

	"enterprise-system/ent"
	"enterprise-system/ent/organizationprofile"
	"enterprise-system/ent/party"
	"enterprise-system/ent/personprofile"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.PartyRepository = (*PartyRepository)(nil)

type PartyRepository struct {
	client *ent.Client
}

func NewPartyRepository(client *ent.Client) *PartyRepository {
	return &PartyRepository{client: client}
}

func (r *PartyRepository) CreateParty(
	ctx context.Context,
	p *domain.Party,
	person *domain.PersonProfile,
	organization *domain.OrganizationProfile,
) error {
	if r == nil || r.client == nil {
		return errors.New("repository: nil ent client")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin transaction: %w", err)
	}

	createParty := tx.Party.Create().
		SetID(p.ID).
		SetTenantID(p.TenantID).
		SetPartyType(party.PartyType(p.PartyType)).
		SetDisplayName(p.DisplayName).
		SetRecordStatus(party.RecordStatus(p.RecordStatus)).
		SetCreatedAt(p.CreatedAt).
		SetUpdatedAt(p.UpdatedAt)
	if p.CanonicalPartyID != nil {
		createParty.SetCanonicalPartyID(*p.CanonicalPartyID)
	}
	if _, err := createParty.Save(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("repository: create party: %w", err)
	}

	if person != nil {
		createPerson := tx.PersonProfile.Create().
			SetID(person.ID).
			SetTenantID(person.TenantID).
			SetPartyID(person.PartyID).
			SetFirstName(person.FirstName).
			SetLastName(person.LastName).
			SetCreatedAt(person.CreatedAt).
			SetUpdatedAt(person.UpdatedAt)
		if person.MiddleName != "" {
			createPerson.SetMiddleName(person.MiddleName)
		}
		if person.SecondLastName != "" {
			createPerson.SetSecondLastName(person.SecondLastName)
		}
		if person.BirthDate != nil {
			createPerson.SetBirthDate(*person.BirthDate)
		}
		if _, err := createPerson.Save(ctx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("repository: create person profile: %w", err)
		}
	}

	if organization != nil {
		createOrg := tx.OrganizationProfile.Create().
			SetID(organization.ID).
			SetTenantID(organization.TenantID).
			SetPartyID(organization.PartyID).
			SetLegalName(organization.LegalName).
			SetTaxID(organization.TaxID).
			SetCreatedAt(organization.CreatedAt).
			SetUpdatedAt(organization.UpdatedAt)
		if organization.TradeName != "" {
			createOrg.SetTradeName(organization.TradeName)
		}
		if _, err := createOrg.Save(ctx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("repository: create organization profile: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: commit party: %w", err)
	}
	return nil
}

func (r *PartyRepository) GetPartyDetail(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyDetail, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("repository: nil ent client")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: begin transaction: %w", err)
	}

	row, err := tx.Party.Query().
		Where(
			party.ID(partyID),
			party.TenantID(tenantID),
			party.DeletedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, domain.ErrPartyNotFound
		}
		return nil, fmt.Errorf("repository: get party: %w", err)
	}

	detail := &domain.PartyDetail{Party: toDomainParty(row)}

	person, err := tx.PersonProfile.Query().
		Where(personprofile.PartyID(partyID), personprofile.DeletedAtIsNil()).
		Only(ctx)
	switch {
	case err == nil:
		detail.PersonProfile = toDomainPersonProfile(person)
	case !ent.IsNotFound(err):
		_ = tx.Rollback()
		return nil, fmt.Errorf("repository: get person profile: %w", err)
	}

	organization, err := tx.OrganizationProfile.Query().
		Where(organizationprofile.PartyID(partyID), organizationprofile.DeletedAtIsNil()).
		Only(ctx)
	switch {
	case err == nil:
		detail.OrganizationProfile = toDomainOrganizationProfile(organization)
	case !ent.IsNotFound(err):
		_ = tx.Rollback()
		return nil, fmt.Errorf("repository: get organization profile: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("repository: commit read party: %w", err)
	}
	return detail, nil
}

func (r *PartyRepository) ListParties(ctx context.Context, tenantID uuid.UUID) ([]*domain.Party, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("repository: nil ent client")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: begin transaction: %w", err)
	}

	rows, err := tx.Party.Query().
		Where(
			party.TenantID(tenantID),
			party.DeletedAtIsNil(),
			party.RecordStatusEQ(party.RecordStatusACTIVE),
		).
		All(ctx)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("repository: list parties: %w", err)
	}

	result := make([]*domain.Party, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainParty(row))
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("repository: commit list parties: %w", err)
	}
	return result, nil
}

func toDomainPersonProfile(e *ent.PersonProfile) *domain.PersonProfile {
	return &domain.PersonProfile{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		FirstName: e.FirstName, MiddleName: e.MiddleName,
		LastName: e.LastName, SecondLastName: e.SecondLastName,
		BirthDate: e.BirthDate, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}

func toDomainOrganizationProfile(e *ent.OrganizationProfile) *domain.OrganizationProfile {
	return &domain.OrganizationProfile{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		LegalName: e.LegalName, TradeName: e.TradeName, TaxID: e.TaxID,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}
