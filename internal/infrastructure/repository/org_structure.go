package repository

import (
	"context"

	"enterprise-system/ent"
	"enterprise-system/ent/department"
	"enterprise-system/ent/departmenttranslation"
	"enterprise-system/ent/employee"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.OrgStructureRepository = (*OrgStructureRepository)(nil)

type OrgStructureRepository struct {
	client *ent.Client
}

func NewOrgStructureRepository(client *ent.Client) *OrgStructureRepository {
	return &OrgStructureRepository{client: client}
}

func (r *OrgStructureRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

// JobFamily -----------------------------------------------------------------

func (r *OrgStructureRepository) CreateJobFamily(ctx context.Context, e *domain.JobFamily) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.JobFamily.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetName(e.Name).
			SetCode(e.Code).
			SetDescription(e.Description).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetJobFamily(ctx context.Context, id uuid.UUID) (*domain.JobFamily, error) {
	var result *domain.JobFamily
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.JobFamily.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainJobFamily(e)
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) ListJobFamilies(ctx context.Context) ([]*domain.JobFamily, error) {
	var result []*domain.JobFamily
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.JobFamily.Query().All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.JobFamily, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainJobFamily(e))
		}
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) UpdateJobFamily(ctx context.Context, e *domain.JobFamily) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.JobFamily.UpdateOneID(e.ID).
			SetName(e.Name).
			SetCode(e.Code).
			SetDescription(e.Description).
			SetUpdatedAt(e.UpdatedAt)
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

// Job -----------------------------------------------------------------------

func (r *OrgStructureRepository) CreateJob(ctx context.Context, e *domain.Job) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.Job.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetJobFamilyID(e.JobFamilyID).
			SetTitle(e.Title).
			SetCode(e.Code).
			SetDescription(e.Description).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.MinSalary != nil {
			create.SetMinSalary(e.MinSalary)
		}
		if e.MaxSalary != nil {
			create.SetMaxSalary(e.MaxSalary)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	var result *domain.Job
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.Job.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainJob(e)
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) ListJobs(ctx context.Context) ([]*domain.Job, error) {
	var result []*domain.Job
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.Job.Query().All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Job, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainJob(e))
		}
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) UpdateJob(ctx context.Context, e *domain.Job) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.Job.UpdateOneID(e.ID).
			SetJobFamilyID(e.JobFamilyID).
			SetTitle(e.Title).
			SetCode(e.Code).
			SetDescription(e.Description).
			SetUpdatedAt(e.UpdatedAt)
		if e.MinSalary != nil {
			update.SetMinSalary(e.MinSalary)
		} else {
			update.ClearMinSalary()
		}
		if e.MaxSalary != nil {
			update.SetMaxSalary(e.MaxSalary)
		} else {
			update.ClearMaxSalary()
		}
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

// Department ----------------------------------------------------------------

func (r *OrgStructureRepository) CreateDepartment(ctx context.Context, e *domain.Department) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.Department.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetName(e.Name).
			SetCode(e.Code).
			SetUnitType(department.UnitType(e.UnitType)).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.ParentDepartmentID != nil {
			create.SetParentDepartmentID(*e.ParentDepartmentID)
		}
		if e.ManagerPartyID != nil {
			create.SetManagerPartyID(*e.ManagerPartyID)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetDepartment(ctx context.Context, id uuid.UUID) (*domain.Department, error) {
	var result *domain.Department
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.Department.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartment(e)
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) ListDepartments(ctx context.Context) ([]*domain.Department, error) {
	var result []*domain.Department
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.Department.Query().All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Department, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainDepartment(e))
		}
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) UpdateDepartment(ctx context.Context, e *domain.Department) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.Department.UpdateOneID(e.ID).
			SetName(e.Name).
			SetCode(e.Code).
			SetUnitType(department.UnitType(e.UnitType)).
			SetUpdatedAt(e.UpdatedAt)
		if e.ParentDepartmentID != nil {
			update.SetParentDepartmentID(*e.ParentDepartmentID)
		} else {
			update.ClearParentDepartmentID()
		}
		if e.ManagerPartyID != nil {
			update.SetManagerPartyID(*e.ManagerPartyID)
		} else {
			update.ClearManagerPartyID()
		}
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetDepartmentTranslation(
	ctx context.Context,
	departmentID uuid.UUID,
	locale string,
) (string, error) {
	var result string
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.DepartmentTranslation.Query().
			Where(
				departmenttranslation.DepartmentID(departmentID),
				departmenttranslation.LocaleCodeEQ(locale),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = row.Name
		return nil
	})
	if ent.IsNotFound(err) {
		return "", domain.ErrTranslationNotFound
	}
	return result, err
}

// Corporate groups ----------------------------------------------------------

func (r *OrgStructureRepository) CreateCorporateGroup(ctx context.Context, e *domain.CorporateGroup) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.CorporateGroup.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetName(e.Name).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.Description != nil {
			create.SetDescription(*e.Description)
		}
		if e.DeletedAt != nil {
			create.SetDeletedAt(*e.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetCorporateGroup(ctx context.Context, id uuid.UUID) (*domain.CorporateGroup, error) {
	var result *domain.CorporateGroup
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.CorporateGroup.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainCorporateGroup(e)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrCorporateGroupNotFound
	}
	return result, err
}

func (r *OrgStructureRepository) AssignDepartmentToGroup(
	ctx context.Context,
	tenantID, departmentID, corporateGroupID uuid.UUID,
) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.DepartmentGroupAssignment.Create().
			SetTenantID(tenantID).
			SetDepartmentID(departmentID).
			SetCorporateGroupID(corporateGroupID).
			Save(ctx)
		return repoError(err)
	})
}

// Employee ------------------------------------------------------------------

func (r *OrgStructureRepository) CreateEmployee(ctx context.Context, e *domain.Employee) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.Employee.Create().
			SetID(e.PartyID).
			SetTenantID(e.TenantID).
			SetEmployeeCode(e.EmployeeCode).
			SetHireDate(e.HireDate).
			SetStatus(employee.Status(e.Status)).
			SetJobID(e.JobID).
			SetDepartmentID(e.DepartmentID).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.ManagerPartyID != nil {
			create.SetManagerPartyID(*e.ManagerPartyID)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetEmployee(ctx context.Context, partyID uuid.UUID) (*domain.Employee, error) {
	var result *domain.Employee
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.Employee.Get(ctx, partyID)
		if err != nil {
			return repoError(err)
		}
		result = toDomainEmployee(e)
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) ListEmployees(ctx context.Context) ([]*domain.Employee, error) {
	var result []*domain.Employee
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.Employee.Query().All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Employee, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainEmployee(e))
		}
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) UpdateEmployee(ctx context.Context, e *domain.Employee) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.Employee.UpdateOneID(e.PartyID).
			SetEmployeeCode(e.EmployeeCode).
			SetHireDate(e.HireDate).
			SetStatus(employee.Status(e.Status)).
			SetJobID(e.JobID).
			SetDepartmentID(e.DepartmentID).
			SetUpdatedAt(e.UpdatedAt)
		if e.ManagerPartyID != nil {
			update.SetManagerPartyID(*e.ManagerPartyID)
		} else {
			update.ClearManagerPartyID()
		}
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

// EmployeeAssignment --------------------------------------------------------

func (r *OrgStructureRepository) CreateEmployeeAssignment(ctx context.Context, e *domain.EmployeeAssignment) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.EmployeeAssignment.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetEmployeePartyID(e.EmployeePartyID).
			SetJobID(e.JobID).
			SetDepartmentID(e.DepartmentID).
			SetStartDate(e.StartDate).
			SetSalary(&e.Salary).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.EndDate != nil {
			create.SetEndDate(*e.EndDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *OrgStructureRepository) GetEmployeeAssignment(ctx context.Context, id uuid.UUID) (*domain.EmployeeAssignment, error) {
	var result *domain.EmployeeAssignment
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.EmployeeAssignment.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainEmployeeAssignment(e)
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) ListEmployeeAssignments(ctx context.Context) ([]*domain.EmployeeAssignment, error) {
	var result []*domain.EmployeeAssignment
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.EmployeeAssignment.Query().All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.EmployeeAssignment, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainEmployeeAssignment(e))
		}
		return nil
	})
	return result, err
}

func (r *OrgStructureRepository) UpdateEmployeeAssignment(ctx context.Context, e *domain.EmployeeAssignment) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.EmployeeAssignment.UpdateOneID(e.ID).
			SetJobID(e.JobID).
			SetDepartmentID(e.DepartmentID).
			SetStartDate(e.StartDate).
			SetSalary(&e.Salary).
			SetUpdatedAt(e.UpdatedAt)
		if e.EndDate != nil {
			update.SetEndDate(*e.EndDate)
		} else {
			update.ClearEndDate()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

// Mapping -------------------------------------------------------------------

func toDomainJobFamily(e *ent.JobFamily) *domain.JobFamily {
	return &domain.JobFamily{
		ID: e.ID, TenantID: e.TenantID, Name: e.Name, Code: e.Code,
		Description: e.Description, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}

func toDomainJob(e *ent.Job) *domain.Job {
	return &domain.Job{
		ID: e.ID, TenantID: e.TenantID, JobFamilyID: e.JobFamilyID,
		Title: e.Title, Code: e.Code, Description: e.Description,
		MinSalary: e.MinSalary, MaxSalary: e.MaxSalary,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainDepartment(e *ent.Department) *domain.Department {
	return &domain.Department{
		ID: e.ID, TenantID: e.TenantID, ParentDepartmentID: e.ParentDepartmentID,
		Name: e.Name, Code: e.Code, UnitType: domain.DepartmentUnitType(e.UnitType),
		ManagerPartyID: e.ManagerPartyID,
		CreatedAt:      e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainCorporateGroup(e *ent.CorporateGroup) *domain.CorporateGroup {
	return &domain.CorporateGroup{
		ID: e.ID, TenantID: e.TenantID, Name: e.Name, Description: e.Description,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainEmployee(e *ent.Employee) *domain.Employee {
	return &domain.Employee{
		PartyID: e.ID, TenantID: e.TenantID, EmployeeCode: e.EmployeeCode,
		HireDate: e.HireDate, Status: domain.EmployeeStatus(e.Status),
		JobID: e.JobID, DepartmentID: e.DepartmentID, ManagerPartyID: e.ManagerPartyID,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainEmployeeAssignment(e *ent.EmployeeAssignment) *domain.EmployeeAssignment {
	salary := domain.Money{}
	if e.Salary != nil {
		salary = *e.Salary
	}
	return &domain.EmployeeAssignment{
		ID: e.ID, TenantID: e.TenantID, EmployeePartyID: e.EmployeePartyID,
		JobID: e.JobID, DepartmentID: e.DepartmentID,
		StartDate: e.StartDate, EndDate: e.EndDate, Salary: salary,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
