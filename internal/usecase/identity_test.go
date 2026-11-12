package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIdentityRepo struct {
	types         map[string]*domain.IdentityDocumentType
	nationalities map[uuid.UUID]*domain.PartyNationality
	documents     map[uuid.UUID]*domain.PartyIdentityDocument
	validities    map[uuid.UUID]*domain.PartyIdentityDocumentValidity
	closed        int
}

func newFakeIdentityRepo() *fakeIdentityRepo {
	repo := &fakeIdentityRepo{
		types:         make(map[string]*domain.IdentityDocumentType),
		nationalities: make(map[uuid.UUID]*domain.PartyNationality),
		documents:     make(map[uuid.UUID]*domain.PartyIdentityDocument),
		validities:    make(map[uuid.UUID]*domain.PartyIdentityDocumentValidity),
	}
	for _, code := range []domain.IdentityDocumentTypeCode{
		domain.IdentityDocumentTypePassport, domain.IdentityDocumentTypeNationalID,
		domain.IdentityDocumentTypeDriverLicense, domain.IdentityDocumentTypeEUCitizenID,
		domain.IdentityDocumentTypeBirthCertificate,
	} {
		repo.types[string(code)] = &domain.IdentityDocumentType{ID: uuid.New(), Code: string(code), Scope: "PERSON"}
	}
	return repo
}

func (r *fakeIdentityRepo) FindDocumentTypeByCode(ctx context.Context, code string) (*domain.IdentityDocumentType, error) {
	documentType, ok := r.types[code]
	if !ok {
		return nil, domain.ErrIdentityDocumentTypeNotFound
	}
	return documentType, nil
}

func (r *fakeIdentityRepo) CreateNationality(ctx context.Context, nationality *domain.PartyNationality) error {
	r.nationalities[nationality.ID] = nationality
	return nil
}

func (r *fakeIdentityRepo) ClearPrimaryNationality(ctx context.Context, tenantID, partyID uuid.UUID) error {
	for _, nationality := range r.nationalities {
		if nationality.TenantID == tenantID && nationality.PartyID == partyID {
			nationality.IsPrimary = false
		}
	}
	return nil
}

func (r *fakeIdentityRepo) FindIdentityDocument(ctx context.Context, tenantID, documentTypeID uuid.UUID, issuingCountryCode, documentNumber string) (*domain.PartyIdentityDocument, error) {
	for _, document := range r.documents {
		if document.TenantID == tenantID && document.DocumentTypeID == documentTypeID &&
			document.IssuingCountryCode == issuingCountryCode &&
			strings.EqualFold(document.DocumentNumber, documentNumber) {
			return document, nil
		}
	}
	return nil, domain.ErrIdentityDocumentNotFound
}

func (r *fakeIdentityRepo) GetIdentityDocument(ctx context.Context, tenantID, id uuid.UUID) (*domain.PartyIdentityDocument, error) {
	document, ok := r.documents[id]
	if !ok || document.TenantID != tenantID {
		return nil, domain.ErrIdentityDocumentNotFound
	}
	return document, nil
}

func (r *fakeIdentityRepo) CreateIdentityDocument(ctx context.Context, document *domain.PartyIdentityDocument) error {
	r.documents[document.ID] = document
	return nil
}

func (r *fakeIdentityRepo) GetOpenDocumentValidity(ctx context.Context, tenantID, documentID uuid.UUID) (*domain.PartyIdentityDocumentValidity, error) {
	for _, validity := range r.validities {
		if validity.TenantID == tenantID && validity.PartyIdentityDocumentID == documentID && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrIdentityDocumentNotFound
}

func (r *fakeIdentityRepo) GetOpenPrimaryDocumentValidity(ctx context.Context, tenantID, partyID uuid.UUID) (*domain.PartyIdentityDocumentValidity, error) {
	for _, validity := range r.validities {
		if validity.TenantID == tenantID && validity.PartyID == partyID && validity.IsPrimary && validity.EndDate == nil {
			return validity, nil
		}
	}
	return nil, domain.ErrIdentityDocumentNotFound
}

func (r *fakeIdentityRepo) CreateDocumentValidity(ctx context.Context, validity *domain.PartyIdentityDocumentValidity) error {
	r.validities[validity.ID] = validity
	return nil
}

func (r *fakeIdentityRepo) CloseDocumentValidity(ctx context.Context, tenantID, validityID uuid.UUID, at time.Time) error {
	validity, ok := r.validities[validityID]
	if !ok || validity.TenantID != tenantID {
		return domain.ErrIdentityDocumentNotFound
	}
	if err := validity.Close(at); err != nil {
		return err
	}
	r.closed++
	return nil
}

func identityTestService(t *testing.T) (*IdentityService, *fakeIdentityRepo, *fakeLocationRepo) {
	t.Helper()
	repo := newFakeIdentityRepo()
	locations := newFakeLocationRepo()
	service := NewIdentityService(repo, locations, newFakeTxManager(),
		func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) })
	return service, repo, locations
}

func TestAddPartyNationality(t *testing.T) {
	service, repo, _ := identityTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()

	first, err := service.AddNationality(context.Background(), AddPartyNationalityInput{
		TenantID: tenantID, PartyID: partyID, CountryCode: "ve", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "VE", first.CountryCode)
	assert.True(t, first.IsPrimary)

	second, err := service.AddNationality(context.Background(), AddPartyNationalityInput{
		TenantID: tenantID, PartyID: partyID, CountryCode: "co", IsPrimary: true,
	})
	require.NoError(t, err)
	assert.True(t, second.IsPrimary)
	assert.False(t, first.IsPrimary, "the previous primary nationality must be cleared")
	assert.Len(t, repo.nationalities, 2)
}

func TestAddPartyNationalityRejectsInvalidCountry(t *testing.T) {
	service, _, _ := identityTestService(t)

	_, err := service.AddNationality(context.Background(), AddPartyNationalityInput{
		TenantID: uuid.New(), PartyID: uuid.New(), CountryCode: "VENEZUELA",
	})

	require.ErrorIs(t, err, domain.ErrInvalidNationality)
}

func TestAddPartyIdentityDocument(t *testing.T) {
	service, repo, locations := identityTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()
	jurisdiction, err := domain.NewLocation(domain.LocationTypeState, nil, "VE", "", "", "Miranda", "world.ve.miranda", time.Now())
	require.NoError(t, err)
	locations.locations[jurisdiction.ID] = jurisdiction

	result, err := service.AddIdentityDocument(context.Background(), AddPartyIdentityDocumentInput{
		TenantID: tenantID, PartyID: partyID, DocumentType: "PASSPORT",
		IssuingCountryCode: "VE", IssuingJurisdictionID: &jurisdiction.ID,
		DocumentNumber: " 12345678 ", IsPrimary: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "12345678", result.Document.DocumentNumber)
	assert.Equal(t, repo.types["PASSPORT"].ID, result.Document.DocumentTypeID)
	assert.Equal(t, "PASSPORT", result.TypeCode)
	assert.True(t, result.Validity.IsPrimary)
	assert.Nil(t, result.Validity.EndDate)
	assert.Len(t, repo.documents, 1)
	assert.Len(t, repo.validities, 1)
}

func TestAddPartyIdentityDocumentRejectsJurisdictionMismatch(t *testing.T) {
	service, repo, locations := identityTestService(t)
	jurisdiction, err := domain.NewLocation(domain.LocationTypeState, nil, "CO", "", "", "Cundinamarca", "world.co.cundinamarca", time.Now())
	require.NoError(t, err)
	locations.locations[jurisdiction.ID] = jurisdiction

	_, err = service.AddIdentityDocument(context.Background(), AddPartyIdentityDocumentInput{
		TenantID: uuid.New(), PartyID: uuid.New(), DocumentType: "PASSPORT",
		IssuingCountryCode: "VE", IssuingJurisdictionID: &jurisdiction.ID,
		DocumentNumber: "12345678",
	})

	require.ErrorIs(t, err, domain.ErrJurisdictionCountryMismatch)
	assert.Empty(t, repo.documents)
}

func TestAddPartyIdentityDocumentRejectsDuplicate(t *testing.T) {
	service, _, _ := identityTestService(t)
	input := AddPartyIdentityDocumentInput{
		TenantID: uuid.New(), PartyID: uuid.New(), DocumentType: "NATIONAL_ID",
		IssuingCountryCode: "VE", DocumentNumber: "V-123",
	}
	_, err := service.AddIdentityDocument(context.Background(), input)
	require.NoError(t, err)

	_, err = service.AddIdentityDocument(context.Background(), input)
	require.ErrorIs(t, err, domain.ErrIdentityDocumentExists)
}

func TestUpdateIdentityDocumentValidity(t *testing.T) {
	service, repo, _ := identityTestService(t)
	tenantID, partyID := uuid.New(), uuid.New()
	created, err := service.AddIdentityDocument(context.Background(), AddPartyIdentityDocumentInput{
		TenantID: tenantID, PartyID: partyID, DocumentType: "PASSPORT",
		IssuingCountryCode: "VE", DocumentNumber: "123", IsPrimary: true,
	})
	require.NoError(t, err)

	updated, err := service.UpdateValidity(context.Background(), UpdateIdentityDocumentValidityInput{
		TenantID: tenantID, PartyID: partyID, DocumentID: created.Document.ID, IsPrimary: true,
	})
	require.NoError(t, err)

	assert.NotNil(t, created.Validity.EndDate)
	assert.NotEqual(t, created.Validity.ID, updated.Validity.ID)
	assert.Nil(t, updated.Validity.EndDate)
	assert.Equal(t, created.Document.ID, updated.Document.ID)
	assert.Equal(t, 1, repo.closed)
}

func (r *fakeIdentityRepo) ListDocumentTypes(ctx context.Context) ([]*domain.IdentityDocumentType, error) {
	result := make([]*domain.IdentityDocumentType, 0, len(r.types))
	for _, documentType := range r.types {
		result = append(result, documentType)
	}
	return result, nil
}

func (r *fakeIdentityRepo) GetDocumentTypeTranslation(ctx context.Context, documentTypeID uuid.UUID, locale string) (string, error) {
	return "", domain.ErrTranslationNotFound
}
