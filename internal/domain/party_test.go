package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParty(t *testing.T) {
	now := testNow()

	party, err := NewParty(testTenantID, PartyTypeIndividual, "  Jane Doe  ", now)

	require.NoError(t, err)
	require.NotNil(t, party)
	assert.NotEqual(t, uuid.Nil, party.ID)
	assert.Equal(t, uuid.Version(7), party.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, testTenantID, party.TenantID)
	assert.Equal(t, PartyTypeIndividual, party.PartyType)
	assert.Equal(t, "Jane Doe", party.DisplayName)
	assert.Nil(t, party.CanonicalPartyID)
	assert.Equal(t, PartyRecordStatusActive, party.RecordStatus)
	assert.Equal(t, now, party.CreatedAt)
	assert.Equal(t, now, party.UpdatedAt)
	assert.Nil(t, party.DeletedAt)
}

func TestNewPartyValidation(t *testing.T) {
	now := testNow()

	tests := []struct {
		name        string
		tenantID    uuid.UUID
		partyType   PartyType
		displayName string
	}{
		{name: "nil tenant", tenantID: uuid.Nil, partyType: PartyTypeIndividual, displayName: "Jane"},
		{name: "invalid party type", tenantID: testTenantID, partyType: PartyType("OTHER"), displayName: "Jane"},
		{name: "empty display name", tenantID: testTenantID, partyType: PartyTypeIndividual, displayName: ""},
		{name: "blank display name", tenantID: testTenantID, partyType: PartyTypeOrganization, displayName: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			party, err := NewParty(tt.tenantID, tt.partyType, tt.displayName, now)

			require.ErrorIs(t, err, ErrInvalidParty)
			assert.Nil(t, party)
		})
	}
}

func TestParsePartyType(t *testing.T) {
	tests := map[string]PartyType{
		"individual":   PartyTypeIndividual,
		" INDIVIDUAL ": PartyTypeIndividual,
		"organization": PartyTypeOrganization,
		"ORGANIZATION": PartyTypeOrganization,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := ParsePartyType(input)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}

	_, err := ParsePartyType("OTHER")
	require.ErrorIs(t, err, ErrInvalidParty)
}

func TestPartyRename(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", now)
	require.NoError(t, err)

	require.NoError(t, party.Rename("  Jane Smith  ", later))
	assert.Equal(t, "Jane Smith", party.DisplayName)
	assert.Equal(t, later, party.UpdatedAt)

	require.ErrorIs(t, party.Rename("   ", later.Add(time.Hour)), ErrInvalidParty)
	assert.Equal(t, "Jane Smith", party.DisplayName, "un rename inválido no debe mutar la entidad")
}

func TestPartySoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	party, err := NewParty(testTenantID, PartyTypeOrganization, "Acme", now)
	require.NoError(t, err)

	require.NoError(t, party.SoftDelete(later))
	require.NotNil(t, party.DeletedAt)
	assert.Equal(t, later, *party.DeletedAt)
	assert.Equal(t, later, party.UpdatedAt)

	require.ErrorIs(t, party.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}

func TestPartyMergeInto(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)
	targetID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002")

	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", now)
	require.NoError(t, err)

	require.NoError(t, party.MergeInto(targetID, later))
	assert.Equal(t, PartyRecordStatusMerged, party.RecordStatus)
	require.NotNil(t, party.CanonicalPartyID)
	assert.Equal(t, targetID, *party.CanonicalPartyID)
	require.NotNil(t, party.DeletedAt)
	assert.Equal(t, later, *party.DeletedAt)
	assert.Equal(t, later, party.UpdatedAt)
	require.NoError(t, party.Validate())
}

func TestPartyMergeIntoRejectsInvalidTargets(t *testing.T) {
	now := testNow()

	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", now)
	require.NoError(t, err)

	require.ErrorIs(t, party.MergeInto(uuid.Nil, now), ErrInvalidParty)
	require.ErrorIs(t, party.MergeInto(party.ID, now), ErrInvalidParty)

	require.NoError(t, party.SoftDelete(now))
	require.ErrorIs(t, party.MergeInto(uuid.New(), now), ErrAlreadyDeleted)
}

func TestPartyCompressTo(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)
	sourceID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001")
	targetID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000002")

	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", now)
	require.NoError(t, err)
	require.NoError(t, party.MergeInto(sourceID, now))

	require.NoError(t, party.CompressTo(targetID, later))
	require.NotNil(t, party.CanonicalPartyID)
	assert.Equal(t, targetID, *party.CanonicalPartyID)
	assert.Equal(t, later, party.UpdatedAt)
	assert.Equal(t, PartyRecordStatusMerged, party.RecordStatus)
	require.NoError(t, party.Validate())
}

func TestPartyValidateMergedInvariants(t *testing.T) {
	now := testNow()
	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", now)
	require.NoError(t, err)

	party.RecordStatus = PartyRecordStatusMerged
	require.ErrorIs(t, party.Validate(), ErrInvalidParty, "MERGED sin canonical/deleted_at debe fallar")

	canonical := uuid.New()
	party.CanonicalPartyID = &canonical
	require.ErrorIs(t, party.Validate(), ErrInvalidParty, "MERGED sin deleted_at debe fallar")

	party.DeletedAt = &now
	require.NoError(t, party.Validate())
}

func TestPartyCompressToRejectsActive(t *testing.T) {
	party, err := NewParty(testTenantID, PartyTypeIndividual, "Jane Doe", testNow())
	require.NoError(t, err)

	err = party.CompressTo(uuid.New(), testNow())
	require.ErrorIs(t, err, ErrInvalidParty)
}
