package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/identitydocumenttype"
	"enterprise-system/ent/partyidentitydocument"
	"enterprise-system/ent/partyidentitydocumentvalidity"
	"enterprise-system/ent/partynationality"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.IdentityRepository = (*IdentityRepository)(nil)

type IdentityRepository struct {
	client *ent.Client
}

func NewIdentityRepository(client *ent.Client) *IdentityRepository {
	return &IdentityRepository{client: client}
}

func (r *IdentityRepository) FindDocumentTypeByCode(ctx context.Context, code string) (*domain.IdentityDocumentType, error) {
	var result *domain.IdentityDocumentType
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.IdentityDocumentType.Query().
			Where(identitydocumenttype.CodeEQ(code)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.IdentityDocumentType{
			ID: row.ID, Code: row.Code, Name: row.Name, Scope: string(row.Scope),
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrIdentityDocumentTypeNotFound
	}
	return result, err
}

func (r *IdentityRepository) CreateNationality(ctx context.Context, nationality *domain.PartyNationality) error {
	if nationality == nil {
		return errors.New("repository: nil nationality")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		_, err := client.PartyNationality.Create().
			SetID(nationality.ID).
			SetTenantID(nationality.TenantID).
			SetPartyID(nationality.PartyID).
			SetCountryCode(nationality.CountryCode).
			SetIsPrimary(nationality.IsPrimary).
			SetCreatedAt(nationality.CreatedAt).
			SetUpdatedAt(nationality.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *IdentityRepository) ClearPrimaryNationality(ctx context.Context, tenantID, partyID uuid.UUID) error {
	return execute(ctx, r.client, func(client *ent.Client) error {
		_, err := client.PartyNationality.Update().
			Where(
				partynationality.TenantID(tenantID),
				partynationality.PartyID(partyID),
				partynationality.IsPrimary(true),
			).
			SetIsPrimary(false).
			SetUpdatedAt(time.Now().UTC()).
			Save(ctx)
		return repoError(err)
	})
}

func (r *IdentityRepository) FindIdentityDocument(
	ctx context.Context,
	tenantID, documentTypeID uuid.UUID,
	issuingCountryCode, documentNumber string,
) (*domain.PartyIdentityDocument, error) {
	var result *domain.PartyIdentityDocument
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyIdentityDocument.Query().
			Where(
				partyidentitydocument.TenantID(tenantID),
				partyidentitydocument.DocumentTypeID(documentTypeID),
				partyidentitydocument.IssuingCountryCode(issuingCountryCode),
				partyidentitydocument.DocumentNumberEQ(documentNumber),
				partyidentitydocument.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainIdentityDocument(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrIdentityDocumentNotFound
	}
	return result, err
}

func (r *IdentityRepository) GetIdentityDocument(ctx context.Context, tenantID, id uuid.UUID) (*domain.PartyIdentityDocument, error) {
	var result *domain.PartyIdentityDocument
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyIdentityDocument.Query().
			Where(
				partyidentitydocument.ID(id),
				partyidentitydocument.TenantID(tenantID),
				partyidentitydocument.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainIdentityDocument(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrIdentityDocumentNotFound
	}
	return result, err
}

func (r *IdentityRepository) CreateIdentityDocument(ctx context.Context, document *domain.PartyIdentityDocument) error {
	if document == nil {
		return errors.New("repository: nil identity document")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.PartyIdentityDocument.Create().
			SetID(document.ID).
			SetTenantID(document.TenantID).
			SetPartyID(document.PartyID).
			SetDocumentTypeID(document.DocumentTypeID).
			SetIssuingCountryCode(document.IssuingCountryCode).
			SetDocumentNumber(document.DocumentNumber).
			SetCreatedAt(document.CreatedAt).
			SetUpdatedAt(document.UpdatedAt)
		if document.IssuingJurisdictionID != nil {
			create.SetIssuingJurisdictionID(*document.IssuingJurisdictionID)
		}
		if document.DeletedAt != nil {
			create.SetDeletedAt(*document.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *IdentityRepository) GetOpenDocumentValidity(
	ctx context.Context,
	tenantID, documentID uuid.UUID,
) (*domain.PartyIdentityDocumentValidity, error) {
	var result *domain.PartyIdentityDocumentValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyIdentityDocumentValidity.Query().
			Where(
				partyidentitydocumentvalidity.TenantID(tenantID),
				partyidentitydocumentvalidity.PartyIdentityDocumentID(documentID),
				partyidentitydocumentvalidity.EndDateIsNil(),
			).
			Order(ent.Desc(partyidentitydocumentvalidity.FieldStartDate)).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainIdentityDocumentValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrIdentityDocumentNotFound
	}
	return result, err
}

func (r *IdentityRepository) GetOpenPrimaryDocumentValidity(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
) (*domain.PartyIdentityDocumentValidity, error) {
	var result *domain.PartyIdentityDocumentValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyIdentityDocumentValidity.Query().
			Where(
				partyidentitydocumentvalidity.TenantID(tenantID),
				partyidentitydocumentvalidity.PartyID(partyID),
				partyidentitydocumentvalidity.IsPrimary(true),
				partyidentitydocumentvalidity.EndDateIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainIdentityDocumentValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrIdentityDocumentNotFound
	}
	return result, err
}

func (r *IdentityRepository) CreateDocumentValidity(
	ctx context.Context,
	validity *domain.PartyIdentityDocumentValidity,
) error {
	if validity == nil {
		return errors.New("repository: nil identity document validity")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.PartyIdentityDocumentValidity.Create().
			SetID(validity.ID).
			SetTenantID(validity.TenantID).
			SetPartyIdentityDocumentID(validity.PartyIdentityDocumentID).
			SetPartyID(validity.PartyID).
			SetIsPrimary(validity.IsPrimary).
			SetStartDate(validity.StartDate).
			SetCreatedAt(validity.CreatedAt).
			SetUpdatedAt(validity.UpdatedAt)
		if validity.EndDate != nil {
			create.SetEndDate(*validity.EndDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *IdentityRepository) CloseDocumentValidity(
	ctx context.Context,
	tenantID, validityID uuid.UUID,
	at time.Time,
) error {
	var affected int
	err := execute(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.PartyIdentityDocumentValidity.Update().
			Where(
				partyidentitydocumentvalidity.ID(validityID),
				partyidentitydocumentvalidity.TenantID(tenantID),
				partyidentitydocumentvalidity.EndDateIsNil(),
			).
			SetEndDate(at).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrIdentityDocumentNotFound
	}
	return nil
}

func toDomainIdentityDocument(e *ent.PartyIdentityDocument) *domain.PartyIdentityDocument {
	return &domain.PartyIdentityDocument{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		DocumentTypeID: e.DocumentTypeID, IssuingCountryCode: e.IssuingCountryCode,
		IssuingJurisdictionID: e.IssuingJurisdictionID, DocumentNumber: e.DocumentNumber,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainIdentityDocumentValidity(e *ent.PartyIdentityDocumentValidity) *domain.PartyIdentityDocumentValidity {
	return &domain.PartyIdentityDocumentValidity{
		ID: e.ID, TenantID: e.TenantID, PartyIdentityDocumentID: e.PartyIdentityDocumentID,
		PartyID: e.PartyID, IsPrimary: e.IsPrimary,
		StartDate: e.StartDate, EndDate: e.EndDate,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
