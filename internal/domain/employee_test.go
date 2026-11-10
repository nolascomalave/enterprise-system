package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmployee(t *testing.T) {
	jobID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000030")
	deptID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000031")
	managerID := uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000032")

	emp, err := NewEmployee(testPartyID, testTenantID, " EMP-001 ", testNow(),
		EmployeeStatusActive, jobID, deptID, &managerID, testNow())

	require.NoError(t, err)
	require.NotNil(t, emp)
	assert.Equal(t, testPartyID, emp.PartyID)
	assert.Equal(t, "EMP-001", emp.EmployeeCode)
	assert.Equal(t, jobID, emp.JobID)
	assert.Equal(t, deptID, emp.DepartmentID)
	assert.Equal(t, managerID, *emp.ManagerPartyID)
	assert.Equal(t, EmployeeStatusActive, emp.Status)
}

func TestEmployeeCannotBeOwnManager(t *testing.T) {
	jobID := uuid.New()
	deptID := uuid.New()

	emp, err := NewEmployee(testPartyID, testTenantID, "EMP-001", testNow(),
		EmployeeStatusActive, jobID, deptID, &testPartyID, testNow())

	require.ErrorIs(t, err, ErrEmployeeSelfManager)
	assert.Nil(t, emp)

	emp, err = NewEmployee(testPartyID, testTenantID, "EMP-001", testNow(),
		EmployeeStatusActive, jobID, deptID, nil, testNow())
	require.NoError(t, err)

	err = emp.AssignManager(&testPartyID, testNow())
	require.ErrorIs(t, err, ErrEmployeeSelfManager)
	assert.Nil(t, emp.ManagerPartyID)
}

func TestEmployeeStatusValidation(t *testing.T) {
	emp, err := NewEmployee(testPartyID, testTenantID, "EMP-001", testNow(),
		EmployeeStatus("OTHER"), uuid.New(), uuid.New(), nil, testNow())

	require.ErrorIs(t, err, ErrInvalidEmployee)
	assert.Nil(t, emp)
}

func TestEmployeeUpdateAndSoftDelete(t *testing.T) {
	emp, err := NewEmployee(testPartyID, testTenantID, "EMP-001", testNow(),
		EmployeeStatusActive, uuid.New(), uuid.New(), nil, testNow())
	require.NoError(t, err)
	later := testNow().Add(1)
	manager := uuid.New()

	require.NoError(t, emp.Update("EMP-002", testNow(), EmployeeStatusOnLeave,
		uuid.New(), uuid.New(), &manager, later))
	assert.Equal(t, "EMP-002", emp.EmployeeCode)
	assert.Equal(t, EmployeeStatusOnLeave, emp.Status)

	require.NoError(t, emp.SoftDelete(later))
	require.ErrorIs(t, emp.Update("X", testNow(), EmployeeStatusActive,
		uuid.New(), uuid.New(), nil, later), ErrAlreadyDeleted)
}
