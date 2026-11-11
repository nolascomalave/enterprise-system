package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPartyContactMechanism(t *testing.T) {
	tenantID, partyID, contactID := uuid.New(), uuid.New(), uuid.New()
	now := testNow()

	relation, err := NewPartyContactMechanism(tenantID, partyID, contactID, true, false, 2, now, nil, now)

	require.NoError(t, err)
	require.NotNil(t, relation)
	assert.Equal(t, tenantID, relation.TenantID)
	assert.Equal(t, partyID, relation.PartyID)
	assert.Equal(t, contactID, relation.ContactMechanismID)
	assert.True(t, relation.IsPrimary)
	assert.Equal(t, 2, relation.ContactMechanismOrder)
	assert.True(t, relation.IsActive(now))
}

func TestPartyContactMechanismValidity(t *testing.T) {
	now := testNow()
	start := now.Add(-time.Hour)
	relation, err := NewPartyContactMechanism(uuid.New(), uuid.New(), uuid.New(), false, true, 1, start, nil, now)
	require.NoError(t, err)

	assert.True(t, relation.IsActive(now))
	require.NoError(t, relation.Close(now.Add(time.Hour)))
	assert.False(t, relation.IsActive(now.Add(2*time.Hour)))
}

func TestPartyContactMechanismRejectsInvalidDatesAndOrder(t *testing.T) {
	now := testNow()
	end := now.Add(-time.Hour)
	_, err := NewPartyContactMechanism(uuid.New(), uuid.New(), uuid.New(), false, false, -1, now, nil, now)
	require.ErrorIs(t, err, ErrInvalidPartyContactMechanism)

	_, err = NewPartyContactMechanism(uuid.New(), uuid.New(), uuid.New(), false, false, 1, now, &end, now)
	require.ErrorIs(t, err, ErrInvalidPartyContactMechanism)
}
