package domain

import (
	"context"

	"github.com/google/uuid"
)

// PartyContactRepository is the persistence port for contacts and addresses.
type PartyContactRepository interface {
	CreateContactMechanism(ctx context.Context, contact *ContactMechanism) error
	GetContactMechanism(ctx context.Context, id uuid.UUID) (*ContactMechanism, error)
	ListContactMechanisms(ctx context.Context, partyID uuid.UUID) ([]*ContactMechanism, error)
	UpdateContactMechanism(ctx context.Context, contact *ContactMechanism) error

	CreateAddress(ctx context.Context, address *Address) error
	GetAddress(ctx context.Context, id uuid.UUID) (*Address, error)
	ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*Address, error)
	UpdateAddress(ctx context.Context, address *Address) error
}
