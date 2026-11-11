package usecase

import "enterprise-system/internal/common/apperror"

// validationError classifies invalid input or a violated business rule.
func validationError(code, message string, err error) error {
	return apperror.NewValidation(code, message, err)
}

// notFoundError classifies a missing (or tenant-hidden) resource.
func notFoundError(code, message string, err error) error {
	return apperror.NewNotFound(code, message, err)
}

// unauthorizedError classifies invalid credentials, tokens or sessions.
func unauthorizedError(code, message string, err error) error {
	return apperror.NewUnauthorized(code, message, err)
}

// internalError hides an unexpected technical failure.
func internalError(err error) error {
	return apperror.NewInternal(err)
}

// --- Auth domain classifiers -------------------------------------------------

func invalidCredentialsError(cause error) error {
	return unauthorizedError("invalid_credentials", "invalid credentials", cause)
}

func invalidTokenError(cause error) error {
	return unauthorizedError("invalid_token", "invalid token", cause)
}

func invalidTOTPError(cause error) error {
	return unauthorizedError("invalid_totp", "invalid 2FA code", cause)
}

func sessionRevokedError(cause error) error {
	return unauthorizedError("session_revoked", "session revoked", cause)
}

func sessionExpiredError(cause error) error {
	return unauthorizedError("session_expired", "session expired", cause)
}

func invalidCurrentPasswordError(cause error) error {
	return unauthorizedError("invalid_current_password", "invalid current password", cause)
}

func twoFANotEnabledError(cause error) error {
	return validationError("2fa_not_enabled", "2FA is not enabled", cause)
}

func weakPasswordError(cause error) error {
	return validationError("weak_password", "password does not meet minimum requirements", cause)
}

func samePasswordError(cause error) error {
	return validationError("same_password", "new password must differ from current password", cause)
}

func invalidResetTokenError(cause error) error {
	return validationError("invalid_reset_token", "invalid or expired password reset token", cause)
}
