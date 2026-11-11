package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContactMechanism(t *testing.T) {
	contact, err := NewContactMechanism(ContactTypeEmail, " user@example.com ")

	require.NoError(t, err)
	require.NotNil(t, contact)
	assert.NotEqual(t, uuid.Nil, contact.ID)
	assert.Equal(t, ContactTypeEmail, contact.Type)
	assert.Equal(t, "user@example.com", contact.Value)
}

func TestContactMechanismEmailValidation(t *testing.T) {
	contact, err := NewContactMechanism(ContactTypeEmail, "not-an-email")

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
			contact, err := NewContactMechanism(tt.ctype, tt.value)
			require.ErrorIs(t, err, ErrInvalidContactMechanism)
			assert.Nil(t, contact)
		})
	}
}

func TestContactMechanismPhoneAndWebsite(t *testing.T) {
	phone, err := NewContactMechanism(ContactTypePhone, "+58 412 1234567")
	require.NoError(t, err)
	assert.Equal(t, ContactTypePhone, phone.Type)

	site, err := NewContactMechanism(ContactTypeWebsite, "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, ContactTypeWebsite, site.Type)
}

func TestContactMechanismUpdate(t *testing.T) {
	contact, err := NewContactMechanism(ContactTypeEmail, "user@example.com")
	require.NoError(t, err)

	require.NoError(t, contact.Update(ContactTypeEmail, "other@example.com"))
	assert.Equal(t, "other@example.com", contact.Value)

	require.ErrorIs(t, contact.Update(ContactTypeEmail, "not-an-email"), ErrInvalidContactMechanism)
}
