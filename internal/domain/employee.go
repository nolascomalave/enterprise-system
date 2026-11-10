package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EmployeeStatus string

const (
	EmployeeStatusActive     EmployeeStatus = "ACTIVE"
	EmployeeStatusOnLeave    EmployeeStatus = "ON_LEAVE"
	EmployeeStatusTerminated EmployeeStatus = "TERMINATED"
)

func (s EmployeeStatus) Valid() bool {
	switch s {
	case EmployeeStatusActive, EmployeeStatusOnLeave, EmployeeStatusTerminated:
		return true
	default:
		return false
	}
}

// Employee is the employment role over a Party. Its PK is PartyID.
type Employee struct {
	PartyID        uuid.UUID
	TenantID       uuid.UUID
	EmployeeCode   string
	HireDate       time.Time
	Status         EmployeeStatus
	JobID          uuid.UUID
	DepartmentID   uuid.UUID
	ManagerPartyID *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NewEmployee(
	partyID, tenantID uuid.UUID,
	employeeCode string,
	hireDate time.Time,
	status EmployeeStatus,
	jobID, departmentID uuid.UUID,
	managerPartyID *uuid.UUID,
	now time.Time,
) (*Employee, error) {
	now = now.UTC()
	if status == "" {
		status = EmployeeStatusActive
	}
	employee := &Employee{
		PartyID:        partyID,
		TenantID:       tenantID,
		EmployeeCode:   strings.TrimSpace(employeeCode),
		HireDate:       hireDate.UTC(),
		Status:         status,
		JobID:          jobID,
		DepartmentID:   departmentID,
		ManagerPartyID: managerPartyID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := employee.Validate(); err != nil {
		return nil, err
	}
	return employee, nil
}

func (e *Employee) Validate() error {
	if e == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidEmployee)
	}
	if e.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidEmployee)
	}
	if e.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidEmployee)
	}
	if strings.TrimSpace(e.EmployeeCode) == "" {
		return fmt.Errorf("%w: employee_code is required", ErrInvalidEmployee)
	}
	if e.HireDate.IsZero() {
		return fmt.Errorf("%w: hire_date is required", ErrInvalidEmployee)
	}
	if !e.Status.Valid() {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidEmployee, e.Status)
	}
	if e.JobID == uuid.Nil {
		return fmt.Errorf("%w: job_id is required", ErrInvalidEmployee)
	}
	if e.DepartmentID == uuid.Nil {
		return fmt.Errorf("%w: department_id is required", ErrInvalidEmployee)
	}
	if e.ManagerPartyID != nil {
		if *e.ManagerPartyID == uuid.Nil {
			return fmt.Errorf("%w: manager_party_id cannot be nil uuid", ErrInvalidEmployee)
		}
		if *e.ManagerPartyID == e.PartyID {
			return fmt.Errorf("%w: %w", ErrInvalidEmployee, ErrEmployeeSelfManager)
		}
	}
	if e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidEmployee)
	}
	if e.UpdatedAt.Before(e.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidEmployee)
	}
	return nil
}

func (e *Employee) AssignManager(managerPartyID *uuid.UUID, now time.Time) error {
	if e.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	if managerPartyID != nil && *managerPartyID == e.PartyID {
		return fmt.Errorf("%w: %w", ErrInvalidEmployee, ErrEmployeeSelfManager)
	}
	e.ManagerPartyID = managerPartyID
	e.UpdatedAt = now.UTC()
	return e.Validate()
}

func (e *Employee) Update(
	employeeCode string,
	hireDate time.Time,
	status EmployeeStatus,
	jobID, departmentID uuid.UUID,
	managerPartyID *uuid.UUID,
	now time.Time,
) error {
	if e.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	e.EmployeeCode = strings.TrimSpace(employeeCode)
	e.HireDate = hireDate.UTC()
	e.Status = status
	e.JobID = jobID
	e.DepartmentID = departmentID
	e.ManagerPartyID = managerPartyID
	e.UpdatedAt = now.UTC()
	return e.Validate()
}

func (e *Employee) SoftDelete(now time.Time) error {
	if e.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	e.DeletedAt = &now
	e.UpdatedAt = now
	return nil
}
