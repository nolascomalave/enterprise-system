package http

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOrgUseCase struct {
	job         *domain.Job
	jobs        []*domain.Job
	department  *domain.Department
	departments []*domain.Department
	employee    *domain.Employee
	employees   []*domain.Employee
	jobInput    usecase.CreateJobInput
	updateJob   usecase.UpdateJobInput
	deptInput   usecase.CreateDepartmentInput
	groupInput  usecase.CreateCorporateGroupInput
	assignInput usecase.AssignDepartmentToGroupInput
	group       *domain.CorporateGroup
	assignErr   error
	empInput    usecase.CreateEmployeeInput
	err         error
}

func (f *fakeOrgUseCase) CreateJob(ctx context.Context, in usecase.CreateJobInput) (*domain.Job, error) {
	f.jobInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}
func (f *fakeOrgUseCase) ListJobs(ctx context.Context) ([]*domain.Job, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.jobs, nil
}
func (f *fakeOrgUseCase) UpdateJob(ctx context.Context, in usecase.UpdateJobInput) (*domain.Job, error) {
	f.updateJob = in
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}
func (f *fakeOrgUseCase) CreateDepartment(ctx context.Context, in usecase.CreateDepartmentInput) (*domain.Department, error) {
	f.deptInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.department, nil
}
func (f *fakeOrgUseCase) ListDepartments(ctx context.Context) ([]*domain.Department, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.departments, nil
}
func (f *fakeOrgUseCase) CreateEmployee(ctx context.Context, in usecase.CreateEmployeeInput) (*domain.Employee, error) {
	f.empInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.employee, nil
}
func (f *fakeOrgUseCase) ListEmployees(ctx context.Context) ([]*domain.Employee, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.employees, nil
}

type fakeMergeUseCase struct {
	result *domain.MergeResult
	err    error
	source uuid.UUID
	target uuid.UUID
}

func (f *fakeMergeUseCase) Execute(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.MergeResult, error) {
	f.source, f.target = sourceID, targetID
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func orgTestRouter(t *testing.T, org *fakeOrgUseCase, merge *fakeMergeUseCase) (*gin.Engine, string, uuid.UUID) {
	t.Helper()
	manager := testAuthManager(t)
	tenantID := uuid.New()
	token := accessToken(t, manager, tenantID, uuid.New(), uuid.New())
	router := NewRouter(Dependencies{
		OrgStructureUseCase: org, MergePartiesUseCase: merge, TokenManager: manager,
	})
	return router, token, tenantID
}

func sampleJob(tenantID uuid.UUID) *domain.Job {
	money := domain.NewMoney(100000)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	return &domain.Job{ID: uuid.New(), TenantID: tenantID, JobFamilyID: uuid.New(), Title: "Engineer", Code: "ENG", MinSalary: &money, CreatedAt: now, UpdatedAt: now}
}

func TestCreateJob(t *testing.T) {
	tenantID := uuid.New()
	job := sampleJob(tenantID)
	uc := &fakeOrgUseCase{job: job}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/jobs",
		[]byte(`{"job_family_id":"`+job.JobFamilyID.String()+`","title":"Engineer","code":"ENG","min_salary":"1000.00"}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	body := decodeBody(t, recorder)
	data := body["data"].(map[string]any)
	assert.Equal(t, "ENG", data["code"])
	assert.Equal(t, "1000.00", data["min_salary"])
	assert.Equal(t, job.JobFamilyID, uc.jobInput.JobFamilyID)
	assert.Equal(t, int64(100000), uc.jobInput.MinSalary.CentsValue())
}

func TestListJobs(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeOrgUseCase{jobs: []*domain.Job{sampleJob(tenantID)}}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/jobs", nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeBody(t, recorder)["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "Engineer", data[0].(map[string]any)["title"])
}

func TestUpdateJob(t *testing.T) {
	tenantID := uuid.New()
	job := sampleJob(tenantID)
	uc := &fakeOrgUseCase{job: job}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPut, "/api/v1/jobs/"+job.ID.String(),
		[]byte(`{"job_family_id":"`+job.JobFamilyID.String()+`","title":"Senior Engineer","code":"ENG2"}`), token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, job.ID, uc.updateJob.ID)
	assert.Equal(t, "Senior Engineer", uc.updateJob.Title)
}

func TestCreateDepartmentWithManager(t *testing.T) {
	tenantID := uuid.New()
	managerID := uuid.New()
	department := &domain.Department{ID: uuid.New(), TenantID: tenantID, Name: "Engineering", Code: "ENG", ManagerPartyID: &managerID}
	uc := &fakeOrgUseCase{department: department}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/departments",
		[]byte(`{"name":"Engineering","code":"ENG","unit_type":"BRANCH","manager_party_id":"`+managerID.String()+`"}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotNil(t, uc.deptInput.ManagerPartyID)
	assert.Equal(t, managerID, *uc.deptInput.ManagerPartyID)
	assert.Equal(t, domain.DepartmentUnitTypeBranch, uc.deptInput.UnitType)
	data := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, managerID.String(), data["manager_party_id"])
}

func TestCreateDepartmentRejectsInvalidUnitType(t *testing.T) {
	uc := &fakeOrgUseCase{}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/departments",
		[]byte(`{"name":"Engineering","code":"ENG","unit_type":"INVALID"}`), token)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestCreateAndListEmployees(t *testing.T) {
	tenantID := uuid.New()
	partyID, jobID, departmentID := uuid.New(), uuid.New(), uuid.New()
	hireDate := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	employee := &domain.Employee{PartyID: partyID, TenantID: tenantID, EmployeeCode: "EMP-1", HireDate: hireDate, Status: domain.EmployeeStatusActive, JobID: jobID, DepartmentID: departmentID}
	uc := &fakeOrgUseCase{employee: employee, employees: []*domain.Employee{employee}}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	create := requestWithToken(router, http.MethodPost, "/api/v1/employees",
		[]byte(`{"party_id":"`+partyID.String()+`","employee_code":"EMP-1","hire_date":"2026-11-10T00:00:00Z","job_id":"`+jobID.String()+`","department_id":"`+departmentID.String()+`"}`), token)
	require.Equal(t, http.StatusCreated, create.Code)
	assert.Equal(t, partyID, uc.empInput.PartyID)

	list := requestWithToken(router, http.MethodGet, "/api/v1/employees", nil, token)
	require.Equal(t, http.StatusOK, list.Code)
	data := decodeBody(t, list)["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "EMP-1", data[0].(map[string]any)["employee_code"])
}

func TestMergeParties(t *testing.T) {
	sourceID, targetID := uuid.New(), uuid.New()
	mergedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	merge := &fakeMergeUseCase{result: &domain.MergeResult{SourceID: sourceID, TargetID: targetID, MergedAt: mergedAt}}
	router, token, _ := orgTestRouter(t, &fakeOrgUseCase{}, merge)

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/parties/merge",
		[]byte(`{"source_party_id":"`+sourceID.String()+`","target_party_id":"`+targetID.String()+`"}`), token)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, sourceID, merge.source)
	assert.Equal(t, targetID, merge.target)
	data := decodeBody(t, recorder)["data"].(map[string]any)
	assert.Equal(t, sourceID.String(), data["source_id"])
	assert.Equal(t, targetID.String(), data["target_id"])
}

func TestOrgAndMergeRoutesRequireJWT(t *testing.T) {
	router := NewRouter(Dependencies{TokenManager: testAuthManager(t)})
	routes := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodGet, "/api/v1/jobs", nil},
		{http.MethodPost, "/api/v1/jobs", []byte(`{}`)},
		{http.MethodPut, "/api/v1/jobs/" + uuid.New().String(), []byte(`{}`)},
		{http.MethodGet, "/api/v1/departments", nil},
		{http.MethodPost, "/api/v1/departments", []byte(`{}`)},
		{http.MethodGet, "/api/v1/employees", nil},
		{http.MethodPost, "/api/v1/employees", []byte(`{}`)},
		{http.MethodPost, "/api/v1/parties/merge", []byte(`{}`)},
	}
	for _, route := range routes {
		recorder := requestWithToken(router, route.method, route.path, route.body, "")
		assert.Equal(t, http.StatusUnauthorized, recorder.Code, "%s %s", route.method, route.path)
	}
}

func TestListDepartments(t *testing.T) {
	tenantID := uuid.New()
	uc := &fakeOrgUseCase{departments: []*domain.Department{{ID: uuid.New(), TenantID: tenantID, Name: "Engineering", Code: "ENG"}}}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/departments", nil, token)

	require.Equal(t, http.StatusOK, recorder.Code)
	data := decodeBody(t, recorder)["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "Engineering", data[0].(map[string]any)["name"])
}

func TestOrgHandlerRejectsInvalidPayloads(t *testing.T) {
	router, token, _ := orgTestRouter(t, &fakeOrgUseCase{}, &fakeMergeUseCase{})

	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodPost, "/api/v1/jobs", []byte(`{"job_family_id":"not-a-uuid","title":"x","code":"x"}`)},
		{http.MethodPost, "/api/v1/jobs", []byte(`{"job_family_id":"` + uuid.New().String() + `","title":"x","code":"x","min_salary":"bad"}`)},
		{http.MethodPut, "/api/v1/jobs/not-a-uuid", []byte(`{"job_family_id":"` + uuid.New().String() + `","title":"x","code":"x"}`)},
		{http.MethodPost, "/api/v1/departments", []byte(`{"name":"x","code":"x","manager_party_id":"bad"}`)},
		{http.MethodPost, "/api/v1/employees", []byte(`{"party_id":"bad","employee_code":"x","hire_date":"2026-11-10T00:00:00Z","job_id":"` + uuid.New().String() + `","department_id":"` + uuid.New().String() + `"}`)},
		{http.MethodPost, "/api/v1/parties/merge", []byte(`{"source_party_id":"bad","target_party_id":"` + uuid.New().String() + `"}`)},
	}
	for _, tc := range cases {
		recorder := requestWithToken(router, tc.method, tc.path, tc.body, token)
		assert.Equal(t, http.StatusBadRequest, recorder.Code, "%s %s", tc.method, tc.path)
	}
}

func TestOrgHandlerMapsUseCaseError(t *testing.T) {
	uc := &fakeOrgUseCase{err: errors.New("database down")}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/jobs", nil, token)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func (f *fakeOrgUseCase) CreateCorporateGroup(ctx context.Context, in usecase.CreateCorporateGroupInput) (*domain.CorporateGroup, error) {
	f.groupInput = in
	if f.err != nil {
		return nil, f.err
	}
	return f.group, nil
}

func (f *fakeOrgUseCase) AssignDepartmentToGroup(ctx context.Context, in usecase.AssignDepartmentToGroupInput) error {
	f.assignInput = in
	return f.assignErr
}

func TestCreateCorporateGroup(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	uc := &fakeOrgUseCase{}
	router, token, tenantID := orgTestRouter(t, uc, &fakeMergeUseCase{})
	uc.group = &domain.CorporateGroup{ID: uuid.New(), TenantID: tenantID, Name: "Holding", CreatedAt: now, UpdatedAt: now}

	recorder := requestWithToken(router, http.MethodPost, "/api/v1/corporate-groups",
		[]byte(`{"name":"Holding","description":"Grupo principal"}`), token)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, tenantID, uc.groupInput.TenantID)
	assert.Equal(t, "Holding", uc.groupInput.Name)
	assert.Equal(t, "Grupo principal", uc.groupInput.Description)
}

func TestAssignDepartmentToGroup(t *testing.T) {
	departmentID := uuid.New()
	groupID := uuid.New()
	uc := &fakeOrgUseCase{}
	router, token, tenantID := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/departments/"+departmentID.String()+"/groups",
		[]byte(`{"corporate_group_id":"`+groupID.String()+`"}`), token)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, tenantID, uc.assignInput.TenantID)
	assert.Equal(t, departmentID, uc.assignInput.DepartmentID)
	assert.Equal(t, groupID, uc.assignInput.CorporateGroupID)
}

func TestAssignDepartmentToGroupRejectsForeignGroup(t *testing.T) {
	uc := &fakeOrgUseCase{assignErr: domain.ErrCorporateGroupTenantMismatch}
	router, token, _ := orgTestRouter(t, uc, &fakeMergeUseCase{})

	recorder := requestWithToken(router, http.MethodPost,
		"/api/v1/departments/"+uuid.New().String()+"/groups",
		[]byte(`{"corporate_group_id":"`+uuid.New().String()+`"}`), token)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}
