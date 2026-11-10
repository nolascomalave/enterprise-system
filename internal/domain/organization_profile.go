package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OrganizationProfile is a composition (1:1) of Party for ORGANIZATION
// parties. It has its own UUIDv7 primary key and a unique party_id.
type OrganizationProfile struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	PartyID   uuid.UUID
	LegalName string
	TradeName string
	TaxID     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewOrganizationProfile(
	tenantID uuid.UUID,
	partyID uuid.UUID,
	legalName string,
	tradeName string,
	taxID string,
	now time.Time,
) (*OrganizationProfile, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate organization profile id: %w", err)
	}

	now = now.UTC()
	profile := &OrganizationProfile{
		ID:        id,
		TenantID:  tenantID,
		PartyID:   partyID,
		LegalName: strings.TrimSpace(legalName),
		TradeName: strings.TrimSpace(tradeName),
		TaxID:     strings.TrimSpace(taxID),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := profile.Validate(); err != nil {
		return nil, err
	}
	return profile, nil
}

func (o *OrganizationProfile) Validate() error {
	if o == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidOrganizationProfile)
	}
	if o.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidOrganizationProfile)
	}
	if o.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidOrganizationProfile)
	}
	if o.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidOrganizationProfile)
	}
	if strings.TrimSpace(o.LegalName) == "" {
		return fmt.Errorf("%w: legal_name is required", ErrInvalidOrganizationProfile)
	}
	if strings.TrimSpace(o.TaxID) == "" {
		return fmt.Errorf("%w: tax_id is required", ErrInvalidOrganizationProfile)
	}
	if o.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidOrganizationProfile)
	}
	if o.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidOrganizationProfile)
	}
	if o.UpdatedAt.Before(o.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidOrganizationProfile)
	}
	if o.DeletedAt != nil && o.DeletedAt.Before(o.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidOrganizationProfile)
	}
	return nil
}

func (o *OrganizationProfile) SoftDelete(now time.Time) error {
	if o.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	o.DeletedAt = &now
	o.UpdatedAt = now
	return nil
}
