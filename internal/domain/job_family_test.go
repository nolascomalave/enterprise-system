package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJobFamily(t *testing.T) {
	jf, err := NewJobFamily(testTenantID, " Recursos Humanos ", " RH ", " Personas ", testNow())

	require.NoError(t, err)
	require.NotNil(t, jf)
	assert.NotEqual(t, uuid.Nil, jf.ID)
	assert.Equal(t, uuid.Version(7), jf.ID.Version())
	assert.Equal(t, testTenantID, jf.TenantID)
	assert.Equal(t, "Recursos Humanos", jf.Name)
	assert.Equal(t, "RH", jf.Code)
	assert.Equal(t, "Personas", jf.Description)
	assert.Nil(t, jf.DeletedAt)
}

func TestNewJobFamilyValidation(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{name: "", code: "RH"},
		{name: "   ", code: "RH"},
		{name: "Recursos Humanos", code: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jf, err := NewJobFamily(testTenantID, tt.name, tt.code, "", testNow())
			require.ErrorIs(t, err, ErrInvalidJobFamily)
			assert.Nil(t, jf)
		})
	}
}

func TestJobFamilyUpdateAndSoftDelete(t *testing.T) {
	jf, err := NewJobFamily(testTenantID, "RH", "RH", "", testNow())
	require.NoError(t, err)
	later := testNow().Add(1)

	require.NoError(t, jf.Update("Personas", "PPL", "desc", later))
	assert.Equal(t, "Personas", jf.Name)
	assert.Equal(t, later, jf.UpdatedAt)

	require.NoError(t, jf.SoftDelete(later))
	require.NotNil(t, jf.DeletedAt)
	require.ErrorIs(t, jf.Update("X", "X", "", later), ErrAlreadyDeleted)
}
