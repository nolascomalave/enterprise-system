package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type AddPartyNationalityInput struct {
	TenantID    uuid.UUID
	PartyID     uuid.UUID
	CountryCode string
	IsPrimary   bool
}

type AddPartyIdentityDocumentInput struct {
	TenantID              uuid.UUID
	PartyID               uuid.UUID
	DocumentType          string
	IssuingCountryCode    string
	IssuingJurisdictionID *uuid.UUID
	DocumentNumber        string
	IsPrimary             bool
}

type UpdateIdentityDocumentValidityInput struct {
	TenantID   uuid.UUID
	PartyID    uuid.UUID
	DocumentID uuid.UUID
	IsPrimary  bool
}

type IdentityDocumentResult struct {
	Document *domain.PartyIdentityDocument
	Validity *domain.PartyIdentityDocumentValidity
	TypeCode string
}

type IdentityService struct {
	repo      domain.IdentityRepository
	locations domain.LocationRepository
	tx        TransactionManager
	now       func() time.Time
}

func NewIdentityService(
	repo domain.IdentityRepository,
	locations domain.LocationRepository,
	tx TransactionManager,
	now func() time.Time,
) *IdentityService {
	if now == nil {
		now = time.Now
	}
	return &IdentityService{repo: repo, locations: locations, tx: tx, now: now}
}

func (s *IdentityService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.repo == nil || s.locations == nil || s.tx == nil {
		return errors.New("identity: nil dependencies")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

func (s *IdentityService) AddNationality(ctx context.Context, in AddPartyNationalityInput) (*domain.PartyNationality, error) {
	nationality, err := domain.NewPartyNationality(in.TenantID, in.PartyID, in.CountryCode, in.IsPrimary, s.now())
	if err != nil {
		return nil, validationError("invalid_nationality", "invalid nationality", err)
	}
	if err := s.withinTx(ctx, func(ctx context.Context) error {
		if nationality.IsPrimary {
			if err := s.repo.ClearPrimaryNationality(ctx, in.TenantID, in.PartyID); err != nil {
				return fmt.Errorf("clear primary nationality: %w", err)
			}
		}
		if err := s.repo.CreateNationality(ctx, nationality); err != nil {
			return fmt.Errorf("create nationality: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return nationality, nil
}

// AddIdentityDocument creates the document master row and opens its initial
// validity window.
func (s *IdentityService) AddIdentityDocument(ctx context.Context, in AddPartyIdentityDocumentInput) (*IdentityDocumentResult, error) {
	code := domain.IdentityDocumentTypeCode(strings.ToUpper(strings.TrimSpace(in.DocumentType)))
	if !code.Valid() {
		return nil, validationError("invalid_identity_document", "invalid document_type", domain.ErrInvalidIdentityDocument)
	}
	number := strings.TrimSpace(in.DocumentNumber)
	if number == "" {
		return nil, validationError("invalid_identity_document", "document_number is required", domain.ErrInvalidIdentityDocument)
	}

	var result *IdentityDocumentResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		documentType, err := s.repo.FindDocumentTypeByCode(ctx, string(code))
		if err != nil {
			return fmt.Errorf("find document type: %w", err)
		}
		country := strings.ToUpper(strings.TrimSpace(in.IssuingCountryCode))
		if err := s.validateJurisdiction(ctx, in.IssuingJurisdictionID, country); err != nil {
			return err
		}

		if _, err := s.repo.FindIdentityDocument(ctx, in.TenantID, documentType.ID, country, number); err == nil {
			return domain.ErrIdentityDocumentExists
		} else if !errors.Is(err, domain.ErrIdentityDocumentNotFound) {
			return fmt.Errorf("find identity document: %w", err)
		}

		document, err := domain.NewPartyIdentityDocument(
			in.TenantID, in.PartyID, documentType.ID, country,
			in.IssuingJurisdictionID, number, s.now(),
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateIdentityDocument(ctx, document); err != nil {
			return fmt.Errorf("create identity document: %w", err)
		}
		if in.IsPrimary {
			if err := s.closeOpenPrimaryDocument(ctx, in.TenantID, in.PartyID, document.ID, s.now()); err != nil {
				return err
			}
		}
		validity, err := domain.NewPartyIdentityDocumentValidity(
			in.TenantID, document.ID, in.PartyID, in.IsPrimary, s.now(), nil, s.now(),
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDocumentValidity(ctx, validity); err != nil {
			return fmt.Errorf("create identity document validity: %w", err)
		}
		result = &IdentityDocumentResult{Document: document, Validity: validity, TypeCode: documentType.Code}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateValidity closes the current open window and opens a new one (renewal
// or any other change that keeps the same document identity).
func (s *IdentityService) UpdateValidity(ctx context.Context, in UpdateIdentityDocumentValidityInput) (*IdentityDocumentResult, error) {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil || in.DocumentID == uuid.Nil {
		return nil, validationError("invalid_identity_document", "ids are required", domain.ErrInvalidIdentityDocument)
	}
	var result *IdentityDocumentResult
	err := s.withinTx(ctx, func(ctx context.Context) error {
		document, err := s.repo.GetIdentityDocument(ctx, in.TenantID, in.DocumentID)
		if err != nil {
			return fmt.Errorf("get identity document: %w", err)
		}
		if document.PartyID != in.PartyID {
			return domain.ErrIdentityDocumentNotFound
		}
		now := s.now()

		if open, err := s.repo.GetOpenDocumentValidity(ctx, in.TenantID, document.ID); err == nil {
			if err := s.repo.CloseDocumentValidity(ctx, in.TenantID, open.ID, now); err != nil {
				return fmt.Errorf("close current validity: %w", err)
			}
		} else if !errors.Is(err, domain.ErrIdentityDocumentNotFound) {
			return fmt.Errorf("get current validity: %w", err)
		}

		if in.IsPrimary {
			if err := s.closeOpenPrimaryDocument(ctx, in.TenantID, in.PartyID, document.ID, now); err != nil {
				return err
			}
		}

		validity, err := domain.NewPartyIdentityDocumentValidity(
			in.TenantID, document.ID, in.PartyID, in.IsPrimary, now, nil, now,
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateDocumentValidity(ctx, validity); err != nil {
			return fmt.Errorf("create identity document validity: %w", err)
		}
		result = &IdentityDocumentResult{Document: document, Validity: validity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *IdentityService) validateJurisdiction(ctx context.Context, jurisdictionID *uuid.UUID, countryCode string) error {
	if jurisdictionID == nil {
		return nil
	}
	location, err := s.locations.GetLocation(ctx, *jurisdictionID)
	if err != nil {
		return fmt.Errorf("get issuing jurisdiction: %w", err)
	}
	if location.CountryCode != countryCode {
		return domain.ErrJurisdictionCountryMismatch
	}
	return nil
}

func (s *IdentityService) closeOpenPrimaryDocument(
	ctx context.Context,
	tenantID, partyID, excludeDocumentID uuid.UUID,
	at time.Time,
) error {
	open, err := s.repo.GetOpenPrimaryDocumentValidity(ctx, tenantID, partyID)
	if errors.Is(err, domain.ErrIdentityDocumentNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get open primary document: %w", err)
	}
	if open.PartyIdentityDocumentID == excludeDocumentID {
		return nil
	}
	if err := s.repo.CloseDocumentValidity(ctx, tenantID, open.ID, at); err != nil {
		return fmt.Errorf("close open primary document: %w", err)
	}
	return nil
}
