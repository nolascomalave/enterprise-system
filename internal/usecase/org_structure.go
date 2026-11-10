package usecase

import (
	"context"
	"fmt"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type OrgStructure struct {
	repo domain.OrgStructureRepository
	now  func() time.Time
}

func NewOrgStructure(repo domain.OrgStructureRepository, now func() time.Time) *OrgStructure {
	if now == nil {
		now = time.Now
	}
	return &OrgStructure{repo: repo, now: now}
}

type CreateJobFamilyInput struct {
	TenantID    uuid.UUID
	Name        string
	Code        string
	Description string
}

type UpdateJobFamilyInput struct {
	ID          uuid.UUID
	Name        string
	Code        string
	Description string
}

func (s *OrgStructure) CreateJobFamily(ctx context.Context, in CreateJobFamilyInput) (*domain.JobFamily, error) {
	entity, err := domain.NewJobFamily(in.TenantID, in.Name, in.Code, in.Description, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateJobFamily(ctx, entity); err != nil {
		return nil, fmt.Errorf("create job family: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) GetJobFamily(ctx context.Context, id uuid.UUID) (*domain.JobFamily, error) {
	return s.repo.GetJobFamily(ctx, id)
}

func (s *OrgStructure) ListJobFamilies(ctx context.Context) ([]*domain.JobFamily, error) {
	return s.repo.ListJobFamilies(ctx)
}

func (s *OrgStructure) UpdateJobFamily(ctx context.Context, in UpdateJobFamilyInput) (*domain.JobFamily, error) {
	entity, err := s.repo.GetJobFamily(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.Name, in.Code, in.Description, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateJobFamily(ctx, entity); err != nil {
		return nil, fmt.Errorf("update job family: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) DeleteJobFamily(ctx context.Context, id uuid.UUID) error {
	entity, err := s.repo.GetJobFamily(ctx, id)
	if err != nil {
		return err
	}
	if err := entity.SoftDelete(s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateJobFamily(ctx, entity); err != nil {
		return fmt.Errorf("delete job family: %w", err)
	}
	return nil
}

type CreateJobInput struct {
	TenantID    uuid.UUID
	JobFamilyID uuid.UUID
	Title       string
	Code        string
	Description string
	MinSalary   *domain.Money
	MaxSalary   *domain.Money
}

type UpdateJobInput struct {
	ID          uuid.UUID
	JobFamilyID uuid.UUID
	Title       string
	Code        string
	Description string
	MinSalary   *domain.Money
	MaxSalary   *domain.Money
}

func (s *OrgStructure) CreateJob(ctx context.Context, in CreateJobInput) (*domain.Job, error) {
	entity, err := domain.NewJob(in.TenantID, in.JobFamilyID, in.Title, in.Code, in.Description,
		in.MinSalary, in.MaxSalary, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateJob(ctx, entity); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	return s.repo.GetJob(ctx, id)
}

func (s *OrgStructure) ListJobs(ctx context.Context) ([]*domain.Job, error) {
	return s.repo.ListJobs(ctx)
}

func (s *OrgStructure) UpdateJob(ctx context.Context, in UpdateJobInput) (*domain.Job, error) {
	entity, err := s.repo.GetJob(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.JobFamilyID, in.Title, in.Code, in.Description,
		in.MinSalary, in.MaxSalary, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateJob(ctx, entity); err != nil {
		return nil, fmt.Errorf("update job: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) DeleteJob(ctx context.Context, id uuid.UUID) error {
	entity, err := s.repo.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if err := entity.SoftDelete(s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateJob(ctx, entity); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

type CreateDepartmentInput struct {
	TenantID           uuid.UUID
	ParentDepartmentID *uuid.UUID
	Name               string
	Code               string
	ManagerPartyID     *uuid.UUID
}

type UpdateDepartmentInput struct {
	ID                 uuid.UUID
	ParentDepartmentID *uuid.UUID
	Name               string
	Code               string
	ManagerPartyID     *uuid.UUID
}

func (s *OrgStructure) CreateDepartment(ctx context.Context, in CreateDepartmentInput) (*domain.Department, error) {
	entity, err := domain.NewDepartment(in.TenantID, in.ParentDepartmentID, in.Name, in.Code,
		in.ManagerPartyID, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateDepartment(ctx, entity); err != nil {
		return nil, fmt.Errorf("create department: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) GetDepartment(ctx context.Context, id uuid.UUID) (*domain.Department, error) {
	return s.repo.GetDepartment(ctx, id)
}

func (s *OrgStructure) ListDepartments(ctx context.Context) ([]*domain.Department, error) {
	return s.repo.ListDepartments(ctx)
}

func (s *OrgStructure) UpdateDepartment(ctx context.Context, in UpdateDepartmentInput) (*domain.Department, error) {
	entity, err := s.repo.GetDepartment(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.ParentDepartmentID, in.Name, in.Code, in.ManagerPartyID, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateDepartment(ctx, entity); err != nil {
		return nil, fmt.Errorf("update department: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) DeleteDepartment(ctx context.Context, id uuid.UUID) error {
	entity, err := s.repo.GetDepartment(ctx, id)
	if err != nil {
		return err
	}
	if err := entity.SoftDelete(s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateDepartment(ctx, entity); err != nil {
		return fmt.Errorf("delete department: %w", err)
	}
	return nil
}

type CreateEmployeeInput struct {
	PartyID        uuid.UUID
	TenantID       uuid.UUID
	EmployeeCode   string
	HireDate       time.Time
	Status         domain.EmployeeStatus
	JobID          uuid.UUID
	DepartmentID   uuid.UUID
	ManagerPartyID *uuid.UUID
}

type UpdateEmployeeInput struct {
	PartyID        uuid.UUID
	EmployeeCode   string
	HireDate       time.Time
	Status         domain.EmployeeStatus
	JobID          uuid.UUID
	DepartmentID   uuid.UUID
	ManagerPartyID *uuid.UUID
}

func (s *OrgStructure) CreateEmployee(ctx context.Context, in CreateEmployeeInput) (*domain.Employee, error) {
	entity, err := domain.NewEmployee(in.PartyID, in.TenantID, in.EmployeeCode, in.HireDate,
		in.Status, in.JobID, in.DepartmentID, in.ManagerPartyID, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateEmployee(ctx, entity); err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) GetEmployee(ctx context.Context, partyID uuid.UUID) (*domain.Employee, error) {
	return s.repo.GetEmployee(ctx, partyID)
}

func (s *OrgStructure) ListEmployees(ctx context.Context) ([]*domain.Employee, error) {
	return s.repo.ListEmployees(ctx)
}

func (s *OrgStructure) UpdateEmployee(ctx context.Context, in UpdateEmployeeInput) (*domain.Employee, error) {
	entity, err := s.repo.GetEmployee(ctx, in.PartyID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.EmployeeCode, in.HireDate, in.Status, in.JobID,
		in.DepartmentID, in.ManagerPartyID, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEmployee(ctx, entity); err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) DeleteEmployee(ctx context.Context, partyID uuid.UUID) error {
	entity, err := s.repo.GetEmployee(ctx, partyID)
	if err != nil {
		return err
	}
	if err := entity.SoftDelete(s.now()); err != nil {
		return err
	}
	if err := s.repo.UpdateEmployee(ctx, entity); err != nil {
		return fmt.Errorf("delete employee: %w", err)
	}
	return nil
}

type CreateEmployeeAssignmentInput struct {
	TenantID        uuid.UUID
	EmployeePartyID uuid.UUID
	JobID           uuid.UUID
	DepartmentID    uuid.UUID
	StartDate       time.Time
	EndDate         *time.Time
	Salary          domain.Money
}

type UpdateEmployeeAssignmentInput struct {
	ID           uuid.UUID
	JobID        uuid.UUID
	DepartmentID uuid.UUID
	StartDate    time.Time
	EndDate      *time.Time
	Salary       domain.Money
}

func (s *OrgStructure) CreateEmployeeAssignment(ctx context.Context, in CreateEmployeeAssignmentInput) (*domain.EmployeeAssignment, error) {
	entity, err := domain.NewEmployeeAssignment(in.TenantID, in.EmployeePartyID, in.JobID,
		in.DepartmentID, in.StartDate, in.EndDate, in.Salary, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateEmployeeAssignment(ctx, entity); err != nil {
		return nil, fmt.Errorf("create employee assignment: %w", err)
	}
	return entity, nil
}

func (s *OrgStructure) GetEmployeeAssignment(ctx context.Context, id uuid.UUID) (*domain.EmployeeAssignment, error) {
	return s.repo.GetEmployeeAssignment(ctx, id)
}

func (s *OrgStructure) ListEmployeeAssignments(ctx context.Context) ([]*domain.EmployeeAssignment, error) {
	return s.repo.ListEmployeeAssignments(ctx)
}

func (s *OrgStructure) UpdateEmployeeAssignment(ctx context.Context, in UpdateEmployeeAssignmentInput) (*domain.EmployeeAssignment, error) {
	entity, err := s.repo.GetEmployeeAssignment(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if err := entity.Update(in.JobID, in.DepartmentID, in.StartDate, in.EndDate, in.Salary, s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateEmployeeAssignment(ctx, entity); err != nil {
		return nil, fmt.Errorf("update employee assignment: %w", err)
	}
	return entity, nil
}

// DeleteEmployeeAssignment closes the assignment instead of deleting history.
func (s *OrgStructure) DeleteEmployeeAssignment(ctx context.Context, id uuid.UUID) error {
	entity, err := s.repo.GetEmployeeAssignment(ctx, id)
	if err != nil {
		return err
	}
	now := s.now()
	if err := entity.End(now, now); err != nil {
		return err
	}
	if err := s.repo.UpdateEmployeeAssignment(ctx, entity); err != nil {
		return fmt.Errorf("close employee assignment: %w", err)
	}
	return nil
}
