package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type IdentityDocumentTypeCode string

const (
	IdentityDocumentTypePassport         IdentityDocumentTypeCode = "PASSPORT"
	IdentityDocumentTypeNationalID       IdentityDocumentTypeCode = "NATIONAL_ID"
	IdentityDocumentTypeDriverLicense    IdentityDocumentTypeCode = "DRIVER_LICENSE"
	IdentityDocumentTypeEUCitizenID      IdentityDocumentTypeCode = "EU_CITIZEN_ID"
	IdentityDocumentTypeBirthCertificate IdentityDocumentTypeCode = "BIRTH_CERTIFICATE"
)

func (c IdentityDocumentTypeCode) Valid() bool {
	switch c {
	case IdentityDocumentTypePassport, IdentityDocumentTypeNationalID,
		IdentityDocumentTypeDriverLicense, IdentityDocumentTypeEUCitizenID,
		IdentityDocumentTypeBirthCertificate:
		return true
	default:
		return false
	}
}

type IdentityDocumentType struct {
	ID    uuid.UUID
	Code  string
	Name  string
	Scope string
}

type PartyNationality struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	PartyID     uuid.UUID
	CountryCode string
	IsPrimary   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewPartyNationality(
	tenantID, partyID uuid.UUID,
	countryCode string,
	isPrimary bool,
	now time.Time,
) (*PartyNationality, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate nationality id: %w", err)
	}
	now = now.UTC()
	nationality := &PartyNationality{
		ID: id, TenantID: tenantID, PartyID: partyID,
		CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)),
		IsPrimary:   isPrimary, CreatedAt: now, UpdatedAt: now,
	}
	if err := nationality.Validate(); err != nil {
		return nil, err
	}
	return nationality, nil
}

func (n *PartyNationality) Validate() error {
	if n == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidNationality)
	}
	if n.ID == uuid.Nil || n.TenantID == uuid.Nil || n.PartyID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidNationality)
	}
	if !identityCountryPattern.MatchString(n.CountryCode) {
		return fmt.Errorf("%w: country_code must be ISO 3166-1 alpha-2", ErrInvalidNationality)
	}
	if n.CreatedAt.IsZero() || n.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidNationality)
	}
	return nil
}

type PartyIdentityDocument struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	PartyID               uuid.UUID
	DocumentTypeID        uuid.UUID
	IssuingCountryCode    string
	IssuingJurisdictionID *uuid.UUID
	DocumentNumber        string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             *time.Time
}

func NewPartyIdentityDocument(
	tenantID, partyID, documentTypeID uuid.UUID,
	issuingCountryCode string,
	issuingJurisdictionID *uuid.UUID,
	documentNumber string,
	now time.Time,
) (*PartyIdentityDocument, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate identity document id: %w", err)
	}
	now = now.UTC()
	document := &PartyIdentityDocument{
		ID: id, TenantID: tenantID, PartyID: partyID, DocumentTypeID: documentTypeID,
		IssuingCountryCode:    strings.ToUpper(strings.TrimSpace(issuingCountryCode)),
		IssuingJurisdictionID: issuingJurisdictionID,
		DocumentNumber:        strings.TrimSpace(documentNumber),
		CreatedAt:             now, UpdatedAt: now,
	}
	if err := document.Validate(); err != nil {
		return nil, err
	}
	return document, nil
}

func (d *PartyIdentityDocument) Validate() error {
	if d == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidIdentityDocument)
	}
	if d.ID == uuid.Nil || d.TenantID == uuid.Nil || d.PartyID == uuid.Nil || d.DocumentTypeID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidIdentityDocument)
	}
	if !identityCountryPattern.MatchString(d.IssuingCountryCode) {
		return fmt.Errorf("%w: issuing_country_code must be ISO 3166-1 alpha-2", ErrInvalidIdentityDocument)
	}
	if d.IssuingJurisdictionID != nil && *d.IssuingJurisdictionID == uuid.Nil {
		return fmt.Errorf("%w: invalid issuing_jurisdiction_id", ErrInvalidIdentityDocument)
	}
	if strings.TrimSpace(d.DocumentNumber) == "" {
		return fmt.Errorf("%w: document_number is required", ErrInvalidIdentityDocument)
	}
	if d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidIdentityDocument)
	}
	return nil
}

type PartyIdentityDocumentValidity struct {
	ID                      uuid.UUID
	TenantID                uuid.UUID
	PartyIdentityDocumentID uuid.UUID
	PartyID                 uuid.UUID
	IsPrimary               bool
	StartDate               time.Time
	EndDate                 *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func NewPartyIdentityDocumentValidity(
	tenantID, documentID, partyID uuid.UUID,
	isPrimary bool,
	startDate time.Time,
	endDate *time.Time,
	now time.Time,
) (*PartyIdentityDocumentValidity, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate identity document validity id: %w", err)
	}
	now = now.UTC()
	validity := &PartyIdentityDocumentValidity{
		ID: id, TenantID: tenantID, PartyIdentityDocumentID: documentID,
		PartyID: partyID, IsPrimary: isPrimary,
		StartDate: startDate.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if endDate != nil {
		end := endDate.UTC()
		validity.EndDate = &end
	}
	if err := validity.Validate(); err != nil {
		return nil, err
	}
	return validity, nil
}

func (v *PartyIdentityDocumentValidity) Validate() error {
	if v == nil {
		return fmt.Errorf("%w: nil validity", ErrInvalidIdentityDocument)
	}
	if v.ID == uuid.Nil || v.TenantID == uuid.Nil ||
		v.PartyIdentityDocumentID == uuid.Nil || v.PartyID == uuid.Nil {
		return fmt.Errorf("%w: ids are required", ErrInvalidIdentityDocument)
	}
	if v.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required", ErrInvalidIdentityDocument)
	}
	if v.EndDate != nil && !v.EndDate.After(v.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidIdentityDocument)
	}
	return nil
}

func (v *PartyIdentityDocumentValidity) Close(at time.Time) error {
	if v.EndDate != nil {
		return ErrAlreadyDeleted
	}
	at = at.UTC()
	if at.Before(v.StartDate) {
		return fmt.Errorf("%w: end_date cannot be before start_date", ErrInvalidIdentityDocument)
	}
	v.EndDate = &at
	v.UpdatedAt = at
	return nil
}

var identityCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// IdentityRepository persists nationalities and identity document metadata.
type IdentityRepository interface {
	FindDocumentTypeByCode(ctx context.Context, code string) (*IdentityDocumentType, error)

	CreateNationality(ctx context.Context, nationality *PartyNationality) error
	ClearPrimaryNationality(ctx context.Context, tenantID, partyID uuid.UUID) error

	FindIdentityDocument(ctx context.Context, tenantID, documentTypeID uuid.UUID, issuingCountryCode, documentNumber string) (*PartyIdentityDocument, error)
	GetIdentityDocument(ctx context.Context, tenantID, id uuid.UUID) (*PartyIdentityDocument, error)
	CreateIdentityDocument(ctx context.Context, document *PartyIdentityDocument) error

	GetOpenDocumentValidity(ctx context.Context, tenantID, documentID uuid.UUID) (*PartyIdentityDocumentValidity, error)
	GetOpenPrimaryDocumentValidity(ctx context.Context, tenantID, partyID uuid.UUID) (*PartyIdentityDocumentValidity, error)
	CreateDocumentValidity(ctx context.Context, validity *PartyIdentityDocumentValidity) error
	CloseDocumentValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error
}
