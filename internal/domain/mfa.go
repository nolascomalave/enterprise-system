package domain

import (
	"context"

	"github.com/google/uuid"
)

type MFAMethod string

const (
	MFAMethodNone     MFAMethod = "NONE"
	MFAMethodTOTP     MFAMethod = "TOTP"
	MFAMethodEmailOTP MFAMethod = "EMAIL_OTP"
)

func (m MFAMethod) Valid() bool {
	switch m {
	case MFAMethodNone, MFAMethodTOTP, MFAMethodEmailOTP:
		return true
	default:
		return false
	}
}

// SecurityCodeSender delivers a one-time security code through a channel.
type SecurityCodeSender interface {
	SendCode(ctx context.Context, destination, code string, contactType ContactType) error
}

// SecurityCodeManager generates and consumes one-time security codes.
type SecurityCodeManager interface {
	Generate(ctx context.Context, userID uuid.UUID, action string) (string, error)
	Verify(ctx context.Context, userID uuid.UUID, action, code string) error
	Invalidate(ctx context.Context, userID uuid.UUID, action string) error
}
