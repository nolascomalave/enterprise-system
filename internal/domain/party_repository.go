package domain

import (
	"context"

	"github.com/google/uuid"
)

// PartyDetail is the aggregate returned by the Party API.
type PartyDetail struct {
	Party               *Party
	PersonProfile       *PersonProfile
	OrganizationProfile *OrganizationProfile
}

// PartyRepository is the persistence port for parties and their 1:1 profiles.
type PartyRepository interface {
	CreateParty(ctx context.Context, party *Party, person *PersonProfile, organization *OrganizationProfile) error
	GetPartyDetail(ctx context.Context, tenantID, partyID uuid.UUID) (*PartyDetail, error)
	ListParties(ctx context.Context, tenantID uuid.UUID) ([]*Party, error)
}
