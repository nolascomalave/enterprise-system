package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var roleTypeCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// RoleType is a global catalog entry. It has no tenant_id and no RLS.
type RoleType struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	IsSystem    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func NewRoleType(code, name, description string, isSystem bool, now time.Time) (*RoleType, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate role type id: %w", err)
	}

	now = now.UTC()
	role := &RoleType{
		ID:          id,
		Code:        normalizeRoleTypeCode(code),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		IsSystem:    isSystem,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := role.Validate(); err != nil {
		return nil, err
	}
	return role, nil
}

func (r *RoleType) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidRoleType)
	}
	if r.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidRoleType)
	}
	if !roleTypeCodePattern.MatchString(r.Code) {
		return fmt.Errorf("%w: code must match %s", ErrInvalidRoleType, roleTypeCodePattern.String())
	}
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRoleType)
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidRoleType)
	}
	if r.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidRoleType)
	}
	if r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidRoleType)
	}
	if r.DeletedAt != nil && r.DeletedAt.Before(r.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidRoleType)
	}
	return nil
}

func (r *RoleType) SoftDelete(now time.Time) error {
	if r.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	r.DeletedAt = &now
	r.UpdatedAt = now
	return nil
}

func normalizeRoleTypeCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))

	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore && b.Len() > 0:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
