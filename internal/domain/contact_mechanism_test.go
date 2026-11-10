package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContactMechanism(t *testing.T) {
	contact, err := NewContactMechanism(testTenantID, testPartyID, ContactTypeEmail,
		" user@example.com ", true, false, testNow())

	require.NoError(t, err)
	require.NotNil(t, contact)
	assert.NotEqual(t, uuid.Nil, contact.ID)
	assert.Equal(t, ContactTypeEmail, contact.Type)
	assert.Equal(t, "user@example.com", contact.Value)
	assert.True(t, contact.IsPrimary)
	assert.False(t, contact.IsVerified)
}

func TestContactMechanismEmailValidation(t *testing.T) {
	contact, err := NewContactMechanism(testTenantID, testPartyID, ContactTypeEmail,
		"not-an-email", false, false, testNow())

	require.ErrorIs(t, err, ErrInvalidContactMechanism)
	assert.Nil(t, contact)
}

func TestContactMechanismValueValidation(t *testing.T) {
	tests := []struct {
		name  string
		ctype ContactType
		value string
	}{
		{name: "invalid phone", ctype: ContactTypePhone, value: "abc"},
		{name: "short phone", ctype: ContactTypeMobile, value: "123"},
		{name: "invalid website", ctype: ContactTypeWebsite, value: "not a url"},
		{name: "ftp website", ctype: ContactTypeWebsite, value: "ftp://example.com"},
		{name: "unsupported type", ctype: ContactType("OTHER"), value: "x"},
		{name: "empty value", ctype: ContactTypeEmail, value: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact, err := NewContactMechanism(testTenantID, testPartyID, tt.ctype, tt.value, false, false, testNow())
			require.ErrorIs(t, err, ErrInvalidContactMechanism)
			assert.Nil(t, contact)
		})
	}
}

func TestContactMechanismPhoneAndWebsite(t *testing.T) {
	phone, err := NewContactMechanism(testTenantID, testPartyID, ContactTypePhone, "+58 412 1234567", false, false, testNow())
	require.NoError(t, err)
	assert.Equal(t, ContactTypePhone, phone.Type)

	site, err := NewContactMechanism(testTenantID, testPartyID, ContactTypeWebsite, "https://example.com", false, false, testNow())
	require.NoError(t, err)
	assert.Equal(t, ContactTypeWebsite, site.Type)
}

func TestContactMechanismActions(t *testing.T) {
	contact, err := NewContactMechanism(testTenantID, testPartyID, ContactTypeEmail, "user@example.com", false, false, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)

	require.NoError(t, contact.SetPrimary(true, later))
	assert.True(t, contact.IsPrimary)

	require.NoError(t, contact.Verify(later))
	assert.True(t, contact.IsVerified)

	require.NoError(t, contact.Update(ContactTypeEmail, "other@example.com", false, true, later))
	assert.Equal(t, "other@example.com", contact.Value)

	require.NoError(t, contact.SoftDelete(later))
	require.NotNil(t, contact.DeletedAt)
	require.ErrorIs(t, contact.Update(ContactTypeEmail, "x@example.com", false, false, later), ErrAlreadyDeleted)
}
