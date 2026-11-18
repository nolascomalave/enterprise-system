package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// Supported security-code actions. Keeping a closed catalog prevents callers
// from creating arbitrary Redis keys under the user namespace.
const (
	SecurityActionChangePassword = "CHANGE_PASSWORD"
	SecurityActionChangeEmail    = "CHANGE_EMAIL"
	SecurityActionEnableMFA      = "ENABLE_MFA"
	SecurityActionDisableMFA     = "DISABLE_MFA"
	SecurityActionLogin          = "LOGIN"
)

var allowedSecurityActions = map[string]struct{}{
	SecurityActionChangePassword: {},
	SecurityActionChangeEmail:    {},
	SecurityActionEnableMFA:      {},
	SecurityActionDisableMFA:     {},
	SecurityActionLogin:          {},
}

// TOTPManager generates and validates TOTP secrets/codes.
type TOTPManager interface {
	GenerateTOTPSecret(issuer, account string) (secret, otpauthURL string, err error)
	ValidateTOTP(secret, code string) bool
	GenerateBackupCodesSHA256(count int) (plain, hashes []string, err error)
}

// MFACredentialRepository is the narrow credential port used by MFAService.
type MFACredentialRepository interface {
	GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*auth.Credentials, error)
	UpdateMFAPreferences(ctx context.Context, tenantID, partyID uuid.UUID, enabled bool, method domain.MFAMethod, at time.Time) error
	UpdateTOTPSecret(ctx context.Context, tenantID, partyID uuid.UUID, secret string, at time.Time) error
	ActivateTOTP(ctx context.Context, tenantID, partyID uuid.UUID, backupCodesHash []string, at time.Time) error
}

// MFAContactRepository is the narrow contact port used by MFAService.
type MFAContactRepository interface {
	ListPartyContacts(ctx context.Context, tenantID, partyID uuid.UUID) ([]*domain.PartyContact, error)
	GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*domain.PartyContact, error)
}

// MFAService manages MFA preferences and security-code delivery.
type MFAService struct {
	users    MFACredentialRepository
	contacts MFAContactRepository
	codes    domain.SecurityCodeManager
	sender   domain.SecurityCodeSender
	totp     TOTPManager
	tx       TransactionManager
	now      func() time.Time
}

func NewMFAService(
	users MFACredentialRepository,
	contacts MFAContactRepository,
	codes domain.SecurityCodeManager,
	sender domain.SecurityCodeSender,
	tx TransactionManager,
	now func() time.Time,
) *MFAService {
	if now == nil {
		now = time.Now
	}
	return &MFAService{
		users: users, contacts: contacts, codes: codes,
		sender: sender, tx: tx, now: now,
	}
}

// SetTOTPManager wires the TOTP generator/validator used by enrolment.
func (s *MFAService) SetTOTPManager(manager TOTPManager) {
	s.totp = manager
}

const totpIssuer = "enterprise-system"

type SetupTOTPResult struct {
	Secret     string
	OTPAuthURL string
}

// SetupTOTP generates a new TOTP secret and stores it as pending enrolment.
// MFA stays disabled until VerifyTOTP validates a code from the user's app.
func (s *MFAService) SetupTOTP(ctx context.Context, tenantID, partyID uuid.UUID) (*SetupTOTPResult, error) {
	if tenantID == uuid.Nil || partyID == uuid.Nil {
		return nil, validationError("invalid_input", "tenant_id and party_id are required", nil)
	}
	if s.totp == nil {
		return nil, internalError(errors.New("TOTP manager is not configured"))
	}

	var result *SetupTOTPResult
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		credentials, err := s.users.GetByPartyID(ctx, tenantID, partyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return notFoundError("user_not_found", "user credentials not found", err)
			}
			return fmt.Errorf("load user credentials: %w", err)
		}
		if credentials.MFAEnabled {
			if credentials.MFAMethod == domain.MFAMethodTOTP && credentials.TOTPVerified {
				return validationError("totp_already_enabled", "TOTP is already enabled", domain.ErrTOTPAlreadyEnabled)
			}
			return validationError(
				"mfa_already_active",
				"disable the active MFA method before starting a new enrolment",
				domain.ErrMFAActive,
			)
		}

		account := partyID.String()
		if s.contacts != nil {
			if partyContact, err := s.contacts.GetPartyContact(ctx, tenantID, credentials.PartyContactMechanismID); err == nil &&
				partyContact != nil && partyContact.Contact != nil &&
				partyContact.Contact.Type == domain.ContactTypeEmail &&
				strings.TrimSpace(partyContact.Contact.Value) != "" {
				account = partyContact.Contact.Value
			}
		}

		secret, otpauthURL, err := s.totp.GenerateTOTPSecret(totpIssuer, account)
		if err != nil {
			return internalError(fmt.Errorf("generate TOTP secret: %w", err))
		}
		if err := s.users.UpdateTOTPSecret(ctx, tenantID, partyID, secret, s.now().UTC()); err != nil {
			return fmt.Errorf("store TOTP secret: %w", err)
		}
		result = &SetupTOTPResult{Secret: secret, OTPAuthURL: otpauthURL}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// VerifyTOTP validates a code against the pending secret, enables TOTP and
// returns one-time backup codes. The plaintext codes are returned only here.
type VerifyTOTPResult struct {
	BackupCodes []string
}

func (s *MFAService) VerifyTOTP(ctx context.Context, tenantID, partyID uuid.UUID, code string) (*VerifyTOTPResult, error) {
	if tenantID == uuid.Nil || partyID == uuid.Nil {
		return nil, validationError("invalid_input", "tenant_id and party_id are required", nil)
	}
	if s.totp == nil {
		return nil, internalError(errors.New("TOTP manager is not configured"))
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, validationError("invalid_totp", "TOTP code is required", auth.ErrInvalidTOTP)
	}

	var result *VerifyTOTPResult
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		credentials, err := s.users.GetByPartyID(ctx, tenantID, partyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return notFoundError("user_not_found", "user credentials not found", err)
			}
			return fmt.Errorf("load user credentials: %w", err)
		}
		if strings.TrimSpace(credentials.TOTPSecret) == "" {
			return validationError("totp_not_setup", "TOTP setup must be started first", domain.ErrTOTPNotConfigured)
		}
		if credentials.MFAEnabled {
			if credentials.MFAMethod == domain.MFAMethodTOTP && credentials.TOTPVerified {
				return validationError("totp_already_enabled", "TOTP is already enabled", domain.ErrTOTPAlreadyEnabled)
			}
			return validationError(
				"mfa_already_active",
				"disable the active MFA method before enrolling TOTP",
				domain.ErrMFAActive,
			)
		}
		if !s.totp.ValidateTOTP(credentials.TOTPSecret, code) {
			return validationError("invalid_totp", "invalid TOTP code", auth.ErrInvalidTOTP)
		}
		hasContact, err := s.hasCodeCapableContact(ctx, tenantID, partyID)
		if err != nil {
			return fmt.Errorf("check security contact: %w", err)
		}
		if !hasContact {
			return validationError(
				"mfa_contact_required",
				"at least one contact mechanism that can receive security codes is required",
				domain.ErrMFAContactRequired,
			)
		}
		plain, hashes, err := s.totp.GenerateBackupCodesSHA256(8)
		if err != nil {
			return internalError(fmt.Errorf("generate backup codes: %w", err))
		}
		if err := s.users.ActivateTOTP(ctx, tenantID, partyID, hashes, s.now().UTC()); err != nil {
			return fmt.Errorf("enable TOTP: %w", err)
		}
		result = &VerifyTOTPResult{BackupCodes: plain}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type UpdateMFAPreferencesInput struct {
	TenantID uuid.UUID
	PartyID  uuid.UUID
	Enabled  bool
	Method   domain.MFAMethod
	OTPCode  string
}

// UpdateMFAPreferences enables or disables MFA. Enabling requires at least one
// active contact mechanism that can actually receive security codes.
func (s *MFAService) UpdateMFAPreferences(ctx context.Context, in UpdateMFAPreferencesInput) error {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil {
		return validationError("invalid_input", "tenant_id and party_id are required", nil)
	}

	method := in.Method
	if !in.Enabled {
		method = domain.MFAMethodNone
	} else {
		if method == "" {
			return validationError("invalid_mfa_method", "mfa method is required", domain.ErrInvalidMFAMethod)
		}
		if !method.Valid() || method == domain.MFAMethodNone {
			return validationError("invalid_mfa_method", "unsupported MFA method", domain.ErrInvalidMFAMethod)
		}
	}

	// Validate the contact and persist the preference in the same transaction
	// so the check cannot race with a concurrent contact removal.
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		credentials, err := s.users.GetByPartyID(ctx, in.TenantID, in.PartyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return notFoundError("user_not_found", "user credentials not found", err)
			}
			return fmt.Errorf("load user credentials: %w", err)
		}

		// Changing an already-active MFA configuration requires step-up. A
		// first-time enable (MFA was off) does not need a code.
		requiresStepUp := credentials.MFAEnabled && (!in.Enabled || method != credentials.MFAMethod)
		if requiresStepUp {
			if strings.TrimSpace(in.OTPCode) == "" {
				return validationError("otp_required", "otp_code is required to change an active MFA configuration", domain.ErrOTPRequired)
			}
			if s.codes == nil {
				return internalError(errors.New("security code manager is not configured"))
			}
			action := SecurityActionEnableMFA
			if !in.Enabled {
				action = SecurityActionDisableMFA
			}
			if err := s.codes.Verify(ctx, in.PartyID, action, in.OTPCode); err != nil {
				switch {
				case errors.Is(err, domain.ErrInvalidSecurityCode):
					return validationError("invalid_otp", "invalid or expired security code", err)
				case errors.Is(err, domain.ErrOTPServiceUnavailable):
					return internalError(err)
				default:
					return internalError(err)
				}
			}
		}

		if in.Enabled && method == domain.MFAMethodTOTP {
			if strings.TrimSpace(credentials.TOTPSecret) == "" {
				return validationError(
					"totp_not_configured",
					"TOTP must be configured before it can be enabled",
					domain.ErrTOTPNotConfigured,
				)
			}
			if !credentials.TOTPVerified {
				return validationError(
					"totp_not_verified",
					"TOTP enrolment must be verified before it can be enabled",
					domain.ErrTOTPNotVerified,
				)
			}
		}
		if in.Enabled {
			hasContact, err := s.hasCodeCapableContact(ctx, in.TenantID, in.PartyID)
			if err != nil {
				return fmt.Errorf("check security contact: %w", err)
			}
			if !hasContact {
				return validationError(
					"mfa_contact_required",
					"at least one contact mechanism that can receive security codes is required",
					domain.ErrMFAContactRequired,
				)
			}
		}
		if err := s.users.UpdateMFAPreferences(ctx, in.TenantID, in.PartyID, in.Enabled, method, s.now().UTC()); err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return notFoundError("user_not_found", "user credentials not found", err)
			}
			return fmt.Errorf("update MFA preferences: %w", err)
		}
		return nil
	})
}

func (s *MFAService) hasCodeCapableContact(ctx context.Context, tenantID, partyID uuid.UUID) (bool, error) {
	contacts, err := s.contacts.ListPartyContacts(ctx, tenantID, partyID)
	if err != nil {
		return false, err
	}
	for _, contact := range contacts {
		if contact == nil || contact.Relation == nil || contact.Contact == nil {
			continue
		}
		if contact.Relation.EndDate != nil {
			continue
		}
		if contact.Contact.CanReceiveSecurityCode && contact.Contact.Type.CanReceiveSecurityCode() {
			return true, nil
		}
	}
	return false, nil
}

type RequestSecurityCodeInput struct {
	TenantID uuid.UUID
	PartyID  uuid.UUID
	Action   string
}

// RequestSecurityCode generates and delivers a security code to the user's
// primary contact mechanism. The code is invalidated if delivery fails.
func (s *MFAService) RequestSecurityCode(ctx context.Context, in RequestSecurityCodeInput) error {
	if in.TenantID == uuid.Nil || in.PartyID == uuid.Nil {
		return validationError("invalid_input", "tenant_id and party_id are required", nil)
	}
	action, err := normalizeSecurityAction(in.Action)
	if err != nil {
		return err
	}

	type loadedContact struct {
		relation *domain.PartyContactMechanism
		contact  *domain.ContactMechanism
	}
	loaded, err := readWithinTransaction(ctx, s.tx, func(ctx context.Context) (loadedContact, error) {
		credentials, err := s.users.GetByPartyID(ctx, in.TenantID, in.PartyID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				return loadedContact{}, notFoundError("user_not_found", "user credentials not found", err)
			}
			return loadedContact{}, fmt.Errorf("load user credentials: %w", err)
		}
		partyContact, err := s.contacts.GetPartyContact(ctx, in.TenantID, credentials.PartyContactMechanismID)
		if err != nil {
			if errors.Is(err, domain.ErrPartyContactNotFound) {
				return loadedContact{}, notFoundError("contact_not_found", "security contact not found", err)
			}
			return loadedContact{}, fmt.Errorf("load security contact: %w", err)
		}
		if partyContact == nil || partyContact.Relation == nil || partyContact.Contact == nil {
			return loadedContact{}, notFoundError("contact_not_found", "security contact not found", nil)
		}
		if partyContact.Relation.PartyID != in.PartyID {
			return loadedContact{}, notFoundError("contact_not_found", "security contact not found", nil)
		}
		return loadedContact{relation: partyContact.Relation, contact: partyContact.Contact}, nil
	})
	if err != nil {
		return err
	}

	if loaded.relation.EndDate != nil {
		return validationError(
			"contact_inactive",
			"the primary contact mechanism is no longer active",
			domain.ErrUnsupportedSecurityCodeChannel,
		)
	}
	contact := loaded.contact
	if strings.TrimSpace(contact.Value) == "" || !contact.CanReceiveSecurityCode || !contact.Type.CanReceiveSecurityCode() {
		return validationError(
			"contact_cannot_receive_codes",
			"the primary contact mechanism cannot receive security codes",
			domain.ErrUnsupportedSecurityCodeChannel,
		)
	}

	code, err := s.codes.Generate(ctx, in.PartyID, action)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOTPCooldown):
			return validationError("otp_cooldown", "wait before requesting another security code", err)
		case errors.Is(err, domain.ErrOTPServiceUnavailable):
			return internalError(err)
		default:
			return internalError(fmt.Errorf("generate security code: %w", err))
		}
	}

	if err := s.sender.SendCode(ctx, contact.Value, code, contact.Type); err != nil {
		s.invalidateQuietly(ctx, in.PartyID, action)
		if errors.Is(err, domain.ErrUnsupportedSecurityCodeChannel) {
			return validationError("unsupported_security_channel", "security code channel is not supported", err)
		}
		return internalError(fmt.Errorf("send security code: %w", err))
	}
	return nil
}

func (s *MFAService) invalidateQuietly(ctx context.Context, partyID uuid.UUID, action string) {
	if s.codes == nil {
		return
	}
	_ = s.codes.Invalidate(context.WithoutCancel(ctx), partyID, action)
}

func normalizeSecurityAction(action string) (string, error) {
	action = strings.ToUpper(strings.TrimSpace(action))
	if _, ok := allowedSecurityActions[action]; !ok {
		return "", validationError("invalid_security_action", "unsupported security code action", nil)
	}
	return action, nil
}
