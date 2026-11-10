package domain

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

type ContactType string

const (
	ContactTypeEmail   ContactType = "EMAIL"
	ContactTypePhone   ContactType = "PHONE"
	ContactTypeMobile  ContactType = "MOBILE"
	ContactTypeFax     ContactType = "FAX"
	ContactTypeWebsite ContactType = "WEBSITE"
)

func (t ContactType) Valid() bool {
	switch t {
	case ContactTypeEmail, ContactTypePhone, ContactTypeMobile, ContactTypeFax, ContactTypeWebsite:
		return true
	default:
		return false
	}
}

type ContactMechanism struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	Type       ContactType
	Value      string
	IsPrimary  bool
	IsVerified bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func NewContactMechanism(
	tenantID, partyID uuid.UUID,
	contactType ContactType,
	value string,
	isPrimary, isVerified bool,
	now time.Time,
) (*ContactMechanism, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate contact mechanism id: %w", err)
	}

	now = now.UTC()
	contact := &ContactMechanism{
		ID:         id,
		TenantID:   tenantID,
		PartyID:    partyID,
		Type:       contactType,
		Value:      strings.TrimSpace(value),
		IsPrimary:  isPrimary,
		IsVerified: isVerified,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := contact.Validate(); err != nil {
		return nil, err
	}
	return contact, nil
}

func (c *ContactMechanism) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidContactMechanism)
	}
	if c.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidContactMechanism)
	}
	if c.TenantID == uuid.Nil {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidContactMechanism)
	}
	if c.PartyID == uuid.Nil {
		return fmt.Errorf("%w: party_id is required", ErrInvalidContactMechanism)
	}
	if !c.Type.Valid() {
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidContactMechanism, c.Type)
	}
	if strings.TrimSpace(c.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidContactMechanism)
	}
	if err := validateContactValue(c.Type, c.Value); err != nil {
		return err
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidContactMechanism)
	}
	if c.UpdatedAt.Before(c.CreatedAt) {
		return fmt.Errorf("%w: updated_at cannot be before created_at", ErrInvalidContactMechanism)
	}
	return nil
}

func validateContactValue(contactType ContactType, value string) error {
	switch contactType {
	case ContactTypeEmail:
		addr, err := mail.ParseAddress(value)
		if err != nil || addr.Address != value {
			return fmt.Errorf("%w: invalid email %q", ErrInvalidContactMechanism, value)
		}
	case ContactTypePhone, ContactTypeMobile, ContactTypeFax:
		digits := 0
		for _, r := range value {
			switch {
			case unicode.IsDigit(r):
				digits++
			case r == '+', r == '-', r == ' ', r == '(', r == ')', r == '.':
			default:
				return fmt.Errorf("%w: invalid phone %q", ErrInvalidContactMechanism, value)
			}
		}
		if digits < 7 {
			return fmt.Errorf("%w: phone must have at least 7 digits", ErrInvalidContactMechanism)
		}
	case ContactTypeWebsite:
		u, err := url.ParseRequestURI(value)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("%w: invalid website %q", ErrInvalidContactMechanism, value)
		}
	}
	return nil
}

func (c *ContactMechanism) Update(contactType ContactType, value string, isPrimary, isVerified bool, now time.Time) error {
	if c.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	c.Type = contactType
	c.Value = strings.TrimSpace(value)
	c.IsPrimary = isPrimary
	c.IsVerified = isVerified
	c.UpdatedAt = now.UTC()
	return c.Validate()
}

func (c *ContactMechanism) SetPrimary(isPrimary bool, now time.Time) error {
	if c.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	c.IsPrimary = isPrimary
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ContactMechanism) Verify(now time.Time) error {
	if c.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	c.IsVerified = true
	c.UpdatedAt = now.UTC()
	return nil
}

func (c *ContactMechanism) SoftDelete(now time.Time) error {
	if c.DeletedAt != nil {
		return ErrAlreadyDeleted
	}
	now = now.UTC()
	c.DeletedAt = &now
	c.UpdatedAt = now
	return nil
}
