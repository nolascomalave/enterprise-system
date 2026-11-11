package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"enterprise-system/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keyFor(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func TestUploadStoresFileInHashSubdirectories(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	require.NoError(t, err)

	content := []byte("hello document")
	key := keyFor(content)
	require.NoError(t, store.Upload(context.Background(), key, bytes.NewReader(content)))

	path := filepath.Join(root, key[:2], key[2:4], key)
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestUploadIsIdempotentForSameKey(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	require.NoError(t, err)

	content := []byte("same content")
	key := keyFor(content)
	require.NoError(t, store.Upload(context.Background(), key, bytes.NewReader(content)))
	require.NoError(t, store.Upload(context.Background(), key, bytes.NewReader(content)))

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
	assert.Equal(t, 1, files, "the same hash must not create a second object")
}

func TestDownloadReturnsContent(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	require.NoError(t, err)

	content := []byte("download me")
	key := keyFor(content)
	require.NoError(t, store.Upload(context.Background(), key, bytes.NewReader(content)))

	reader, err := store.Download(context.Background(), key)
	require.NoError(t, err)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestExistsAndDelete(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	require.NoError(t, err)

	content := []byte("temporary")
	key := keyFor(content)

	exists, err := store.Exists(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, store.Upload(context.Background(), key, bytes.NewReader(content)))
	exists, err = store.Exists(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, exists)

	require.NoError(t, store.Delete(context.Background(), key))
	exists, err = store.Exists(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteMissingFileIsIdempotent(t *testing.T) {
	store, err := New(t.TempDir())
	require.NoError(t, err)

	require.NoError(t, store.Delete(context.Background(), keyFor([]byte("missing"))))
}

func TestDownloadMissingFile(t *testing.T) {
	store, err := New(t.TempDir())
	require.NoError(t, err)

	_, err = store.Download(context.Background(), keyFor([]byte("missing")))
	require.ErrorIs(t, err, domain.ErrFileNotFound)
}

func TestRejectsUnsafeKeys(t *testing.T) {
	store, err := New(t.TempDir())
	require.NoError(t, err)

	for _, key := range []string{
		"",
		"../../etc/passwd",
		"abc",
		"ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789",
		"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
	} {
		_, err := store.Exists(context.Background(), key)
		require.ErrorIsf(t, err, domain.ErrInvalidFileKey, "key %q", key)
	}
}
