package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAddress(t *testing.T) {
	address, err := NewAddress(testTenantID, testPartyID, AddressTypeLegal,
		" Av. Principal ", " Piso 2 ", " Caracas ", " Distrito Capital ", " 1010 ", "ve", true, testNow())

	require.NoError(t, err)
	require.NotNil(t, address)
	assert.NotEqual(t, uuid.Nil, address.ID)
	assert.Equal(t, "Av. Principal", address.Line1)
	assert.Equal(t, "Piso 2", address.Line2)
	assert.Equal(t, "Caracas", address.City)
	assert.Equal(t, "VE", address.Country)
	assert.True(t, address.IsPrimary)
}

func TestAddressValidation(t *testing.T) {
	tests := []struct {
		name    string
		atype   AddressType
		line1   string
		city    string
		country string
	}{
		{name: "empty line1", atype: AddressTypeLegal, line1: "", city: "Caracas", country: "VE"},
		{name: "empty city", atype: AddressTypeLegal, line1: "Av. Principal", city: "", country: "VE"},
		{name: "country too long", atype: AddressTypeLegal, line1: "Av. Principal", city: "Caracas", country: "USA"},
		{name: "country too short", atype: AddressTypeLegal, line1: "Av. Principal", city: "Caracas", country: "V"},
		{name: "invalid type", atype: AddressType("OTHER"), line1: "Av. Principal", city: "Caracas", country: "VE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := NewAddress(testTenantID, testPartyID, tt.atype,
				tt.line1, "", tt.city, "", "", tt.country, false, testNow())
			require.ErrorIs(t, err, ErrInvalidAddress)
			assert.Nil(t, address)
		})
	}
}

func TestAddressActions(t *testing.T) {
	address, err := NewAddress(testTenantID, testPartyID, AddressTypeLegal,
		"Av. Principal", "", "Caracas", "", "", "VE", false, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)

	require.NoError(t, address.SetPrimary(true, later))
	assert.True(t, address.IsPrimary)

	require.NoError(t, address.Update(AddressTypeFiscal,
		"Calle 2", "Oficina 1", "Valencia", "Carabobo", "2001", "ve", true, later))
	assert.Equal(t, AddressTypeFiscal, address.Type)
	assert.Equal(t, "VE", address.Country)

	require.NoError(t, address.SoftDelete(later))
	require.NotNil(t, address.DeletedAt)
	require.ErrorIs(t, address.Update(AddressTypeLegal, "x", "", "x", "", "", "VE", false, later), ErrAlreadyDeleted)
}
