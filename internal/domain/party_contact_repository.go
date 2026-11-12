package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PartyContactRepository persists global contacts, their tenant-scoped party
// relations, and addresses.
type PartyContactRepository interface {
	FindContactMechanismByTypeValue(ctx context.Context, contactType ContactType, value string) (*ContactMechanism, error)
	FindOrCreateContactMechanism(ctx context.Context, contact *ContactMechanism) (*ContactMechanism, error)
	CreatePartyContactMechanism(ctx context.Context, relation *PartyContactMechanism) error
	GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*PartyContact, error)
	ListPartyContacts(ctx context.Context, tenantID, partyID uuid.UUID) ([]*PartyContact, error)
	UpdatePartyContactMechanism(ctx context.Context, relation *PartyContactMechanism) error
	FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*PartyContactMechanism, error)

	CreateAddress(ctx context.Context, address *Address) error
	GetAddress(ctx context.Context, id uuid.UUID) (*Address, error)
	ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*Address, error)
	UpdateAddress(ctx context.Context, address *Address) error
	GetOpenAddressValidity(ctx context.Context, tenantID, addressID uuid.UUID) (*AddressValidity, error)
	GetOpenPrimaryAddressValidity(ctx context.Context, tenantID, partyID uuid.UUID, addressType AddressType) (*AddressValidity, error)
	CreateAddressValidity(ctx context.Context, validity *AddressValidity) error
	CloseAddressValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error
}
