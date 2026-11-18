package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMFAUserRepository struct {
	credentials *auth.Credentials
	getErr      error
	updateErr   error

	updatedTenant     uuid.UUID
	updatedParty      uuid.UUID
	updatedEnable     bool
	updatedMethod     domain.MFAMethod
	updateCalls       int
	updatedTOTPSecret string
	updateTOTPCalls   int
	updateTOTPErr     error

	activateCalls int
	activateErr   error
}

func (r *fakeMFAUserRepository) GetByPartyID(ctx context.Context, tenantID, partyID uuid.UUID) (*auth.Credentials, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.credentials == nil {
		return nil, auth.ErrNotFound
	}
	return r.credentials, nil
}

func (r *fakeMFAUserRepository) UpdateTOTPSecret(ctx context.Context, tenantID, partyID uuid.UUID, secret string, at time.Time) error {
	r.updateTOTPCalls++
	if r.updateTOTPErr != nil {
		return r.updateTOTPErr
	}
	r.updatedTOTPSecret = secret
	return nil
}

func (r *fakeMFAUserRepository) ActivateTOTP(ctx context.Context, tenantID, partyID uuid.UUID, at time.Time) error {
	r.activateCalls++
	return r.activateErr
}

func (r *fakeMFAUserRepository) UpdateMFAPreferences(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	enabled bool,
	method domain.MFAMethod,
	at time.Time,
) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updatedTenant, r.updatedParty = tenantID, partyID
	r.updatedEnable, r.updatedMethod = enabled, method
	return nil
}

type fakeMFAContactRepository struct {
	contacts  []*domain.PartyContact
	byID      map[uuid.UUID]*domain.PartyContact
	listErr   error
	getErr    error
	listCalls int
	getCalls  int
}

func (r *fakeMFAContactRepository) ListPartyContacts(ctx context.Context, tenantID, partyID uuid.UUID) ([]*domain.PartyContact, error) {
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.contacts, nil
}

func (r *fakeMFAContactRepository) GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*domain.PartyContact, error) {
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	contact, ok := r.byID[relationID]
	if !ok {
		return nil, domain.ErrPartyContactNotFound
	}
	return contact, nil
}

type fakeSecurityCodeManager struct {
	code          string
	generateErr   error
	verifyErr     error
	invalidateErr error

	generatedUser   uuid.UUID
	generatedAction string
	generateCalls   int
	invalidated     []string
	invalidateCalls int

	verifiedUser   uuid.UUID
	verifiedAction string
	verifiedCode   string
	verifyCalls    int
}

func (m *fakeSecurityCodeManager) Generate(ctx context.Context, userID uuid.UUID, action string) (string, error) {
	m.generateCalls++
	m.generatedUser, m.generatedAction = userID, action
	return m.code, m.generateErr
}

func (m *fakeSecurityCodeManager) Verify(ctx context.Context, userID uuid.UUID, action, code string) error {
	m.verifyCalls++
	m.verifiedUser, m.verifiedAction, m.verifiedCode = userID, action, code
	return m.verifyErr
}

func (m *fakeSecurityCodeManager) Invalidate(ctx context.Context, userID uuid.UUID, action string) error {
	m.invalidateCalls++
	m.invalidated = append(m.invalidated, action)
	return m.invalidateErr
}

type fakeSecurityCodeSender struct {
	err         error
	destination string
	code        string
	contactType domain.ContactType
	calls       int
}

func (s *fakeSecurityCodeSender) SendCode(ctx context.Context, destination, code string, contactType domain.ContactType) error {
	s.calls++
	s.destination, s.code, s.contactType = destination, code, contactType
	return s.err
}

func codeCapableContact(tenantID, partyID uuid.UUID) *domain.PartyContact {
	return &domain.PartyContact{
		Relation: &domain.PartyContactMechanism{
			ID: uuid.New(), TenantID: tenantID, PartyID: partyID,
			IsPrimary: true, ContactMechanismOrder: 0,
			StartDate: time.Now().Add(-time.Hour),
		},
		Contact: &domain.ContactMechanism{
			ID: uuid.New(), Type: domain.ContactTypeEmail,
			Value: "user@example.com", CanReceiveSecurityCode: true,
		},
	}
}

func newMFAServiceForTest(
	users *fakeMFAUserRepository,
	contacts *fakeMFAContactRepository,
	codes *fakeSecurityCodeManager,
	sender *fakeSecurityCodeSender,
) *MFAService {
	return NewMFAService(users, contacts, codes, sender, newFakeTxManager(), time.Now)
}

func TestUpdateMFAPreferencesEnablesEmailOTPWithValidContact(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{codeCapableContact(tenantID, partyID)}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethodEmailOTP,
	})

	require.NoError(t, err)
	require.Equal(t, 1, users.updateCalls)
	assert.True(t, users.updatedEnable)
	assert.Equal(t, domain.MFAMethodEmailOTP, users.updatedMethod)
}

func TestUpdateMFAPreferencesRequiresCodeCapableContact(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{{
		Relation: &domain.PartyContactMechanism{ID: uuid.New(), TenantID: tenantID, PartyID: partyID},
		Contact: &domain.ContactMechanism{
			ID: uuid.New(), Type: domain.ContactTypeFax,
			Value: "+15555550100", CanReceiveSecurityCode: false,
		},
	}}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethodEmailOTP,
	})

	require.ErrorIs(t, err, domain.ErrMFAContactRequired)
	assert.Equal(t, 0, users.updateCalls)
}

func TestUpdateMFAPreferencesRejectsInvalidMethod(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethod("SMS"),
	})

	require.ErrorIs(t, err, domain.ErrInvalidMFAMethod)
	assert.Equal(t, 0, users.updateCalls)
}

func TestUpdateMFAPreferencesDisablingForcesNone(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: false, Method: domain.MFAMethodEmailOTP,
	})

	require.NoError(t, err)
	require.Equal(t, 1, users.updateCalls)
	assert.False(t, users.updatedEnable)
	assert.Equal(t, domain.MFAMethodNone, users.updatedMethod)
}

func TestRequestSecurityCodeSendsCodeToPrimaryContact(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, codes.generateCalls)
	assert.Equal(t, partyID, codes.generatedUser)
	assert.Equal(t, "CHANGE_PASSWORD", codes.generatedAction)
	require.Equal(t, 1, sender.calls)
	assert.Equal(t, "user@example.com", sender.destination)
	assert.Equal(t, "123456", sender.code)
	assert.Equal(t, domain.ContactTypeEmail, sender.contactType)
}

func TestRequestSecurityCodeRejectsContactThatCannotReceiveCodes(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{
		relationID: {
			Relation: &domain.PartyContactMechanism{ID: relationID, TenantID: tenantID, PartyID: partyID},
			Contact: &domain.ContactMechanism{
				ID: uuid.New(), Type: domain.ContactTypeWebsite,
				Value: "https://example.com", CanReceiveSecurityCode: false,
			},
		},
	}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.ErrorIs(t, err, domain.ErrUnsupportedSecurityCodeChannel)
	assert.Equal(t, 0, codes.generateCalls)
	assert.Equal(t, 0, sender.calls)
}

func TestRequestSecurityCodeMapsCooldown(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{generateErr: domain.ErrOTPCooldown}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.ErrorIs(t, err, domain.ErrOTPCooldown)
	assert.Equal(t, 0, sender.calls)
}

func TestRequestSecurityCodeFailsClosedOnRedisError(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{generateErr: domain.ErrOTPServiceUnavailable}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrOTPServiceUnavailable)
	assert.Equal(t, 0, sender.calls)
}

func TestRequestSecurityCodeInvalidatesWhenDeliveryFails(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{err: errors.New("smtp down")}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.Error(t, err)
	assert.Equal(t, 1, codes.invalidateCalls)
	assert.Equal(t, "CHANGE_PASSWORD", codes.invalidated[0])
}

func TestRequestSecurityCodeRejectsUnknownAction(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "DROP EVERYTHING",
	})

	require.Error(t, err)
	assert.Equal(t, 0, codes.generateCalls)
	assert.Equal(t, 0, sender.calls)
}

func TestRequestSecurityCodeReturnsNotFoundWhenCredentialsMissing(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{}
	codes := &fakeSecurityCodeManager{code: "123456"}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, codes, &fakeSecurityCodeSender{})

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.Error(t, err)
	assert.Equal(t, 0, codes.generateCalls)
}

func TestUpdateMFAPreferencesRejectsTOTPWithoutSecret(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{codeCapableContact(tenantID, partyID)}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethodTOTP,
	})

	require.ErrorIs(t, err, domain.ErrTOTPNotConfigured)
	assert.Equal(t, 0, users.activateCalls)
}

func TestUpdateMFAPreferencesRejectsUnverifiedTOTP(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, TOTPSecret: "JBSWY3DPEHPK3PXP", TOTPVerified: false,
	}}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{codeCapableContact(tenantID, partyID)}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethodTOTP,
	})

	require.ErrorIs(t, err, domain.ErrTOTPNotVerified)
	assert.Equal(t, 0, users.updateCalls)
}

func TestUpdateMFAPreferencesAllowsTOTPWithSecret(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, TOTPSecret: "JBSWY3DPEHPK3PXP", TOTPVerified: true,
	}}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{codeCapableContact(tenantID, partyID)}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})

	err := service.UpdateMFAPreferences(context.Background(), UpdateMFAPreferencesInput{
		TenantID: tenantID, PartyID: partyID, Enabled: true, Method: domain.MFAMethodTOTP,
	})

	require.NoError(t, err)
	require.Equal(t, 1, users.updateCalls)
	assert.Equal(t, domain.MFAMethodTOTP, users.updatedMethod)
}

func TestRequestSecurityCodeRejectsInactiveRelation(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	end := time.Now()
	contact.Relation.EndDate = &end
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.ErrorIs(t, err, domain.ErrUnsupportedSecurityCodeChannel)
	assert.Equal(t, 0, codes.generateCalls)
	assert.Equal(t, 0, sender.calls)
}

func TestRequestSecurityCodeRejectsContactFromAnotherParty(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, uuid.New())
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	codes := &fakeSecurityCodeManager{code: "123456"}
	sender := &fakeSecurityCodeSender{}
	service := newMFAServiceForTest(users, contacts, codes, sender)

	err := service.RequestSecurityCode(context.Background(), RequestSecurityCodeInput{
		TenantID: tenantID, PartyID: partyID, Action: "CHANGE_PASSWORD",
	})

	require.Error(t, err)
	assert.Equal(t, 0, codes.generateCalls)
	assert.Equal(t, 0, sender.calls)
}

type fakeTOTPManager struct {
	secret string
	url    string
	genErr error

	issuer  string
	account string

	valid           bool
	validatedSecret string
	validatedCode   string
}

func (m *fakeTOTPManager) GenerateTOTPSecret(issuer, account string) (string, string, error) {
	m.issuer, m.account = issuer, account
	return m.secret, m.url, m.genErr
}

func (m *fakeTOTPManager) ValidateTOTP(secret, code string) bool {
	m.validatedSecret, m.validatedCode = secret, code
	return m.valid
}

func TestSetupTOTPStoresSecretWithoutEnablingMFA(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	relationID := uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, PartyContactMechanismID: relationID,
	}}
	contact := codeCapableContact(tenantID, partyID)
	contact.Relation.ID = relationID
	contacts := &fakeMFAContactRepository{byID: map[uuid.UUID]*domain.PartyContact{relationID: contact}}
	totp := &fakeTOTPManager{secret: "JBSWY3DPEHPK3PXP", url: "otpauth://totp/enterprise-system:user@example.com?secret=JBSWY3DPEHPK3PXP"}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})
	service.SetTOTPManager(totp)

	result, err := service.SetupTOTP(context.Background(), tenantID, partyID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", result.Secret)
	assert.Equal(t, totp.url, result.OTPAuthURL)
	assert.Equal(t, "user@example.com", totp.account)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", users.updatedTOTPSecret)
	assert.Equal(t, 0, users.updateCalls, "setup must not enable MFA by itself")
	assert.Equal(t, 1, users.updateTOTPCalls)
}

func TestSetupTOTPRejectsAlreadyEnabledTOTP(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, MFAEnabled: true,
		MFAMethod: domain.MFAMethodTOTP, TOTPSecret: "EXISTING",
	}}
	totp := &fakeTOTPManager{secret: "NEWSECRET"}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})
	service.SetTOTPManager(totp)

	_, err := service.SetupTOTP(context.Background(), tenantID, partyID)

	require.ErrorIs(t, err, domain.ErrTOTPAlreadyEnabled)
	assert.Equal(t, 0, users.updateTOTPCalls)
}

func TestVerifyTOTPActivatesTOTPWhenCodeIsValid(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, TOTPSecret: "JBSWY3DPEHPK3PXP",
	}}
	totp := &fakeTOTPManager{valid: true}
	contacts := &fakeMFAContactRepository{contacts: []*domain.PartyContact{codeCapableContact(tenantID, partyID)}}
	service := newMFAServiceForTest(users, contacts, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})
	service.SetTOTPManager(totp)

	err := service.VerifyTOTP(context.Background(), tenantID, partyID, "123456")

	require.NoError(t, err)
	assert.Equal(t, "JBSWY3DPEHPK3PXP", totp.validatedSecret)
	assert.Equal(t, "123456", totp.validatedCode)
	assert.Equal(t, 1, users.activateCalls)
	assert.Equal(t, 0, users.updateCalls)
}

func TestVerifyTOTPRejectsInvalidCode(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{
		TenantID: tenantID, PartyID: partyID, TOTPSecret: "JBSWY3DPEHPK3PXP",
	}}
	totp := &fakeTOTPManager{valid: false}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})
	service.SetTOTPManager(totp)

	err := service.VerifyTOTP(context.Background(), tenantID, partyID, "000000")

	require.ErrorIs(t, err, auth.ErrInvalidTOTP)
	assert.Equal(t, 0, users.activateCalls)
}

func TestVerifyTOTPRequiresSetup(t *testing.T) {
	tenantID, partyID := uuid.New(), uuid.New()
	users := &fakeMFAUserRepository{credentials: &auth.Credentials{TenantID: tenantID, PartyID: partyID}}
	service := newMFAServiceForTest(users, &fakeMFAContactRepository{}, &fakeSecurityCodeManager{}, &fakeSecurityCodeSender{})
	service.SetTOTPManager(&fakeTOTPManager{valid: true})

	err := service.VerifyTOTP(context.Background(), tenantID, partyID, "123456")

	require.ErrorIs(t, err, domain.ErrTOTPNotConfigured)
	assert.Equal(t, 0, users.activateCalls)
}
