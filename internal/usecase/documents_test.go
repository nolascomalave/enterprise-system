package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"enterprise-system/internal/common/apperror"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/storage/local"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAttachedDocumentRepo struct {
	documents []*domain.AttachedDocument
	err       error
	created   int
}

func (r *fakeAttachedDocumentRepo) Create(ctx context.Context, document *domain.AttachedDocument) error {
	if r.err != nil {
		return r.err
	}
	r.created++
	r.documents = append(r.documents, document)
	return nil
}

func (r *fakeAttachedDocumentRepo) FindByHash(ctx context.Context, tenantID uuid.UUID, hash []byte) (*domain.AttachedDocument, error) {
	if r.err != nil {
		return nil, r.err
	}
	for _, document := range r.documents {
		if document.TenantID == tenantID && bytes.Equal(document.DocumentHash, hash) {
			return document, nil
		}
	}
	return nil, domain.ErrAttachedDocumentNotFound
}

func documentTestService(t *testing.T, repo *fakeAttachedDocumentRepo) (*DocumentService, *local.Storage, string, uuid.UUID) {
	t.Helper()
	root := t.TempDir()
	storage, err := local.New(root)
	require.NoError(t, err)
	tenantID := uuid.New()
	service := NewDocumentService(storage, repo, newFakeTxManager(), "local",
		func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) })
	return service, storage, root, tenantID
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	files := 0
	require.NoError(t, filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files++
		}
		return nil
	}))
	return files
}

func TestUploadDocumentDeduplicatesIdenticalContent(t *testing.T) {
	repo := &fakeAttachedDocumentRepo{}
	service, storage, root, tenantID := documentTestService(t, repo)
	content := []byte("duplicate content")

	first, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "first.txt", MimeType: "text/plain",
		Reader: bytes.NewReader(content),
	})
	require.NoError(t, err)

	second, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "copy.txt", MimeType: "text/plain",
		Reader: bytes.NewReader(content),
	})
	require.NoError(t, err)

	expected := sha256.Sum256(content)
	assert.Equal(t, expected[:], first.DocumentHash)
	assert.Equal(t, first.StorageKey, second.StorageKey)
	assert.Equal(t, "local", first.StorageProvider)
	assert.Equal(t, int64(len(content)), first.SizeBytes)
	assert.Equal(t, 2, repo.created, "each upload creates its own metadata row")
	assert.Equal(t, 1, countFiles(t, root),
		"identical content must be stored only once")

	exists, err := storage.Exists(context.Background(), first.StorageKey)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestUploadDocumentStoresDifferentContentSeparately(t *testing.T) {
	repo := &fakeAttachedDocumentRepo{}
	service, _, root, tenantID := documentTestService(t, repo)

	first, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "a.txt", MimeType: "text/plain", Reader: bytes.NewReader([]byte("a")),
	})
	require.NoError(t, err)
	second, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "b.txt", MimeType: "text/plain", Reader: bytes.NewReader([]byte("b")),
	})
	require.NoError(t, err)

	assert.NotEqual(t, first.StorageKey, second.StorageKey)
	assert.Equal(t, 2, countFiles(t, root))
}

func TestUploadDocumentRejectsInvalidInput(t *testing.T) {
	repo := &fakeAttachedDocumentRepo{}
	service, _, _, tenantID := documentTestService(t, repo)

	tests := []UploadDocumentInput{
		{TenantID: uuid.Nil, OriginTable: "parties", OriginID: uuid.New(), OriginalName: "a.txt", Reader: bytes.NewReader([]byte("a"))},
		{TenantID: tenantID, OriginTable: " ", OriginID: uuid.New(), OriginalName: "a.txt", Reader: bytes.NewReader([]byte("a"))},
		{TenantID: tenantID, OriginTable: "parties", OriginID: uuid.Nil, OriginalName: "a.txt", Reader: bytes.NewReader([]byte("a"))},
		{TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(), OriginalName: "../a.txt", Reader: bytes.NewReader([]byte("a"))},
		{TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(), OriginalName: "", Reader: bytes.NewReader([]byte("a"))},
		{TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(), OriginalName: "a.txt", Reader: nil},
	}

	for _, input := range tests {
		_, err := service.Upload(context.Background(), input)
		var appErr *apperror.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperror.TypeValidation, appErr.Type)
	}
}

func TestUploadDocumentRejectsEmptyContent(t *testing.T) {
	repo := &fakeAttachedDocumentRepo{}
	service, _, _, tenantID := documentTestService(t, repo)

	_, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "empty.txt", MimeType: "text/plain", Reader: bytes.NewReader(nil),
	})

	var appErr *apperror.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.TypeValidation, appErr.Type)
}

func TestUploadDocumentPropagatesRepositoryErrors(t *testing.T) {
	repo := &fakeAttachedDocumentRepo{err: errors.New("database down")}
	service, _, _, tenantID := documentTestService(t, repo)

	_, err := service.Upload(context.Background(), UploadDocumentInput{
		TenantID: tenantID, OriginTable: "parties", OriginID: uuid.New(),
		OriginalName: "a.txt", MimeType: "text/plain", Reader: bytes.NewReader([]byte("a")),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database down")
}
