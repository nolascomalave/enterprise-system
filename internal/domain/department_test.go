package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDepartment(t *testing.T) {
	parent := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000020")
	manager := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000021")

	dept, err := NewDepartment(testTenantID, &parent, " Personas ", "PPL", &manager, testNow())

	require.NoError(t, err)
	require.NotNil(t, dept)
	assert.NotEqual(t, uuid.Nil, dept.ID)
	assert.Equal(t, parent, *dept.ParentDepartmentID)
	assert.Equal(t, "Personas", dept.Name)
	assert.Equal(t, "PPL", dept.Code)
	assert.Equal(t, manager, *dept.ManagerPartyID)
}

func TestNewDepartmentValidation(t *testing.T) {
	manager := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000021")

	dept, err := NewDepartment(testTenantID, nil, "Personas", "PPL", &manager, testNow())
	require.NoError(t, err)

	dept.ParentDepartmentID = &dept.ID
	require.ErrorIs(t, dept.Validate(), ErrInvalidDepartment)

	dept.ParentDepartmentID = nil
	dept.Name = " "
	require.ErrorIs(t, dept.Validate(), ErrInvalidDepartment)
}

func TestDepartmentUpdateAndSoftDelete(t *testing.T) {
	dept, err := NewDepartment(testTenantID, nil, "Personas", "PPL", nil, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)
	manager := uuid.New()

	require.NoError(t, dept.Update(nil, "Talento", "TAL", &manager, later))
	assert.Equal(t, "Talento", dept.Name)
	assert.Equal(t, manager, *dept.ManagerPartyID)

	require.NoError(t, dept.SoftDelete(later))
	require.ErrorIs(t, dept.Update(nil, "X", "X", nil, later), ErrAlreadyDeleted)
}
