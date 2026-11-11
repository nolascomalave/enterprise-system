// Package apperror defines the Hybrid Unified App Error used to classify
// errors across the layers of the application.
//
// Domain and use case code keeps using sentinel errors (errors.Is); AppError
// adds a stable category, a client-safe message and the technical cause that
// must only be logged. The HTTP middleware is the single place that maps an
// AppError to a status code.
package apperror

import "errors"

// Type is the category that determines how the HTTP layer treats an error.
type Type string

const (
	// TypeNotFound means the resource does not exist or is not visible to the
	// caller. It maps to HTTP 404.
	TypeNotFound Type = "not_found"
	// TypeValidation means the request or business rule is invalid. It maps to
	// HTTP 400.
	TypeValidation Type = "validation"
	// TypeUnauthorized means credentials, token or session are invalid. It maps
	// to HTTP 401.
	TypeUnauthorized Type = "unauthorized"
	// TypeInternal means an unexpected technical failure. It maps to HTTP 500
	// and its technical detail is never exposed to the client.
	TypeInternal Type = "internal"
)

const (
	internalCode    = "internal_error"
	internalMessage = "internal server error"
)

// AppError carries a classified error.
//
// Message is safe to return to the client. Err is the underlying technical
// cause and must only be used for logging. Error() includes both so logs keep
// the full chain, while the HTTP middleware only reads Message.
type AppError struct {
	Type    Type
	Code    string
	Message string
	Err     error
}

// Error implements the error interface.
func (e *AppError) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Err == nil:
		return e.Message
	case e.Message == "":
		return e.Err.Error()
	default:
		return e.Message + ": " + e.Err.Error()
	}
}

// Unwrap exposes the technical cause so errors.Is/As keep working through an
// AppError.
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// New builds an AppError with an explicit type.
func New(typ Type, code, message string, err error) *AppError {
	return &AppError{Type: typ, Code: code, Message: message, Err: err}
}

// NewNotFound classifies a missing resource. It maps to HTTP 404.
func NewNotFound(code, message string, err error) *AppError {
	return New(TypeNotFound, code, message, err)
}

// NewValidation classifies invalid input or a violated business rule. It maps
// to HTTP 400.
func NewValidation(code, message string, err error) *AppError {
	return New(TypeValidation, code, message, err)
}

// NewUnauthorized classifies invalid credentials, tokens or sessions. It maps
// to HTTP 401.
func NewUnauthorized(code, message string, err error) *AppError {
	return New(TypeUnauthorized, code, message, err)
}

// NewInternal hides a technical failure behind a generic 500 response while
// preserving the cause for logs.
func NewInternal(err error) *AppError {
	return New(TypeInternal, internalCode, internalMessage, err)
}

// From returns err unchanged when it already is an *AppError; otherwise it is
// wrapped as an internal error. It never returns nil for a non-nil error.
func From(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return NewInternal(err)
}
