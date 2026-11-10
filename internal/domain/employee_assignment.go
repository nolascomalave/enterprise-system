package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EmployeeAssignment struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	EmployeePartyID uuid.UUID
	JobID           uuid.UUID
	DepartmentID    uuid.UUID
	StartDate       time.Time
	EndDate         *time.Time
	Salary          Money
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewEmployeeAssignment(
	tenantID, employeePartyID, jobID, departmentID uuid.UUID,
	startDate time.Time,
	endDate *time.Time,
	salary Money,
	now time.Time,
) (*EmployeeAssignment, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate employee assignment id: %w", err)
	}

	now = now.UTC()
	assignment := &EmployeeAssignment{
		ID:              id,
		TenantID:        tenantID,
		EmployeePartyID: employeePartyID,
		JobID:           jobID,
		DepartmentID:    departmentID,
		StartDate:       startDate.UTC(),
		EndDate:         endDate,
		Salary:          salary,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if assignment.EndDate != nil {
		end := assignment.EndDate.UTC()
		assignment.EndDate = &end
	}
	if err := assignment.Validate(); err != nil {
		return nil, err
	}
	return assignment, nil
}

func (a *EmployeeAssignment) Validate() error {
	if a == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidEmployeeAssignment)
	}
	if a.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidEmployeeAssignment)
	}
	if a.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidEmployeeAssignment)
	}
	if a.EmployeePartyID == uuid.Nil {
		return fmt.Errorf("%w: employee_party_id is required", ErrInvalidEmployeeAssignment)
	}
	if a.JobID == uuid.Nil {
		return fmt.Errorf("%w: job_id is required", ErrInvalidEmployeeAssignment)
	}
	if a.DepartmentID == uuid.Nil {
		return fmt.Errorf("%w: department_id is required", ErrInvalidEmployeeAssignment)
	}
	if a.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidEmployeeAssignment)
	}
	if a.EndDate != nil && !a.EndDate.After(a.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidEmployeeAssignment)
	}
	if a.Salary.IsNegative() {
		return fmt.Errorf("%w: salary cannot be negative", ErrInvalidEmployeeAssignment)
	}
	if a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidEmployeeAssignment)
	}
	if a.UpdatedAt.Before(a.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidEmployeeAssignment)
	}
	return nil
}

func (a *EmployeeAssignment) IsOpen() bool {
	return a.EndDate == nil
}

// Overlaps reports whether two assignments for the same employee overlap in
// time. The database enforces this with an EXCLUDE USING GIST constraint; this
// helper lets the domain reject obvious conflicts before hitting the database.
func (a *EmployeeAssignment) Overlaps(other *EmployeeAssignment) bool {
	if a == nil || other == nil || a.EmployeePartyID != other.EmployeePartyID {
		return false
	}

	switch {
	case a.EndDate == nil && other.EndDate == nil:
		return true
	case a.EndDate == nil:
		return other.EndDate.After(a.StartDate)
	case other.EndDate == nil:
		return a.EndDate.After(other.StartDate)
	default:
		return a.StartDate.Before(*other.EndDate) && other.StartDate.Before(*a.EndDate)
	}
}

func (a *EmployeeAssignment) End(at, now time.Time) error {
	if a.EndDate != nil {
		return fmt.Errorf("%w: assignment is already closed", ErrInvalidEmployeeAssignment)
	}
	at = at.UTC()
	if !at.After(a.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidEmployeeAssignment)
	}
	a.EndDate = &at
	a.UpdatedAt = now.UTC()
	return a.Validate()
}

func (a *EmployeeAssignment) Update(
	jobID, departmentID uuid.UUID,
	startDate time.Time,
	endDate *time.Time,
	salary Money,
	now time.Time,
) error {
	a.JobID = jobID
	a.DepartmentID = departmentID
	a.StartDate = startDate.UTC()
	a.EndDate = endDate
	if a.EndDate != nil {
		end := a.EndDate.UTC()
		a.EndDate = &end
	}
	a.Salary = salary
	a.UpdatedAt = now.UTC()
	return a.Validate()
}
