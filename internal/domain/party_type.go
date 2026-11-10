package domain

import (
	"fmt"
	"strings"
)

// PartyType discriminates the concrete profile that a Party must have.
type PartyType string

const (
	PartyTypeIndividual   PartyType = "INDIVIDUAL"
	PartyTypeOrganization PartyType = "ORGANIZATION"
)

func (t PartyType) Valid() bool {
	switch t {
	case PartyTypeIndividual, PartyTypeOrganization:
		return true
	default:
		return false
	}
}

func ParsePartyType(value string) (PartyType, error) {
	switch PartyType(strings.ToUpper(strings.TrimSpace(value))) {
	case PartyTypeIndividual:
		return PartyTypeIndividual, nil
	case PartyTypeOrganization:
		return PartyTypeOrganization, nil
	default:
		return "", fmt.Errorf("%w: unsupported party_type %q", ErrInvalidParty, value)
	}
}
