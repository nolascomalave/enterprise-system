package repository

import (
	"errors"
	"testing"

	"enterprise-system/ent"
	"enterprise-system/internal/common/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoErrorKeepsNil(t *testing.T) {
	assert.NoError(t, repoError(nil))
}

func TestRepoErrorPreservesAppError(t *testing.T) {
	original := apperror.NewValidation("invalid", "invalid", nil)

	got := repoError(original)

	assert.Same(t, original, got)
}

func TestRepoErrorTranslatesNotFound(t *testing.T) {
	cause := &ent.NotFoundError{}

	got := repoError(cause)

	var appErr *apperror.AppError
	require.ErrorAs(t, got, &appErr)
	assert.Equal(t, apperror.TypeNotFound, appErr.Type)
	// The technical cause must remain inspectable by the upper layers.
	assert.True(t, ent.IsNotFound(got))
}

func TestRepoErrorTranslatesTechnicalFailure(t *testing.T) {
	cause := errors.New("connection refused")

	got := repoError(cause)

	var appErr *apperror.AppError
	require.ErrorAs(t, got, &appErr)
	assert.Equal(t, apperror.TypeInternal, appErr.Type)
	assert.ErrorIs(t, got, cause)
}

func TestRepoErrorTranslatesConstraintViolation(t *testing.T) {
	cause := &ent.ConstraintError{}

	got := repoError(cause)

	var appErr *apperror.AppError
	require.ErrorAs(t, got, &appErr)
	assert.Equal(t, apperror.TypeValidation, appErr.Type)
	assert.True(t, ent.IsConstraintError(got))
}
