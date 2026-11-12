package apperror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotFound(t *testing.T) {
	cause := errors.New("ent: party not found")
	err := NewNotFound("party_not_found", "party not found", cause)

	assert.Equal(t, TypeNotFound, err.Type)
	assert.Equal(t, "party_not_found", err.Code)
	assert.Equal(t, "party not found", err.Message)
	assert.ErrorIs(t, err, cause)
	assert.Contains(t, err.Error(), "party not found")
}

func TestNewValidation(t *testing.T) {
	cause := errors.New("invalid uuid")
	err := NewValidation("invalid_party_id", "invalid party id", cause)

	assert.Equal(t, TypeValidation, err.Type)
	assert.Equal(t, "invalid_party_id", err.Code)
	assert.Equal(t, "invalid party id", err.Message)
	assert.ErrorIs(t, err, cause)
}

func TestNewUnauthorized(t *testing.T) {
	cause := errors.New("bad token")
	err := NewUnauthorized("unauthorized", "unauthorized", cause)

	assert.Equal(t, TypeUnauthorized, err.Type)
	assert.Equal(t, "unauthorized", err.Code)
	assert.ErrorIs(t, err, cause)
}

func TestNewInternal(t *testing.T) {
	cause := errors.New("pq: password=secret")
	err := NewInternal(cause)

	assert.Equal(t, TypeInternal, err.Type)
	assert.Equal(t, "internal_error", err.Code)
	assert.Equal(t, "internal server error", err.Message)
	assert.ErrorIs(t, err, cause)
}

func TestFromPreservesAppError(t *testing.T) {
	original := NewValidation("bad_request", "bad request", nil)

	got := From(original)

	assert.Same(t, original, got)
}

func TestFromWrapsUnknownAsInternal(t *testing.T) {
	cause := errors.New("connection refused")
	got := From(cause)

	assert.Equal(t, TypeInternal, got.Type)
	assert.ErrorIs(t, got, cause)
}

func TestErrorsAsFindsWrappedAppError(t *testing.T) {
	original := NewNotFound("party_not_found", "party not found", nil)
	wrapped := fmt.Errorf("get party: %w", original)

	var target *AppError
	require.ErrorAs(t, wrapped, &target)
	assert.Equal(t, "party_not_found", target.Code)
}

func TestUnwrapPreservesSentinel(t *testing.T) {
	sentinel := errors.New("domain sentinel")
	err := NewValidation("invalid", "invalid", sentinel)

	assert.ErrorIs(t, err, sentinel)
}
