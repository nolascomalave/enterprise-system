package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Department struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	ParentDepartmentID *uuid.UUID
	Name               string
	Code               string
	ManagerPartyID     *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DeletedAt          *time.Time
}

func NewDepartment(
	tenantID uuid.UUID,
	parentDepartmentID *uuid.UUID,
	name, code string,
	managerPartyID *uuid.UUID,
	now time.Time,
) (*Department, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate department id: %w", err)
	}

	now = now.UTC()
	dept := &Department{
		ID:                 id,
		TenantID:           tenantID,
		ParentDepartmentID: parentDepartmentID,
		Name:               strings.TrimSpace(name),
		Code:               strings.TrimSpace(code),
		ManagerPartyID:     managerPartyID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := dept.Validate(); err != nil {
		return nil, err
	}
	return dept, nil
}

func (d *Department) Validate() error {
	if d == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidDepartment)
	}
	if d.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidDepartment)
	}
	if d.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidDepartment)
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidDepartment)
	}
	if strings.TrimSpace(d.Code) == "" {
		return fmt.Errorf("%w: code is required", ErrInvalidDepartment)
	}
	if d.ParentDepartmentID != nil {
		if *d.ParentDepartmentID == uuid.Nil || *d.ParentDepartmentID == d.ID {
			return fmt.Errorf("%w: invalid parent_department_id", ErrInvalidDepartment)
		}
	}
	if d.ManagerPartyID != nil && *d.ManagerPartyID == uuid.Nil {
		return fmt.Errorf("%w: manager_party_id cannot be nil uuid", ErrInvalidDepartment)
	}
	if d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidDepartment)
	}
	if d.UpdatedAt.Before(d.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidDepartment)
	}
	return nil
}

func (d *Department) Update(
	parentDepartmentID *uuid.UUID,
	name, code string,
	managerPartyID *uuid.UUID,
	now time.Time,
) error {
	if d.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	d.ParentDepartmentID = parentDepartmentID
	d.Name = strings.TrimSpace(name)
	d.Code = strings.TrimSpace(code)
	d.ManagerPartyID = managerPartyID
	d.UpdatedAt = now.UTC()
	return d.Validate()
}

func (d *Department) SoftDelete(now time.Time) error {
	if d.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	d.DeletedAt = &now
	d.UpdatedAt = now
	return nil
}
