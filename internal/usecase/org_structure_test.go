package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOrgRepo struct {
	jobFamilies  map[uuid.UUID]*domain.JobFamily
	jobs         map[uuid.UUID]*domain.Job
	departments  map[uuid.UUID]*domain.Department
	employees    map[uuid.UUID]*domain.Employee
	assignments  map[uuid.UUID]*domain.EmployeeAssignment
	createJfErr  error
	updateJfErr  error
	getJfErr     error
	createAsgErr error
	updateAsgErr error
}

func newFakeOrgRepo() *fakeOrgRepo {
	return &fakeOrgRepo{
		jobFamilies: make(map[uuid.UUID]*domain.JobFamily),
		jobs:        make(map[uuid.UUID]*domain.Job),
		departments: make(map[uuid.UUID]*domain.Department),
		employees:   make(map[uuid.UUID]*domain.Employee),
		assignments: make(map[uuid.UUID]*domain.EmployeeAssignment),
	}
}

func (r *fakeOrgRepo) CreateJobFamily(ctx context.Context, e *domain.JobFamily) error {
	if r.createJfErr != nil {
		return r.createJfErr
	}
	r.jobFamilies[e.ID] = e
	return nil
}
func (r *fakeOrgRepo) GetJobFamily(ctx context.Context, id uuid.UUID) (*domain.JobFamily, error) {
	if r.getJfErr != nil {
		return nil, r.getJfErr
	}
	e, ok := r.jobFamilies[id]
	if !ok {
		return nil, errors.New("job family not found")
	}
	return e, nil
}
func (r *fakeOrgRepo) ListJobFamilies(ctx context.Context) ([]*domain.JobFamily, error) {
	out := make([]*domain.JobFamily, 0, len(r.jobFamilies))
	for _, e := range r.jobFamilies {
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeOrgRepo) UpdateJobFamily(ctx context.Context, e *domain.JobFamily) error {
	if r.updateJfErr != nil {
		return r.updateJfErr
	}
	r.jobFamilies[e.ID] = e
	return nil
}

func (r *fakeOrgRepo) CreateJob(ctx context.Context, e *domain.Job) error {
	r.jobs[e.ID] = e
	return nil
}
func (r *fakeOrgRepo) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	e, ok := r.jobs[id]
	if !ok {
		return nil, errors.New("job not found")
	}
	return e, nil
}
func (r *fakeOrgRepo) ListJobs(ctx context.Context) ([]*domain.Job, error) {
	out := make([]*domain.Job, 0, len(r.jobs))
	for _, e := range r.jobs {
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeOrgRepo) UpdateJob(ctx context.Context, e *domain.Job) error {
	r.jobs[e.ID] = e
	return nil
}

func (r *fakeOrgRepo) CreateDepartment(ctx context.Context, e *domain.Department) error {
	r.departments[e.ID] = e
	return nil
}
func (r *fakeOrgRepo) GetDepartment(ctx context.Context, id uuid.UUID) (*domain.Department, error) {
	e, ok := r.departments[id]
	if !ok {
		return nil, errors.New("department not found")
	}
	return e, nil
}
func (r *fakeOrgRepo) ListDepartments(ctx context.Context) ([]*domain.Department, error) {
	out := make([]*domain.Department, 0, len(r.departments))
	for _, e := range r.departments {
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeOrgRepo) UpdateDepartment(ctx context.Context, e *domain.Department) error {
	r.departments[e.ID] = e
	return nil
}

func (r *fakeOrgRepo) CreateEmployee(ctx context.Context, e *domain.Employee) error {
	r.employees[e.PartyID] = e
	return nil
}
func (r *fakeOrgRepo) GetEmployee(ctx context.Context, id uuid.UUID) (*domain.Employee, error) {
	e, ok := r.employees[id]
	if !ok {
		return nil, errors.New("employee not found")
	}
	return e, nil
}
func (r *fakeOrgRepo) ListEmployees(ctx context.Context) ([]*domain.Employee, error) {
	out := make([]*domain.Employee, 0, len(r.employees))
	for _, e := range r.employees {
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeOrgRepo) UpdateEmployee(ctx context.Context, e *domain.Employee) error {
	r.employees[e.PartyID] = e
	return nil
}

func (r *fakeOrgRepo) CreateEmployeeAssignment(ctx context.Context, e *domain.EmployeeAssignment) error {
	if r.createAsgErr != nil {
		return r.createAsgErr
	}
	r.assignments[e.ID] = e
	return nil
}
func (r *fakeOrgRepo) GetEmployeeAssignment(ctx context.Context, id uuid.UUID) (*domain.EmployeeAssignment, error) {
	e, ok := r.assignments[id]
	if !ok {
		return nil, errors.New("assignment not found")
	}
	return e, nil
}
func (r *fakeOrgRepo) ListEmployeeAssignments(ctx context.Context) ([]*domain.EmployeeAssignment, error) {
	out := make([]*domain.EmployeeAssignment, 0, len(r.assignments))
	for _, e := range r.assignments {
		out = append(out, e)
	}
	return out, nil
}
func (r *fakeOrgRepo) UpdateEmployeeAssignment(ctx context.Context, e *domain.EmployeeAssignment) error {
	if r.updateAsgErr != nil {
		return r.updateAsgErr
	}
	r.assignments[e.ID] = e
	return nil
}

func orgTestNow() time.Time {
	return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
}

func TestOrgStructureCreateJobFamily(t *testing.T) {
	repo := newFakeOrgRepo()
	txManager := newFakeTxManager()
	uc := NewOrgStructure(repo, txManager, orgTestNow)

	jf, err := uc.CreateJobFamily(context.Background(), CreateJobFamilyInput{
		TenantID: testTenantID(), Name: " Recursos Humanos ", Code: " RH ",
	})

	require.NoError(t, err)
	require.NotNil(t, jf)
	assert.Equal(t, "Recursos Humanos", jf.Name)
	assert.Equal(t, "RH", jf.Code)
	assert.Contains(t, repo.jobFamilies, jf.ID)
	assert.Equal(t, 1, txManager.calls, "the write use case must open a transaction")
}

func TestOrgStructureCreateJobFamilyValidation(t *testing.T) {
	uc := NewOrgStructure(newFakeOrgRepo(), newFakeTxManager(), orgTestNow)

	_, err := uc.CreateJobFamily(context.Background(), CreateJobFamilyInput{
		TenantID: testTenantID(), Name: " ", Code: "RH",
	})

	require.ErrorIs(t, err, domain.ErrInvalidJobFamily)
}

func TestOrgStructureCreateJobRejectsInvalidSalaryRange(t *testing.T) {
	uc := NewOrgStructure(newFakeOrgRepo(), newFakeTxManager(), orgTestNow)
	min := domain.NewMoney(2000)
	max := domain.NewMoney(1000)

	_, err := uc.CreateJob(context.Background(), CreateJobInput{
		TenantID: testTenantID(), JobFamilyID: uuid.New(), Title: "Gerente", Code: "GER",
		MinSalary: &min, MaxSalary: &max,
	})

	require.ErrorIs(t, err, domain.ErrInvalidJob)
}

func TestOrgStructureCreateEmployeeRejectsSelfManager(t *testing.T) {
	uc := NewOrgStructure(newFakeOrgRepo(), newFakeTxManager(), orgTestNow)
	partyID := uuid.New()

	_, err := uc.CreateEmployee(context.Background(), CreateEmployeeInput{
		PartyID: partyID, TenantID: testTenantID(), EmployeeCode: "EMP-001",
		HireDate: orgTestNow(), Status: domain.EmployeeStatusActive,
		JobID: uuid.New(), DepartmentID: uuid.New(), ManagerPartyID: &partyID,
	})

	require.ErrorIs(t, err, domain.ErrEmployeeSelfManager)
}

func TestOrgStructureUpdateAndDeleteJobFamily(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)

	jf, err := uc.CreateJobFamily(context.Background(), CreateJobFamilyInput{
		TenantID: testTenantID(), Name: "RH", Code: "RH",
	})
	require.NoError(t, err)

	updated, err := uc.UpdateJobFamily(context.Background(), UpdateJobFamilyInput{
		ID: jf.ID, Name: "Personas", Code: "PPL",
	})
	require.NoError(t, err)
	assert.Equal(t, "Personas", updated.Name)

	require.NoError(t, uc.DeleteJobFamily(context.Background(), jf.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestOrgStructureDeleteEmployeeAssignmentClosesIt(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	start := orgTestNow().AddDate(0, -1, 0)

	assignment, err := uc.CreateEmployeeAssignment(context.Background(), CreateEmployeeAssignmentInput{
		TenantID: testTenantID(), EmployeePartyID: uuid.New(), JobID: uuid.New(),
		DepartmentID: uuid.New(), StartDate: start, Salary: domain.NewMoney(1000),
	})
	require.NoError(t, err)

	require.NoError(t, uc.DeleteEmployeeAssignment(context.Background(), assignment.ID))
	require.NotNil(t, assignment.EndDate)
	assert.False(t, assignment.IsOpen())
}

func TestOrgStructurePropagatesRepositoryError(t *testing.T) {
	repo := newFakeOrgRepo()
	repo.createJfErr = errors.New("db down")
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)

	_, err := uc.CreateJobFamily(context.Background(), CreateJobFamilyInput{
		TenantID: testTenantID(), Name: "RH", Code: "RH",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create job family")
}

func testTenantID() uuid.UUID {
	return uuid.MustParse("018f3a4e-1c2d-7b3e-8f00-000000000001")
}

func TestOrgStructureJobCRUD(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	ctx := context.Background()

	job, err := uc.CreateJob(ctx, CreateJobInput{
		TenantID: testTenantID(), JobFamilyID: uuid.New(), Title: "Developer", Code: "DEV",
	})
	require.NoError(t, err)

	got, err := uc.GetJob(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "Developer", got.Title)

	list, err := uc.ListJobs(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateJob(ctx, UpdateJobInput{
		ID: job.ID, JobFamilyID: job.JobFamilyID, Title: "Senior Developer", Code: "DEV",
	})
	require.NoError(t, err)
	assert.Equal(t, "Senior Developer", updated.Title)

	require.NoError(t, uc.DeleteJob(ctx, job.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestOrgStructureDepartmentCRUD(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	ctx := context.Background()

	dept, err := uc.CreateDepartment(ctx, CreateDepartmentInput{
		TenantID: testTenantID(), Name: "Personas", Code: "PPL",
	})
	require.NoError(t, err)

	got, err := uc.GetDepartment(ctx, dept.ID)
	require.NoError(t, err)
	assert.Equal(t, "Personas", got.Name)

	list, err := uc.ListDepartments(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateDepartment(ctx, UpdateDepartmentInput{
		ID: dept.ID, Name: "Talento", Code: "TAL",
	})
	require.NoError(t, err)
	assert.Equal(t, "Talento", updated.Name)

	require.NoError(t, uc.DeleteDepartment(ctx, dept.ID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestOrgStructureEmployeeCRUD(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	ctx := context.Background()
	partyID := uuid.New()
	managerID := uuid.New()

	emp, err := uc.CreateEmployee(ctx, CreateEmployeeInput{
		PartyID: partyID, TenantID: testTenantID(), EmployeeCode: "EMP-001",
		HireDate: orgTestNow(), Status: domain.EmployeeStatusActive,
		JobID: uuid.New(), DepartmentID: uuid.New(),
	})
	require.NoError(t, err)
	assert.Equal(t, partyID, emp.PartyID)

	got, err := uc.GetEmployee(ctx, partyID)
	require.NoError(t, err)
	assert.Equal(t, "EMP-001", got.EmployeeCode)

	list, err := uc.ListEmployees(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateEmployee(ctx, UpdateEmployeeInput{
		PartyID: partyID, EmployeeCode: "EMP-001", HireDate: orgTestNow(),
		Status: domain.EmployeeStatusOnLeave, JobID: got.JobID,
		DepartmentID: got.DepartmentID, ManagerPartyID: &managerID,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.EmployeeStatusOnLeave, updated.Status)

	require.NoError(t, uc.DeleteEmployee(ctx, partyID))
	assert.NotNil(t, updated.DeletedAt)
}

func TestOrgStructureEmployeeAssignmentCRUD(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	ctx := context.Background()
	start := orgTestNow().AddDate(0, -1, 0)

	assignment, err := uc.CreateEmployeeAssignment(ctx, CreateEmployeeAssignmentInput{
		TenantID: testTenantID(), EmployeePartyID: uuid.New(), JobID: uuid.New(),
		DepartmentID: uuid.New(), StartDate: start, Salary: domain.NewMoney(1000),
	})
	require.NoError(t, err)

	got, err := uc.GetEmployeeAssignment(ctx, assignment.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), got.Salary.Cents)

	list, err := uc.ListEmployeeAssignments(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	updated, err := uc.UpdateEmployeeAssignment(ctx, UpdateEmployeeAssignmentInput{
		ID: assignment.ID, JobID: assignment.JobID, DepartmentID: assignment.DepartmentID,
		StartDate: start, Salary: domain.NewMoney(1200),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1200), updated.Salary.Cents)
}

func TestOrgStructureJobFamilyGetAndList(t *testing.T) {
	repo := newFakeOrgRepo()
	uc := NewOrgStructure(repo, newFakeTxManager(), orgTestNow)
	ctx := context.Background()

	jf, err := uc.CreateJobFamily(ctx, CreateJobFamilyInput{
		TenantID: testTenantID(), Name: "RH", Code: "RH",
	})
	require.NoError(t, err)

	got, err := uc.GetJobFamily(ctx, jf.ID)
	require.NoError(t, err)
	assert.Equal(t, "RH", got.Name)

	list, err := uc.ListJobFamilies(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}
