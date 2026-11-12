package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// UploadDocumentInput describes one document upload. Reader is consumed once;
// the use case buffers it to a temporary file to compute the hash before
// deciding whether the content must be written to storage.
type UploadDocumentInput struct {
	TenantID     uuid.UUID
	OriginTable  string
	OriginID     uuid.UUID
	OriginalName string
	MimeType     string
	Description  string
	Reader       io.Reader
}

// DocumentService implements the secure hybrid storage use case (CAS +
// relational metadata).
type DocumentService struct {
	storage   domain.FileStorage
	documents domain.AttachedDocumentRepository
	tx        TransactionManager
	provider  string
	now       func() time.Time
}

func NewDocumentService(
	storage domain.FileStorage,
	documents domain.AttachedDocumentRepository,
	tx TransactionManager,
	provider string,
	now func() time.Time,
) *DocumentService {
	if provider == "" {
		provider = "local"
	}
	if now == nil {
		now = time.Now
	}
	return &DocumentService{
		storage: storage, documents: documents, tx: tx,
		provider: provider, now: now,
	}
}

// Upload validates the input, streams it through SHA-256, deduplicates by
// content hash inside the tenant, and persists the metadata.
func (s *DocumentService) Upload(ctx context.Context, in UploadDocumentInput) (*domain.AttachedDocument, error) {
	if s == nil || s.storage == nil || s.documents == nil || s.tx == nil {
		return nil, internalError(errors.New("documents: nil dependencies"))
	}
	name, err := sanitizeOriginalName(in.OriginalName)
	if err != nil {
		return nil, validationError("invalid_document_name", "invalid file name", err)
	}
	if in.TenantID == uuid.Nil {
		return nil, validationError("invalid_tenant", "tenant is required", nil)
	}
	if in.OriginID == uuid.Nil {
		return nil, validationError("invalid_origin", "origin_id is required", nil)
	}
	originTable := strings.TrimSpace(in.OriginTable)
	if originTable == "" {
		return nil, validationError("invalid_origin", "origin_table is required", nil)
	}
	if in.Reader == nil {
		return nil, validationError("invalid_document", "file content is required", nil)
	}
	mimeType := strings.TrimSpace(in.MimeType)
	if mimeType == "" {
		return nil, validationError("invalid_document", "mime_type is required", nil)
	}

	size, hash, temp, err := bufferDocument(in.Reader)
	if err != nil {
		if errors.Is(err, errEmptyDocument) {
			return nil, validationError("empty_document", "document is empty", err)
		}
		return nil, internalError(fmt.Errorf("buffer document: %w", err))
	}
	defer func() {
		temp.Close()
		os.Remove(temp.Name())
	}()

	existing, err := readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.AttachedDocument, error) {
		document, findErr := s.documents.FindByHash(ctx, in.TenantID, hash)
		if errors.Is(findErr, domain.ErrAttachedDocumentNotFound) {
			return nil, nil
		}
		return document, findErr
	})
	if err != nil {
		return nil, fmt.Errorf("find document by hash: %w", err)
	}

	storageKey := hex.EncodeToString(hash)
	if existing != nil {
		// Same tenant + same content: reuse the existing object and never write
		// it to disk again.
		storageKey = existing.StorageKey
	}

	exists, err := s.storage.Exists(ctx, storageKey)
	if err != nil {
		return nil, internalError(fmt.Errorf("check document storage: %w", err))
	}
	if !exists {
		if _, err := temp.Seek(0, io.SeekStart); err != nil {
			return nil, internalError(fmt.Errorf("rewind document: %w", err))
		}
		if err := s.storage.Upload(ctx, storageKey, temp); err != nil {
			return nil, internalError(fmt.Errorf("store document: %w", err))
		}
	}

	now := s.now().UTC()
	documentID, err := uuid.NewV7()
	if err != nil {
		return nil, internalError(fmt.Errorf("generate document id: %w", err))
	}
	document := &domain.AttachedDocument{
		ID:              documentID,
		TenantID:        in.TenantID,
		OriginTable:     originTable,
		OriginID:        in.OriginID,
		OriginalName:    name,
		StorageKey:      storageKey,
		StorageProvider: s.provider,
		MimeType:        mimeType,
		SizeBytes:       size,
		DocumentHash:    hash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if description := strings.TrimSpace(in.Description); description != "" {
		document.Description = &description
	}

	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.documents.Create(ctx, document)
	}); err != nil {
		return nil, fmt.Errorf("create attached document: %w", err)
	}
	return document, nil
}

var errEmptyDocument = errors.New("empty document")

// bufferDocument copies the stream to a temporary file while hashing it with
// io.TeeReader. The file is rewound for a later storage upload.
func bufferDocument(reader io.Reader) (int64, []byte, *os.File, error) {
	temp, err := os.CreateTemp("", "document-upload-*")
	if err != nil {
		return 0, nil, nil, fmt.Errorf("create temp file: %w", err)
	}
	hasher := sha256.New()
	size, err := io.Copy(temp, io.TeeReader(reader, hasher))
	if err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return 0, nil, nil, err
	}
	if size == 0 {
		temp.Close()
		os.Remove(temp.Name())
		return 0, nil, nil, errEmptyDocument
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return 0, nil, nil, err
	}
	return size, hasher.Sum(nil), temp, nil
}

func sanitizeOriginalName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", errors.New("empty file name")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", errors.New("file name must not contain path separators")
	}
	base := filepath.Base(name)
	if base == "." || base == ".." || base != name {
		return "", errors.New("invalid file name")
	}
	return base, nil
}
