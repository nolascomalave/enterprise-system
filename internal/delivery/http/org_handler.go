package http

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/delivery/http/dto"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"enterprise-system/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrgStructureUseCase interface {
	CreateJob(ctx context.Context, in usecase.CreateJobInput) (*domain.Job, error)
	ListJobs(ctx context.Context) ([]*domain.Job, error)
	UpdateJob(ctx context.Context, in usecase.UpdateJobInput) (*domain.Job, error)

	CreateDepartment(ctx context.Context, in usecase.CreateDepartmentInput) (*domain.Department, error)
	ListDepartments(ctx context.Context) ([]*domain.Department, error)

	CreateCorporateGroup(ctx context.Context, in usecase.CreateCorporateGroupInput) (*domain.CorporateGroup, error)
	AssignDepartmentToGroup(ctx context.Context, in usecase.AssignDepartmentToGroupInput) error

	CreateEmployee(ctx context.Context, in usecase.CreateEmployeeInput) (*domain.Employee, error)
	ListEmployees(ctx context.Context) ([]*domain.Employee, error)
}

type OrgHandler struct {
	uc      OrgStructureUseCase
	catalog CatalogUseCase
}

func NewOrgHandler(uc OrgStructureUseCase, catalog CatalogUseCase) *OrgHandler {
	return &OrgHandler{uc: uc, catalog: catalog}
}

func (h *OrgHandler) session(c *gin.Context) (enttenant.SessionContext, bool) {
	sc, ok := enttenant.SessionContextFrom(c.Request.Context())
	if !ok {
		c.Error(unauthorizedError(ErrUnauthorized))
		return enttenant.SessionContext{}, false
	}
	return sc, true
}

// CreateJob godoc
// @Summary      Create a job
// @Description  Creates a tenant-scoped job inside a job family.
// @Tags         Jobs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateJobRequest true "Job data"
// @Success      201 {object} dto.JobEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /jobs [post]
func (h *OrgHandler) CreateJob(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.CreateJobRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	familyID, err := parseRequiredUUID(request.JobFamilyID, "job_family_id")
	if err != nil {
		c.Error(err)
		return
	}
	minSalary, err := parseOptionalMoney(request.MinSalary, "min_salary")
	if err != nil {
		c.Error(err)
		return
	}
	maxSalary, err := parseOptionalMoney(request.MaxSalary, "max_salary")
	if err != nil {
		c.Error(err)
		return
	}
	job, err := h.uc.CreateJob(c.Request.Context(), usecase.CreateJobInput{
		TenantID: sc.TenantID, JobFamilyID: familyID, Title: request.Title, Code: request.Code,
		Description: request.Description, MinSalary: minSalary, MaxSalary: maxSalary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewJobResponse(job)})
}

// ListJobs godoc
// @Summary      List jobs
// @Description  Lists ACTIVE jobs for the authenticated tenant.
// @Tags         Jobs
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.JobListEnvelope
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /jobs [get]
func (h *OrgHandler) ListJobs(c *gin.Context) {
	if _, ok := h.session(c); !ok {
		return
	}
	jobs, err := h.uc.ListJobs(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewJobListResponse(jobs)})
}

// UpdateJob godoc
// @Summary      Update a job
// @Tags         Jobs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Job UUID"
// @Param        request body dto.UpdateJobRequest true "Updated job data"
// @Success      200 {object} dto.JobEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /jobs/{id} [put]
func (h *OrgHandler) UpdateJob(c *gin.Context) {
	if _, ok := h.session(c); !ok {
		return
	}
	id, err := parseRequiredUUID(c.Param("id"), "id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.UpdateJobRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	familyID, err := parseRequiredUUID(request.JobFamilyID, "job_family_id")
	if err != nil {
		c.Error(err)
		return
	}
	minSalary, err := parseOptionalMoney(request.MinSalary, "min_salary")
	if err != nil {
		c.Error(err)
		return
	}
	maxSalary, err := parseOptionalMoney(request.MaxSalary, "max_salary")
	if err != nil {
		c.Error(err)
		return
	}
	job, err := h.uc.UpdateJob(c.Request.Context(), usecase.UpdateJobInput{
		ID: id, JobFamilyID: familyID, Title: request.Title, Code: request.Code,
		Description: request.Description, MinSalary: minSalary, MaxSalary: maxSalary,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewJobResponse(job)})
}

// CreateDepartment godoc
// @Summary      Create a department
// @Description  Creates a department and optionally assigns an employee manager.
// @Tags         Departments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateDepartmentRequest true "Department data"
// @Success      201 {object} dto.DepartmentEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /departments [post]
func (h *OrgHandler) CreateDepartment(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.CreateDepartmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	parentID, err := parseOptionalUUID(request.ParentDepartmentID, "parent_department_id")
	if err != nil {
		c.Error(err)
		return
	}
	managerID, err := parseOptionalUUID(request.ManagerPartyID, "manager_party_id")
	if err != nil {
		c.Error(err)
		return
	}
	department, err := h.uc.CreateDepartment(c.Request.Context(), usecase.CreateDepartmentInput{
		TenantID: sc.TenantID, ParentDepartmentID: parentID, Name: request.Name,
		Code: request.Code, UnitType: domain.DepartmentUnitType(request.UnitType),
		ManagerPartyID: managerID,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewDepartmentResponse(department)})
}

// ListDepartments godoc
// @Summary      List departments
// @Tags         Departments
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.DepartmentListEnvelope
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /departments [get]
func (h *OrgHandler) ListDepartments(c *gin.Context) {
	if _, ok := h.session(c); !ok {
		return
	}
	if h.catalog != nil {
		departments, err := h.catalog.ListDepartments(c.Request.Context(), LocaleFromContext(c.Request.Context()))
		if err != nil {
			c.Error(err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": dto.NewLocalizedDepartmentListResponse(departments)})
		return
	}
	departments, err := h.uc.ListDepartments(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewDepartmentListResponse(departments)})
}

// CreateCorporateGroup godoc
// @Summary      Create a corporate group
// @Description  Creates a tenant-scoped group used to aggregate departments across units.
// @Tags         Corporate Groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateCorporateGroupRequest true "Corporate group data"
// @Success      201 {object} dto.CorporateGroupEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /corporate-groups [post]
func (h *OrgHandler) CreateCorporateGroup(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.CreateCorporateGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	group, err := h.uc.CreateCorporateGroup(c.Request.Context(), usecase.CreateCorporateGroupInput{
		TenantID: sc.TenantID, Name: request.Name, Description: request.Description,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewCorporateGroupResponse(group)})
}

// AssignDepartmentToGroup godoc
// @Summary      Assign a department to a corporate group
// @Description  Links a tenant department to a corporate group. Both must belong to the same tenant.
// @Tags         Corporate Groups
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Department UUID"
// @Param        request body dto.AssignDepartmentToGroupRequest true "Corporate group id"
// @Success      204
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      404 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /departments/{id}/groups [post]
func (h *OrgHandler) AssignDepartmentToGroup(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	departmentID, err := parseRequiredUUID(c.Param("id"), "department_id")
	if err != nil {
		c.Error(err)
		return
	}
	var request dto.AssignDepartmentToGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	groupID, err := uuid.Parse(strings.TrimSpace(request.CorporateGroupID))
	if err != nil || groupID == uuid.Nil {
		c.Error(apperror.NewValidation("invalid_corporate_group_id", "invalid corporate_group_id", err))
		return
	}
	if err := h.uc.AssignDepartmentToGroup(c.Request.Context(), usecase.AssignDepartmentToGroupInput{
		TenantID: sc.TenantID, DepartmentID: departmentID, CorporateGroupID: groupID,
	}); err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateEmployee godoc
// @Summary      Create an employee
// @Description  Converts an existing INDIVIDUAL party into an employee with a job and department.
// @Tags         Employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateEmployeeRequest true "Employee data"
// @Success      201 {object} dto.EmployeeEnvelope
// @Failure      400 {object} dto.ErrorResponse
// @Failure      401 {object} dto.ErrorResponse
// @Failure      409 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /employees [post]
func (h *OrgHandler) CreateEmployee(c *gin.Context) {
	sc, ok := h.session(c)
	if !ok {
		return
	}
	var request dto.CreateEmployeeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Error(badRequestError(err))
		return
	}
	partyID, err := parseRequiredUUID(request.PartyID, "party_id")
	if err != nil {
		c.Error(err)
		return
	}
	jobID, err := parseRequiredUUID(request.JobID, "job_id")
	if err != nil {
		c.Error(err)
		return
	}
	departmentID, err := parseRequiredUUID(request.DepartmentID, "department_id")
	if err != nil {
		c.Error(err)
		return
	}
	managerID, err := parseOptionalUUID(request.ManagerPartyID, "manager_party_id")
	if err != nil {
		c.Error(err)
		return
	}
	if request.HireDate == nil {
		c.Error(badRequestError(errHireDateRequired))
		return
	}
	employee, err := h.uc.CreateEmployee(c.Request.Context(), usecase.CreateEmployeeInput{
		PartyID: partyID, TenantID: sc.TenantID, EmployeeCode: request.EmployeeCode,
		HireDate: *request.HireDate, Status: domain.EmployeeStatus(request.Status),
		JobID: jobID, DepartmentID: departmentID, ManagerPartyID: managerID,
	})
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": dto.NewEmployeeResponse(employee)})
}

// ListEmployees godoc
// @Summary      List employees
// @Tags         Employees
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} dto.EmployeeListEnvelope
// @Failure      401 {object} dto.ErrorResponse
// @Failure      500 {object} dto.ErrorResponse
// @Router       /employees [get]
func (h *OrgHandler) ListEmployees(c *gin.Context) {
	if _, ok := h.session(c); !ok {
		return
	}
	employees, err := h.uc.ListEmployees(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": dto.NewEmployeeListResponse(employees)})
}

func parseRequiredUUID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(value))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, apperror.NewValidation("bad_request", "bad request", fmt.Errorf("invalid %s", field))
	}
	return id, nil
}

func parseOptionalUUID(value *string, field string) (*uuid.UUID, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	id, err := parseRequiredUUID(*value, field)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseOptionalMoney(value *string, field string) (*domain.Money, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	money, err := domain.ParseMoney(strings.TrimSpace(*value))
	if err != nil {
		return nil, apperror.NewValidation("bad_request", "bad request", fmt.Errorf("invalid %s: %w", field, err))
	}
	return &money, nil
}
