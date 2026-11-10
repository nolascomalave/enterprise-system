package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPersonProfile(t *testing.T) {
	now := testNow()
	birthDate := time.Date(1990, 5, 10, 0, 0, 0, 0, time.UTC)

	profile, err := NewPersonProfile(
		testTenantID,
		testPartyID,
		"  Jane ",
		" Ann ",
		" Doe ",
		" Smith ",
		&birthDate,
		now,
	)

	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.NotEqual(t, uuid.Nil, profile.ID)
	assert.Equal(t, uuid.Version(7), profile.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, testTenantID, profile.TenantID)
	assert.Equal(t, testPartyID, profile.PartyID)
	assert.Equal(t, "Jane", profile.FirstName)
	assert.Equal(t, "Ann", profile.MiddleName)
	assert.Equal(t, "Doe", profile.LastName)
	assert.Equal(t, "Smith", profile.SecondLastName)
	require.NotNil(t, profile.BirthDate)
	assert.Equal(t, birthDate, *profile.BirthDate)
	assert.Equal(t, now, profile.CreatedAt)
	assert.Equal(t, now, profile.UpdatedAt)
	assert.Nil(t, profile.DeletedAt)
}

func TestNewPersonProfileValidation(t *testing.T) {
	now := testNow()
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		tenantID  uuid.UUID
		partyID   uuid.UUID
		firstName string
		lastName  string
		birthDate *time.Time
	}{
		{name: "nil tenant", tenantID: uuid.Nil, partyID: testPartyID, firstName: "Jane", lastName: "Doe"},
		{name: "nil party", tenantID: testTenantID, partyID: uuid.Nil, firstName: "Jane", lastName: "Doe"},
		{name: "empty first name", tenantID: testTenantID, partyID: testPartyID, firstName: "", lastName: "Doe"},
		{name: "empty last name", tenantID: testTenantID, partyID: testPartyID, firstName: "Jane", lastName: ""},
		{name: "future birth date", tenantID: testTenantID, partyID: testPartyID, firstName: "Jane", lastName: "Doe", birthDate: &future},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := NewPersonProfile(
				tt.tenantID,
				tt.partyID,
				tt.firstName,
				"",
				tt.lastName,
				"",
				tt.birthDate,
				now,
			)

			require.ErrorIs(t, err, ErrInvalidPersonProfile)
			assert.Nil(t, profile)
		})
	}
}

func TestPersonProfileFullName(t *testing.T) {
	now := testNow()

	full, err := NewPersonProfile(testTenantID, testPartyID, "Jane", "Ann", "Doe", "Smith", nil, now)
	require.NoError(t, err)
	assert.Equal(t, "Jane Ann Doe Smith", full.FullName())

	short, err := NewPersonProfile(testTenantID, testPartyID, "Jane", "", "Doe", "", nil, now)
	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", short.FullName())
}

func TestPersonProfileSoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	profile, err := NewPersonProfile(testTenantID, testPartyID, "Jane", "", "Doe", "", nil, now)
	require.NoError(t, err)

	require.NoError(t, profile.SoftDelete(later))
	require.NotNil(t, profile.DeletedAt)
	assert.Equal(t, later, *profile.DeletedAt)
	assert.Equal(t, later, profile.UpdatedAt)

	require.ErrorIs(t, profile.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}
