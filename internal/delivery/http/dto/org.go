package dto

import (
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type CreateJobRequest struct {
	JobFamilyID string  `json:"job_family_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Code        string  `json:"code" binding:"required"`
	Description string  `json:"description,omitempty"`
	MinSalary   *string `json:"min_salary,omitempty"`
	MaxSalary   *string `json:"max_salary,omitempty"`
}

type UpdateJobRequest struct {
	JobFamilyID string  `json:"job_family_id" binding:"required"`
	Title       string  `json:"title" binding:"required"`
	Code        string  `json:"code" binding:"required"`
	Description string  `json:"description,omitempty"`
	MinSalary   *string `json:"min_salary,omitempty"`
	MaxSalary   *string `json:"max_salary,omitempty"`
}

type JobResponse struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	JobFamilyID string    `json:"job_family_id"`
	Title       string    `json:"title"`
	Code        string    `json:"code"`
	Description string    `json:"description,omitempty"`
	MinSalary   *string   `json:"min_salary,omitempty"`
	MaxSalary   *string   `json:"max_salary,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateDepartmentRequest struct {
	ParentDepartmentID *string `json:"parent_department_id,omitempty"`
	Name               string  `json:"name" binding:"required"`
	Code               string  `json:"code" binding:"required"`
	UnitType           string  `json:"unit_type,omitempty" binding:"omitempty,oneof=DIVISION BRANCH OFFICE DEPARTMENT"`
	ManagerPartyID     *string `json:"manager_party_id,omitempty"`
}

type DepartmentResponse struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	ParentDepartmentID *string   `json:"parent_department_id,omitempty"`
	Name               string    `json:"name"`
	Code               string    `json:"code"`
	UnitType           string    `json:"unit_type"`
	ManagerPartyID     *string   `json:"manager_party_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateCorporateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
}

type CorporateGroupResponse struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

type AssignDepartmentToGroupRequest struct {
	CorporateGroupID string `json:"corporate_group_id" binding:"required"`
}

type CreateEmployeeRequest struct {
	PartyID        string     `json:"party_id" binding:"required"`
	EmployeeCode   string     `json:"employee_code" binding:"required"`
	HireDate       *time.Time `json:"hire_date" binding:"required"`
	Status         string     `json:"status,omitempty"`
	JobID          string     `json:"job_id" binding:"required"`
	DepartmentID   string     `json:"department_id" binding:"required"`
	ManagerPartyID *string    `json:"manager_party_id,omitempty"`
}

type EmployeeResponse struct {
	PartyID        string    `json:"party_id"`
	TenantID       string    `json:"tenant_id"`
	EmployeeCode   string    `json:"employee_code"`
	HireDate       time.Time `json:"hire_date"`
	Status         string    `json:"status"`
	JobID          string    `json:"job_id"`
	DepartmentID   string    `json:"department_id"`
	ManagerPartyID *string   `json:"manager_party_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type JobEnvelope struct {
	Data JobResponse `json:"data"`
}

type JobListEnvelope struct {
	Data []JobResponse `json:"data"`
}

type DepartmentEnvelope struct {
	Data DepartmentResponse `json:"data"`
}

type DepartmentListEnvelope struct {
	Data []DepartmentResponse `json:"data"`
}

type CorporateGroupEnvelope struct {
	Data CorporateGroupResponse `json:"data"`
}

type EmployeeEnvelope struct {
	Data EmployeeResponse `json:"data"`
}

type EmployeeListEnvelope struct {
	Data []EmployeeResponse `json:"data"`
}

func NewJobResponse(job *domain.Job) JobResponse {
	if job == nil {
		return JobResponse{}
	}
	return JobResponse{
		ID: job.ID.String(), TenantID: job.TenantID.String(), JobFamilyID: job.JobFamilyID.String(),
		Title: job.Title, Code: job.Code, Description: job.Description,
		MinSalary: moneyPtr(job.MinSalary), MaxSalary: moneyPtr(job.MaxSalary),
		CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt,
	}
}

func NewJobListResponse(jobs []*domain.Job) []JobResponse {
	result := make([]JobResponse, 0, len(jobs))
	for _, job := range jobs {
		result = append(result, NewJobResponse(job))
	}
	return result
}

func NewDepartmentResponse(department *domain.Department) DepartmentResponse {
	if department == nil {
		return DepartmentResponse{}
	}
	return DepartmentResponse{
		ID: department.ID.String(), TenantID: department.TenantID.String(),
		ParentDepartmentID: uuidPtr(department.ParentDepartmentID),
		Name:               department.Name, Code: department.Code,
		UnitType:       string(department.UnitType),
		ManagerPartyID: uuidPtr(department.ManagerPartyID),
		CreatedAt:      department.CreatedAt, UpdatedAt: department.UpdatedAt,
	}
}

func NewCorporateGroupResponse(group *domain.CorporateGroup) CorporateGroupResponse {
	if group == nil {
		return CorporateGroupResponse{}
	}
	return CorporateGroupResponse{
		ID: group.ID.String(), TenantID: group.TenantID.String(),
		Name: group.Name, Description: group.Description,
		CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt, DeletedAt: group.DeletedAt,
	}
}

func NewDepartmentListResponse(departments []*domain.Department) []DepartmentResponse {
	result := make([]DepartmentResponse, 0, len(departments))
	for _, department := range departments {
		result = append(result, NewDepartmentResponse(department))
	}
	return result
}

func NewEmployeeResponse(employee *domain.Employee) EmployeeResponse {
	if employee == nil {
		return EmployeeResponse{}
	}
	return EmployeeResponse{
		PartyID: employee.PartyID.String(), TenantID: employee.TenantID.String(),
		EmployeeCode: employee.EmployeeCode, HireDate: employee.HireDate,
		Status: string(employee.Status), JobID: employee.JobID.String(),
		DepartmentID:   employee.DepartmentID.String(),
		ManagerPartyID: uuidPtr(employee.ManagerPartyID),
		CreatedAt:      employee.CreatedAt, UpdatedAt: employee.UpdatedAt,
	}
}

func NewEmployeeListResponse(employees []*domain.Employee) []EmployeeResponse {
	result := make([]EmployeeResponse, 0, len(employees))
	for _, employee := range employees {
		result = append(result, NewEmployeeResponse(employee))
	}
	return result
}

func moneyPtr(money *domain.Money) *string {
	if money == nil {
		return nil
	}
	value := money.String()
	return &value
}

func uuidPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := id.String()
	return &value
}
