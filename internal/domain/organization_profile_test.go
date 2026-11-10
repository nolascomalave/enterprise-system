package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrganizationProfile(t *testing.T) {
	now := testNow()

	profile, err := NewOrganizationProfile(
		testTenantID,
		testPartyID,
		"  Acme LLC ",
		" Acme ",
		" J-12345678-9 ",
		now,
	)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.NotEqual(t, uuid.Nil, profile.ID)
	assert.Equal(t, uuid.Version(7), profile.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, testTenantID, profile.TenantID)
	assert.Equal(t, testPartyID, profile.PartyID)
	assert.Equal(t, "Acme LLC", profile.LegalName)
	assert.Equal(t, "Acme", profile.TradeName)
	assert.Equal(t, "J-12345678-9", profile.TaxID)
	assert.Equal(t, now, profile.CreatedAt)
	assert.Equal(t, now, profile.UpdatedAt)
	assert.Nil(t, profile.DeletedAt)
}

func TestNewOrganizationProfileValidation(t *testing.T) {
	now := testNow()

	tests := []struct {
		name      string
		tenantID  uuid.UUID
		partyID   uuid.UUID
		legalName string
		taxID     string
	}{
		{name: "nil tenant", tenantID: uuid.Nil, partyID: testPartyID, legalName: "Acme LLC", taxID: "J-1"},
		{name: "nil party", tenantID: testTenantID, partyID: uuid.Nil, legalName: "Acme LLC", taxID: "J-1"},
		{name: "empty legal name", tenantID: testTenantID, partyID: testPartyID, legalName: "", taxID: "J-1"},
		{name: "empty tax id", tenantID: testTenantID, partyID: testPartyID, legalName: "Acme LLC", taxID: ""},
		{name: "blank tax id", tenantID: testTenantID, partyID: testPartyID, legalName: "Acme LLC", taxID: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := NewOrganizationProfile(
				tt.tenantID,
				tt.partyID,
				tt.legalName,
				"",
				tt.taxID,
				now,
			)

			require.ErrorIs(t, err, ErrInvalidOrganizationProfile)
			assert.Nil(t, profile)
		})
	}
}

func TestOrganizationProfileSoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	profile, err := NewOrganizationProfile(testTenantID, testPartyID, "Acme LLC", "", "J-1", now)
	require.NoError(t, err)

	require.NoError(t, profile.SoftDelete(later))
	require.NotNil(t, profile.DeletedAt)
	assert.Equal(t, later, *profile.DeletedAt)
	assert.Equal(t, later, profile.UpdatedAt)

	require.ErrorIs(t, profile.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}
