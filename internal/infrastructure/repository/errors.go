package repository

import (
	"errors"

	"enterprise-system/ent"
	"enterprise-system/internal/common/apperror"
)

// repoError translates an unexpected infrastructure failure into an internal
// AppError while preserving nil and already classified errors.
func repoError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return err
	}
	if ent.IsNotFound(err) {
		return apperror.NewNotFound("not_found", "resource not found", err)
	}
	if ent.IsConstraintError(err) {
		return apperror.NewValidation("constraint_violation", "resource conflicts with existing data", err)
	}
	return apperror.NewInternal(err)
}
