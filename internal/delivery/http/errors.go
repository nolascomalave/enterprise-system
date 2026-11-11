package http

import (
	"errors"

	"enterprise-system/internal/common/apperror"
)

var (
	errInvalidTenantID  = errors.New("invalid tenant_id")
	errHireDateRequired = errors.New("hire_date is required")
)

// badRequestError classifies invalid input from the HTTP boundary.
func badRequestError(err error) error {
	return apperror.NewValidation("bad_request", "bad request", err)
}

// unauthorizedError classifies missing or invalid authentication.
func unauthorizedError(err error) error {
	return apperror.NewUnauthorized("unauthorized", "unauthorized", err)
}
