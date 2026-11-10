package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJob(t *testing.T) {
	min := NewMoney(100000)
	max := NewMoney(200000)

	job, err := NewJob(testTenantID, uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000010"),
		" Gerente ", "GER", " Rol ", &min, &max, testNow())

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.NotEqual(t, uuid.Nil, job.ID)
	assert.Equal(t, "Gerente", job.Title)
	assert.Equal(t, "GER", job.Code)
	assert.Equal(t, min, *job.MinSalary)
	assert.Equal(t, max, *job.MaxSalary)
}

func TestNewJobSalaryValidation(t *testing.T) {
	min := NewMoney(200000)
	max := NewMoney(100000)
	negative := NewMoney(-1)

	tests := []struct {
		name string
		min  *Money
		max  *Money
	}{
		{name: "min greater than max", min: &min, max: &max},
		{name: "negative min", min: &negative, max: nil},
		{name: "negative max", min: nil, max: &negative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := NewJob(testTenantID, uuid.New(), "Gerente", "GER", "", tt.min, tt.max, testNow())
			require.ErrorIs(t, err, ErrInvalidJob)
			assert.Nil(t, job)
		})
	}
}

func TestJobUpdateAndSoftDelete(t *testing.T) {
	job, err := NewJob(testTenantID, uuid.New(), "Gerente", "GER", "", nil, nil, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)

	require.NoError(t, job.Update(uuid.New(), "Director", "DIR", "", nil, nil, later))
	assert.Equal(t, "Director", job.Title)

	require.NoError(t, job.SoftDelete(later))
	require.ErrorIs(t, job.Update(uuid.New(), "X", "X", "", nil, nil, later), ErrAlreadyDeleted)
}
