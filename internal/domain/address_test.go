package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAddress(t *testing.T) {
	cityID := uuid.New()
	address, err := NewAddress(testTenantID, testPartyID, AddressTypeLegal,
		" Av. Principal ", " Piso 2 ", cityID, " 1010 ", "ve", true, testNow())

	require.NoError(t, err)
	require.NotNil(t, address)
	assert.NotEqual(t, uuid.Nil, address.ID)
	assert.Equal(t, "Av. Principal", address.Line1)
	assert.Equal(t, "Piso 2", address.Line2)
	assert.Equal(t, cityID, address.CityLocationID)
	assert.Equal(t, "VE", address.Country)
	assert.True(t, address.IsPrimary)
}

func TestAddressValidation(t *testing.T) {
	cityID := uuid.New()
	tests := []struct {
		name    string
		atype   AddressType
		line1   string
		cityID  uuid.UUID
		country string
	}{
		{name: "empty line1", atype: AddressTypeLegal, line1: "", cityID: cityID, country: "VE"},
		{name: "missing city location", atype: AddressTypeLegal, line1: "Av. Principal", cityID: uuid.Nil, country: "VE"},
		{name: "country too long", atype: AddressTypeLegal, line1: "Av. Principal", cityID: cityID, country: "USA"},
		{name: "country too short", atype: AddressTypeLegal, line1: "Av. Principal", cityID: cityID, country: "V"},
		{name: "invalid type", atype: AddressType("OTHER"), line1: "Av. Principal", cityID: cityID, country: "VE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := NewAddress(testTenantID, testPartyID, tt.atype,
				tt.line1, "", tt.cityID, "", tt.country, false, testNow())
			require.ErrorIs(t, err, ErrInvalidAddress)
			assert.Nil(t, address)
		})
	}
}

func TestAddressActions(t *testing.T) {
	cityID := uuid.New()
	address, err := NewAddress(testTenantID, testPartyID, AddressTypeLegal,
		"Av. Principal", "", cityID, "", "VE", false, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)
	newCityID := uuid.New()

	require.NoError(t, address.SetPrimary(true, later))
	assert.True(t, address.IsPrimary)

	require.NoError(t, address.Update(AddressTypeFiscal,
		"Calle 2", "Oficina 1", newCityID, "2001", "ve", true, later))
	assert.Equal(t, AddressTypeFiscal, address.Type)
	assert.Equal(t, newCityID, address.CityLocationID)
	assert.Equal(t, "VE", address.Country)

	require.NoError(t, address.SoftDelete(later))
	require.NotNil(t, address.DeletedAt)
	require.ErrorIs(t, address.Update(AddressTypeLegal, "x", "", uuid.New(), "", "VE", false, later), ErrAlreadyDeleted)
}
