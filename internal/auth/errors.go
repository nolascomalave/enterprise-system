package auth

import "errors"

var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("expired token")
	ErrInvalidTOTP            = errors.New("invalid TOTP code")
	ErrInvalidBackupCode      = errors.New("invalid backup code")
	Err2FARequired            = errors.New("2FA required")
	Err2FANotEnabled          = errors.New("2FA is not enabled")
	ErrSessionRevoked         = errors.New("session revoked")
	ErrSessionExpired         = errors.New("session expired")
	ErrWeakPassword           = errors.New("password does not meet minimum requirements")
	ErrInvalidCurrentPassword = errors.New("invalid current password")
	ErrSamePassword           = errors.New("new password must differ from current password")
	ErrInvalidResetToken      = errors.New("invalid or expired password reset token")
	// ErrConcurrentModification means another request changed the same record
	// between the read and the conditional write.
	ErrConcurrentModification = errors.New("record modified concurrently")
)
