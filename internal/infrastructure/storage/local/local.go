// Package local implements the domain.FileStorage port on the local
// filesystem using a content-addressed layout (SHA-256 hex key).
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"enterprise-system/internal/domain"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Storage stores files as <root>/<first2>/<next2>/<sha256>.
type Storage struct {
	root string
}

// New creates a local storage rooted at dir. The directory is created if it
// does not exist.
func New(dir string) (*Storage, error) {
	if dir == "" {
		return nil, errors.New("local storage: empty root directory")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("local storage: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("local storage: create root: %w", err)
	}
	return &Storage{root: abs}, nil
}

func (s *Storage) objectPath(key string) (string, error) {
	if !sha256Hex.MatchString(key) {
		return "", domain.ErrInvalidFileKey
	}
	return filepath.Join(s.root, key[:2], key[2:4], key), nil
}

func (s *Storage) Upload(ctx context.Context, key string, content io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.objectPath(key)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("local storage: create object directory: %w", err)
	}

	// Content-addressed writes are idempotent: an existing object already has
	// the requested bytes.
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local storage: stat object: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return fmt.Errorf("local storage: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, content); err != nil {
		tmp.Close()
		return fmt.Errorf("local storage: write object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("local storage: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		return fmt.Errorf("local storage: set object permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("local storage: publish object: %w", err)
	}
	return nil
}

func (s *Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.objectPath(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("local storage: open object: %w", err)
	}
	return file, nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.objectPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("local storage: delete object: %w", err)
	}
	return nil
}

func (s *Storage) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	path, err := s.objectPath(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("local storage: stat object: %w", err)
	}
}
