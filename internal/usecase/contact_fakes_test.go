package usecase

import (
	"context"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type fakeContactRepo struct {
	contact  *domain.ContactMechanism
	relation *domain.PartyContactMechanism
	err      error
}

func (r *fakeContactRepo) FindContactMechanismByTypeValue(ctx context.Context, contactType domain.ContactType, value string) (*domain.ContactMechanism, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.contact == nil {
		return nil, domain.ErrContactMechanismNotFound
	}
	return r.contact, nil
}

func (r *fakeContactRepo) FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*domain.PartyContactMechanism, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.relation == nil || r.relation.ContactMechanismID != contactMechanismID {
		return nil, domain.ErrPartyContactNotFound
	}
	if !r.relation.IsActive(at) {
		return nil, domain.ErrPartyContactNotFound
	}
	return r.relation, nil
}

func contactForCredentials(creds *auth.Credentials) (*domain.ContactMechanism, *domain.PartyContactMechanism) {
	if creds == nil {
		return nil, nil
	}
	contact := &domain.ContactMechanism{
		ID: uuid.New(), Type: domain.ContactTypeEmail, Value: "user@example.com",
	}
	relation := &domain.PartyContactMechanism{
		ID: creds.PartyContactMechanismID, TenantID: creds.TenantID, PartyID: creds.PartyID,
		ContactMechanismID: contact.ID,
		StartDate:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	return contact, relation
}
