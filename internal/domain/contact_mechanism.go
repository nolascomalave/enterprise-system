package domain

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
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

// ContactMechanism is global master data: a unique (type, value) pair shared by
// parties across tenants. Tenant-specific lifecycle lives in
// PartyContactMechanism.
type ContactMechanism struct {
	ID    uuid.UUID
	Type  ContactType
	Value string
}

func NewContactMechanism(contactType ContactType, value string) (*ContactMechanism, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate contact mechanism id: %w", err)
	}
	contact := &ContactMechanism{
		ID:    id,
		Type:  contactType,
		Value: strings.TrimSpace(value),
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
	if !c.Type.Valid() {
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidContactMechanism, c.Type)
	}
	if strings.TrimSpace(c.Value) == "" {
		return fmt.Errorf("%w: value is required", ErrInvalidContactMechanism)
	}
	if err := validateContactValue(c.Type, c.Value); err != nil {
		return err
	}
	return nil
}

func validateContactValue(contactType ContactType, value string) error {
	switch contactType {
	case ContactTypeEmail:
		addr, err := mail.ParseAddress(value)
		if err != nil || !strings.EqualFold(addr.Address, value) {
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

func (c *ContactMechanism) Update(contactType ContactType, value string) error {
	if c == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidContactMechanism)
	}
	c.Type = contactType
	c.Value = strings.TrimSpace(value)
	return c.Validate()
}
