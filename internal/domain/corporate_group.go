package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CorporateGroup groups departments across the tenant's organizational units.
type CorporateGroup struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func NewCorporateGroup(tenantID uuid.UUID, name, description string, now time.Time) (*CorporateGroup, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate corporate group id: %w", err)
	}
	now = now.UTC()
	group := &CorporateGroup{
		ID: id, TenantID: tenantID,
		Name: strings.TrimSpace(name), CreatedAt: now, UpdatedAt: now,
	}
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		group.Description = &trimmed
	}
	if err := group.Validate(); err != nil {
		return nil, err
	}
	return group, nil
}

func (g *CorporateGroup) Validate() error {
	if g == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidCorporateGroup)
	}
	if g.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidCorporateGroup)
	}
	if g.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidCorporateGroup)
	}
	if strings.TrimSpace(g.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCorporateGroup)
	}
	if g.CreatedAt.IsZero() || g.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidCorporateGroup)
	}
	if g.UpdatedAt.Before(g.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidCorporateGroup)
	}
	return nil
}

func (g *CorporateGroup) SoftDelete(now time.Time) error {
	if g.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	g.DeletedAt = &now
	g.UpdatedAt = now
	return nil
}
