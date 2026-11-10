package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	JobFamilyID uuid.UUID
	Title       string
	Code        string
	Description string
	MinSalary   *Money
	MaxSalary   *Money
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func NewJob(
	tenantID uuid.UUID,
	jobFamilyID uuid.UUID,
	title, code, description string,
	minSalary, maxSalary *Money,
	now time.Time,
) (*Job, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate job id: %w", err)
	}

	now = now.UTC()
	job := &Job{
		ID:          id,
		TenantID:    tenantID,
		JobFamilyID: jobFamilyID,
		Title:       strings.TrimSpace(title),
		Code:        strings.TrimSpace(code),
		Description: strings.TrimSpace(description),
		MinSalary:   minSalary,
		MaxSalary:   maxSalary,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := job.Validate(); err != nil {
		return nil, err
	}
	return job, nil
}

func (j *Job) Validate() error {
	if j == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidJob)
	}
	if j.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidJob)
	}
	if j.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidJob)
	}
	if j.JobFamilyID == uuid.Nil {
		return fmt.Errorf("%w: job_family_id is required", ErrInvalidJob)
	}
	if strings.TrimSpace(j.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidJob)
	}
	if strings.TrimSpace(j.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrInvalidJob)
	}
	if j.MinSalary != nil && j.MinSalary.IsNegative() {
		return fmt.Errorf("%w: min_salary cannot be negative", ErrInvalidJob)
	}
	if j.MaxSalary != nil && j.MaxSalary.IsNegative() {
		return fmt.Errorf("%w: max_salary cannot be negative", ErrInvalidJob)
	}
	if j.MinSalary != nil && j.MaxSalary != nil && j.MinSalary.GreaterThan(*j.MaxSalary) {
		return fmt.Errorf("%w: min_salary cannot be greater than max_salary", ErrInvalidJob)
	}
	if j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidJob)
	}
	if j.UpdatedAt.Before(j.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidJob)
	}
	return nil
}

func (j *Job) Update(
	jobFamilyID uuid.UUID,
	title, code, description string,
	minSalary, maxSalary *Money,
	now time.Time,
) error {
	if j.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	j.JobFamilyID = jobFamilyID
	j.Title = strings.TrimSpace(title)
	j.Code = strings.TrimSpace(code)
	j.Description = strings.TrimSpace(description)
	j.MinSalary = minSalary
	j.MaxSalary = maxSalary
	j.UpdatedAt = now.UTC()
	return j.Validate()
}

func (j *Job) SoftDelete(now time.Time) error {
	if j.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	j.DeletedAt = &now
	j.UpdatedAt = now
	return nil
}
