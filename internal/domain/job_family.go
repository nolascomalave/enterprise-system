package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type JobFamily struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Code        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func NewJobFamily(tenantID uuid.UUID, name, code, description string, now time.Time) (*JobFamily, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate job family id: %w", err)
	}

	now = now.UTC()
	jf := &JobFamily{
		ID:          id,
		TenantID:    tenantID,
		Name:        strings.TrimSpace(name),
		Code:        strings.TrimSpace(code),
		Description: strings.TrimSpace(description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := jf.Validate(); err != nil {
		return nil, err
	}
	return jf, nil
}

func (j *JobFamily) Validate() error {
	if j == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidJobFamily)
	}
	if j.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidJobFamily)
	}
	if j.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidJobFamily)
	}
	if strings.TrimSpace(j.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidJobFamily)
	}
	if strings.TrimSpace(j.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrInvalidJobFamily)
	}
	if j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidJobFamily)
	}
	if j.UpdatedAt.Before(j.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidJobFamily)
	}
	return nil
}

func (j *JobFamily) Update(name, code, description string, now time.Time) error {
	if j.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	j.Name = strings.TrimSpace(name)
	j.Code = strings.TrimSpace(code)
	j.Description = strings.TrimSpace(description)
	j.UpdatedAt = now.UTC()
	return j.Validate()
}

func (j *JobFamily) SoftDelete(now time.Time) error {
	if j.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	j.DeletedAt = &now
	j.UpdatedAt = now
	return nil
}
