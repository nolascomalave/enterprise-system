package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// AttachedDocument is the metadata of a file stored in a content-addressed
// storage. The file content is never kept in PostgreSQL.
type AttachedDocument struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	OriginTable     string
	OriginID        uuid.UUID
	OriginalName    string
	StorageKey      string
	StorageProvider string
	MimeType        string
	SizeBytes       int64
	DocumentHash    []byte
	Description     *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

// AttachedDocumentRepository persists document metadata inside the tenant
// scope (RLS applies).
type AttachedDocumentRepository interface {
	Create(ctx context.Context, document *AttachedDocument) error
	// FindByHash returns the active document for the tenant with the given
	// SHA-256 bytes, or ErrAttachedDocumentNotFound when there is none.
	FindByHash(ctx context.Context, tenantID uuid.UUID, hash []byte) (*AttachedDocument, error)
}

// FileStorage is the hexagonal port for content-addressed object storage.
// Keys are SHA-256 hex digests; streams are used to avoid loading whole files
// in memory.
type FileStorage interface {
	Upload(ctx context.Context, key string, content io.Reader) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
