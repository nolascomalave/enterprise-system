package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPartyRole(t *testing.T) {
	now := testNow()

	role, err := NewPartyRole(testTenantID, testPartyID, testRoleTypeID, now)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.NotEqual(t, uuid.Nil, role.ID)
	assert.Equal(t, uuid.Version(7), role.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, testTenantID, role.TenantID)
	assert.Equal(t, testPartyID, role.PartyID)
	assert.Equal(t, testRoleTypeID, role.RoleTypeID)
	assert.Equal(t, now, role.CreatedAt)
	assert.Equal(t, now, role.UpdatedAt)
	assert.Nil(t, role.DeletedAt)
}

func TestNewPartyRoleValidation(t *testing.T) {
	now := testNow()

	tests := []struct {
		name       string
		tenantID   uuid.UUID
		partyID    uuid.UUID
		roleTypeID uuid.UUID
	}{
		{name: "nil tenant", tenantID: uuid.Nil, partyID: testPartyID, roleTypeID: testRoleTypeID},
		{name: "nil party", tenantID: testTenantID, partyID: uuid.Nil, roleTypeID: testRoleTypeID},
		{name: "nil role type", tenantID: testTenantID, partyID: testPartyID, roleTypeID: uuid.Nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := NewPartyRole(tt.tenantID, tt.partyID, tt.roleTypeID, now)

			require.ErrorIs(t, err, ErrInvalidPartyRole)
			assert.Nil(t, role)
		})
	}
}

func TestPartyRoleSoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	role, err := NewPartyRole(testTenantID, testPartyID, testRoleTypeID, now)
	require.NoError(t, err)

	require.NoError(t, role.SoftDelete(later))
	require.NotNil(t, role.DeletedAt)
	assert.Equal(t, later, *role.DeletedAt)
	assert.Equal(t, later, role.UpdatedAt)

	require.ErrorIs(t, role.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}
