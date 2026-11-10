package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRoleType(t *testing.T) {
	now := testNow()

	role, err := NewRoleType(" tenant-admin ", " Tenant Admin ", " Administra el tenant ", true, now)

	require.NoError(t, err)
	require.NotNil(t, role)
	assert.NotEqual(t, uuid.Nil, role.ID)
	assert.Equal(t, uuid.Version(7), role.ID.Version(), "el id debe ser UUIDv7")
	assert.Equal(t, "TENANT_ADMIN", role.Code)
	assert.Equal(t, "Tenant Admin", role.Name)
	assert.Equal(t, "Administra el tenant", role.Description)
	assert.True(t, role.IsSystem)
	assert.Equal(t, now, role.CreatedAt)
	assert.Equal(t, now, role.UpdatedAt)
	assert.Nil(t, role.DeletedAt)
}

func TestNewRoleTypeValidation(t *testing.T) {
	now := testNow()

	tests := []struct {
		name string
		code string
		role string
	}{
		{name: "empty code", code: "", role: "Tenant Admin"},
		{name: "code without letters", code: "123", role: "Tenant Admin"},
		{name: "code with only symbols", code: "!!!", role: "Tenant Admin"},
		{name: "empty name", code: "TENANT_ADMIN", role: ""},
		{name: "blank name", code: "TENANT_ADMIN", role: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, err := NewRoleType(tt.code, tt.role, "", false, now)

			require.ErrorIs(t, err, ErrInvalidRoleType)
			assert.Nil(t, role)
		})
	}
}

func TestRoleTypeCodeNormalization(t *testing.T) {
	now := testNow()

	tests := map[string]string{
		" tenant-admin ": "TENANT_ADMIN",
		"tenant_admin":   "TENANT_ADMIN",
		"Tenant Admin":   "TENANT_ADMIN",
		"owner":          "OWNER",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			role, err := NewRoleType(input, "Role", "", false, now)

			require.NoError(t, err)
			assert.Equal(t, want, role.Code)
		})
	}
}

func TestRoleTypeSoftDelete(t *testing.T) {
	now := testNow()
	later := now.Add(time.Hour)

	role, err := NewRoleType("OWNER", "Owner", "", false, now)
	require.NoError(t, err)

	require.NoError(t, role.SoftDelete(later))
	require.NotNil(t, role.DeletedAt)
	assert.Equal(t, later, *role.DeletedAt)
	assert.Equal(t, later, role.UpdatedAt)

	require.ErrorIs(t, role.SoftDelete(later.Add(time.Hour)), ErrAlreadyDeleted)
}
