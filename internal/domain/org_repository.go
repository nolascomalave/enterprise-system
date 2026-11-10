package domain

import (
	"context"

	"github.com/google/uuid"
)

// OrgStructureRepository is the persistence port for the organizational
// structure and employment aggregates. Implementations run under the tenant RLS
// context.
type OrgStructureRepository interface {
	CreateJobFamily(ctx context.Context, jobFamily *JobFamily) error
	GetJobFamily(ctx context.Context, id uuid.UUID) (*JobFamily, error)
	ListJobFamilies(ctx context.Context) ([]*JobFamily, error)
	UpdateJobFamily(ctx context.Context, jobFamily *JobFamily) error

	CreateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id uuid.UUID) (*Job, error)
	ListJobs(ctx context.Context) ([]*Job, error)
	UpdateJob(ctx context.Context, job *Job) error

	CreateDepartment(ctx context.Context, department *Department) error
	GetDepartment(ctx context.Context, id uuid.UUID) (*Department, error)
	ListDepartments(ctx context.Context) ([]*Department, error)
	UpdateDepartment(ctx context.Context, department *Department) error

	CreateEmployee(ctx context.Context, employee *Employee) error
	GetEmployee(ctx context.Context, partyID uuid.UUID) (*Employee, error)
	ListEmployees(ctx context.Context) ([]*Employee, error)
	UpdateEmployee(ctx context.Context, employee *Employee) error

	CreateEmployeeAssignment(ctx context.Context, assignment *EmployeeAssignment) error
	GetEmployeeAssignment(ctx context.Context, id uuid.UUID) (*EmployeeAssignment, error)
	ListEmployeeAssignments(ctx context.Context) ([]*EmployeeAssignment, error)
	UpdateEmployeeAssignment(ctx context.Context, assignment *EmployeeAssignment) error
}
