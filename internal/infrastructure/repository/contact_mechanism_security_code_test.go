package repository

import (
	"testing"

	"enterprise-system/ent"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDomainContactMechanismMapsSecurityCodeFlag(t *testing.T) {
	row := &ent.ContactMechanism{
		ID:                     uuid.New(),
		Type:                   "EMAIL",
		Value:                  "user@example.com",
		CanReceiveSecurityCode: true,
	}

	contact := toDomainContactMechanism(row)

	require.NotNil(t, contact)
	assert.Equal(t, domain.ContactTypeEmail, contact.Type)
	assert.True(t, contact.CanReceiveSecurityCode)
}
