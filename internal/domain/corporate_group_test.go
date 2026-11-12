package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCorporateGroup(t *testing.T) {
	group, err := NewCorporateGroup(testTenantID, "  Holding  ", "  Grupo principal  ", testNow())

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, group.ID)
	assert.Equal(t, testTenantID, group.TenantID)
	assert.Equal(t, "Holding", group.Name)
	require.NotNil(t, group.Description)
	assert.Equal(t, "Grupo principal", *group.Description)
}

func TestNewCorporateGroupValidation(t *testing.T) {
	_, err := NewCorporateGroup(uuid.Nil, "Holding", "", testNow())
	require.ErrorIs(t, err, ErrInvalidCorporateGroup)

	_, err = NewCorporateGroup(testTenantID, "  ", "", testNow())
	require.ErrorIs(t, err, ErrInvalidCorporateGroup)
}

func TestCorporateGroupSoftDelete(t *testing.T) {
	group, err := NewCorporateGroup(testTenantID, "Holding", "", testNow())
	require.NoError(t, err)

	require.NoError(t, group.SoftDelete(testNow().Add(1)))
	assert.NotNil(t, group.DeletedAt)
	require.ErrorIs(t, group.SoftDelete(testNow().Add(2)), ErrAlreadyDeleted)
}
