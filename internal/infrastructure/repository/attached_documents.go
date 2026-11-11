package repository

import (
	"context"
	"errors"

	"enterprise-system/ent"
	"enterprise-system/ent/attacheddocument"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.AttachedDocumentRepository = (*AttachedDocumentRepository)(nil)

type AttachedDocumentRepository struct {
	client *ent.Client
}

func NewAttachedDocumentRepository(client *ent.Client) *AttachedDocumentRepository {
	return &AttachedDocumentRepository{client: client}
}

func (r *AttachedDocumentRepository) Create(ctx context.Context, document *domain.AttachedDocument) error {
	if document == nil {
		return errors.New("repository: nil attached document")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.AttachedDocument.Create().
			SetID(document.ID).
			SetTenantID(document.TenantID).
			SetOriginTable(document.OriginTable).
			SetOriginID(document.OriginID).
			SetOriginalName(document.OriginalName).
			SetStorageKey(document.StorageKey).
			SetStorageProvider(document.StorageProvider).
			SetMimeType(document.MimeType).
			SetSizeBytes(document.SizeBytes).
			SetDocumentHash(document.DocumentHash).
			SetCreatedAt(document.CreatedAt).
			SetUpdatedAt(document.UpdatedAt)
		if document.Description != nil {
			create.SetDescription(*document.Description)
		}
		if document.DeletedAt != nil {
			create.SetDeletedAt(*document.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *AttachedDocumentRepository) FindByHash(
	ctx context.Context,
	tenantID uuid.UUID,
	hash []byte,
) (*domain.AttachedDocument, error) {
	var result *domain.AttachedDocument
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.AttachedDocument.Query().
			Where(
				attacheddocument.TenantID(tenantID),
				attacheddocument.DocumentHashEQ(hash),
				attacheddocument.DeletedAtIsNil(),
			).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainAttachedDocument(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrAttachedDocumentNotFound
	}
	return result, err
}

func toDomainAttachedDocument(e *ent.AttachedDocument) *domain.AttachedDocument {
	return &domain.AttachedDocument{
		ID:              e.ID,
		TenantID:        e.TenantID,
		OriginTable:     e.OriginTable,
		OriginID:        e.OriginID,
		OriginalName:    e.OriginalName,
		StorageKey:      e.StorageKey,
		StorageProvider: e.StorageProvider,
		MimeType:        e.MimeType,
		SizeBytes:       e.SizeBytes,
		DocumentHash:    e.DocumentHash,
		Description:     e.Description,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
		DeletedAt:       e.DeletedAt,
	}
}
