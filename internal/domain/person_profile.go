package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PersonProfile is a composition (1:1) of Party for INDIVIDUAL parties. It has
// its own UUIDv7 primary key and a unique party_id.
type PersonProfile struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	PartyID        uuid.UUID
	FirstName      string
	MiddleName     string
	LastName       string
	SecondLastName string
	BirthDate      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NewPersonProfile(
	tenantID uuid.UUID,
	partyID uuid.UUID,
	firstName string,
	middleName string,
	lastName string,
	secondLastName string,
	birthDate *time.Time,
	now time.Time,
) (*PersonProfile, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate person profile id: %w", err)
	}

	now = now.UTC()
	profile := &PersonProfile{
		ID:             id,
		TenantID:       tenantID,
		PartyID:        partyID,
		FirstName:      strings.TrimSpace(firstName),
		MiddleName:     strings.TrimSpace(middleName),
		LastName:       strings.TrimSpace(lastName),
		SecondLastName: strings.TrimSpace(secondLastName),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if birthDate != nil {
		bd := birthDate.UTC()
		profile.BirthDate = &bd
	}

	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if profile.BirthDate != nil && profile.BirthDate.After(now) {
		return nil, fmt.Errorf("%w: birth_date cannot be in the future", ErrInvalidPersonProfile)
	}
	return profile, nil
}

func (p *PersonProfile) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidPersonProfile)
	}
	if p.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidPersonProfile)
	}
	if p.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidPersonProfile)
	}
	if p.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidPersonProfile)
	}
	if strings.TrimSpace(p.FirstName) == "" {
		return fmt.Errorf("%w: first_name is required", ErrInvalidPersonProfile)
	}
	if strings.TrimSpace(p.LastName) == "" {
		return fmt.Errorf("%w: last_name is required", ErrInvalidPersonProfile)
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is required", ErrInvalidPersonProfile)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updated_at is required", ErrInvalidPersonProfile)
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidPersonProfile)
	}
	if p.DeletedAt != nil && p.DeletedAt.Before(p.CreatedAt) {
		return fmt.Errorf("%w: deleted_at cannot be before created_at", ErrInvalidPersonProfile)
	}
	return nil
}

func (p *PersonProfile) FullName() string {
	parts := make([]string, 0, 4)
	for _, part := range []string{p.FirstName, p.MiddleName, p.LastName, p.SecondLastName} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " ")
}

func (p *PersonProfile) SoftDelete(now time.Time) error {
	if p.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	p.DeletedAt = &now
	p.UpdatedAt = now
	return nil
}
