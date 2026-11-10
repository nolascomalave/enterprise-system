package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmployeeAssignment(t *testing.T) {
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(1, 0, 0)

	assignment, err := NewEmployeeAssignment(testTenantID, testPartyID, uuid.New(), uuid.New(),
		start, &end, NewMoney(150000), testNow())

	require.NoError(t, err)
	require.NotNil(t, assignment)
	assert.NotEqual(t, uuid.Nil, assignment.ID)
	assert.Equal(t, testTenantID, assignment.TenantID)
	assert.Equal(t, testPartyID, assignment.EmployeePartyID)
	assert.Equal(t, start, assignment.StartDate)
	assert.Equal(t, end, *assignment.EndDate)
	assert.Equal(t, int64(150000), assignment.Salary.Cents)
}

func TestEmployeeAssignmentValidation(t *testing.T) {
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	before := start.Add(-time.Hour)

	tests := []struct {
		name   string
		end    *time.Time
		salary Money
	}{
		{name: "end before start", end: &before, salary: NewMoney(100)},
		{name: "negative salary", end: nil, salary: NewMoney(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignment, err := NewEmployeeAssignment(testTenantID, testPartyID, uuid.New(), uuid.New(),
				start, tt.end, tt.salary, testNow())

			require.ErrorIs(t, err, ErrInvalidEmployeeAssignment)
			assert.Nil(t, assignment)
		})
	}
}

func TestEmployeeAssignmentOverlaps(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	open := &EmployeeAssignment{EmployeePartyID: testPartyID, StartDate: start}
	closed := &EmployeeAssignment{
		EmployeePartyID: testPartyID,
		StartDate:       start.AddDate(0, 6, 0),
		EndDate:         ptrTime(start.AddDate(0, 12, 0)),
	}
	later := &EmployeeAssignment{
		EmployeePartyID: testPartyID,
		StartDate:       start.AddDate(0, 13, 0),
	}

	assert.True(t, open.Overlaps(closed))
	assert.True(t, closed.Overlaps(open))
	assert.False(t, closed.Overlaps(later))
}

func TestEmployeeAssignmentEnd(t *testing.T) {
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	assignment, err := NewEmployeeAssignment(testTenantID, testPartyID, uuid.New(), uuid.New(),
		start, nil, NewMoney(100), testNow())
	require.NoError(t, err)

	end := start.AddDate(1, 0, 0)
	require.NoError(t, assignment.End(end, testNow().Add(1)))
	require.NotNil(t, assignment.EndDate)
	assert.Equal(t, end, *assignment.EndDate)
	assert.False(t, assignment.IsOpen())

	require.Error(t, assignment.End(end.Add(time.Hour), testNow()))
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
