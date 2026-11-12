package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocationTypeValidation(t *testing.T) {
	valid := []LocationType{
		LocationTypeRegion, LocationTypeSubregion, LocationTypeState,
		LocationTypeProvince, LocationTypeCity, LocationTypeMunicipality, LocationTypeCounty,
	}
	for _, locationType := range valid {
		assert.Truef(t, locationType.Valid(), "type %q", locationType)
	}
	assert.False(t, LocationType("COUNTRY").Valid())
}

func TestNewLocation(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	location, err := NewLocation(
		LocationTypeCity, nil, "VE", "VE-A", "ext-1", "Caracas", "world.ve.caracas", now,
	)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, location.ID)
	assert.Equal(t, LocationTypeCity, location.LocationType)
	assert.Equal(t, "VE", location.CountryCode)
	assert.Equal(t, "world.ve.caracas", location.Path)
}

func TestNewLocationValidation(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

	tests := []Location{
		{LocationType: "COUNTRY", CountryCode: "VE", DefaultName: "Caracas", Path: "world.ve.caracas"},
		{LocationType: LocationTypeCity, CountryCode: "VENEZUELA", DefaultName: "Caracas", Path: "world.ve.caracas"},
		{LocationType: LocationTypeCity, CountryCode: "VE", DefaultName: " ", Path: "world.ve.caracas"},
		{LocationType: LocationTypeCity, CountryCode: "VE", DefaultName: "Caracas", Path: ""},
	}
	for i := range tests {
		tests[i].ID = uuid.New()
		tests[i].CreatedAt = now
		tests[i].UpdatedAt = now
		require.ErrorIsf(t, tests[i].Validate(), ErrInvalidLocation, "case %d", i)
	}
}
