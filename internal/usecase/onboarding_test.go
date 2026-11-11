package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/infrastructure/enttenant"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOnboardingUoW is a fake TransactionManager; fakeOnboardingStore is the
// stateless persistence port. Together they replace the old unit of work.
type fakeOnboardingUoW struct {
	store      *fakeOnboardingStore
	rolledBack bool
	committed  bool
}

func newFakeOnboardingUoW() *fakeOnboardingUoW {
	return &fakeOnboardingUoW{store: newFakeOnboardingStore()}
}

func (u *fakeOnboardingUoW) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		u.rolledBack = true
		return err
	}
	u.committed = true
	return nil
}

func (u *fakeOnboardingUoW) WithinSavepoint(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeOnboardingStore struct {
	subscription    *domain.Subscription
	orgParty        *domain.Party
	orgProfile      *domain.OrganizationProfile
	tenantRole      *domain.PartyRole
	adminParty      *domain.Party
	personProfile   *domain.PersonProfile
	adminRole       *domain.PartyRole
	contact         *domain.ContactMechanism
	relation        *domain.PartyContactMechanism
	credentials     *auth.Credentials
	tenantIDs       []uuid.UUID
	subscriptionErr error
	credentialsErr  error
	tenantRoleID    uuid.UUID
	adminRoleID     uuid.UUID
}

func newFakeOnboardingStore() *fakeOnboardingStore {
	return &fakeOnboardingStore{
		tenantRoleID: uuid.New(), adminRoleID: uuid.New(),
	}
}

func requireTenant(ctx context.Context) error {
	sc, ok := enttenant.SessionContextFrom(ctx)
	if !ok || sc.TenantID == uuid.Nil {
		return errors.New("missing tenant context")
	}
	return nil
}

func (s *fakeOnboardingStore) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	if s.subscriptionErr != nil {
		return s.subscriptionErr
	}
	s.subscription = sub
	return nil
}

func (s *fakeOnboardingStore) FindRoleTypeID(ctx context.Context, code string) (uuid.UUID, error) {
	if err := requireTenant(ctx); err != nil {
		return uuid.Nil, err
	}
	switch code {
	case "TENANT":
		return s.tenantRoleID, nil
	case "ADMIN":
		return s.adminRoleID, nil
	default:
		return uuid.Nil, domain.ErrRoleTypeNotFound
	}
}

func (s *fakeOnboardingStore) CreateParty(ctx context.Context, p *domain.Party) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	if p.PartyType == domain.PartyTypeOrganization {
		s.orgParty = p
	} else {
		s.adminParty = p
	}
	return nil
}

func (s *fakeOnboardingStore) CreateOrganizationProfile(ctx context.Context, p *domain.OrganizationProfile) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	s.orgProfile = p
	return nil
}

func (s *fakeOnboardingStore) CreatePersonProfile(ctx context.Context, p *domain.PersonProfile) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	s.personProfile = p
	return nil
}

func (s *fakeOnboardingStore) CreatePartyRole(ctx context.Context, r *domain.PartyRole) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	if r.RoleTypeID == s.tenantRoleID {
		s.tenantRole = r
	} else {
		s.adminRole = r
	}
	return nil
}

func (s *fakeOnboardingStore) FindOrCreateContactMechanism(ctx context.Context, contactType domain.ContactType, value string, now time.Time) (*domain.ContactMechanism, error) {
	if err := requireTenant(ctx); err != nil {
		return nil, err
	}
	if s.contact == nil {
		contact, err := domain.NewContactMechanism(contactType, value)
		if err != nil {
			return nil, err
		}
		s.contact = contact
	}
	return s.contact, nil
}

func (s *fakeOnboardingStore) CreatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	s.relation = relation
	return nil
}

func (s *fakeOnboardingStore) CreateUserCredentials(ctx context.Context, c *auth.Credentials) error {
	if err := requireTenant(ctx); err != nil {
		return err
	}
	if s.credentialsErr != nil {
		return s.credentialsErr
	}
	sc, _ := enttenant.SessionContextFrom(ctx)
	s.tenantIDs = append(s.tenantIDs, sc.TenantID)
	s.credentials = c
	return nil
}

func onboardingTestManager(t *testing.T) *auth.Manager {
	t.Helper()
	m, err := auth.NewManager(auth.Config{
		Secret:     []byte("01234567890123456789012345678901"),
		Issuer:     "enterprise-system",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		BcryptCost: 4,
		Now:        func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) },
	})
	require.NoError(t, err)
	return m
}

func validRegisterInput() RegisterTenantInput {
	return RegisterTenantInput{
		CompanyName: "Acme Corp", TaxID: "J-123456",
		Slug:           "Acme Corp",
		AdminFirstName: "Jane", AdminLastName: "Doe",
		AdminEmail: "Jane@Example.com", AdminPassword: "password123",
	}
}

func TestRegisterTenantCreatesFullGraph(t *testing.T) {
	manager := onboardingTestManager(t)
	uow := newFakeOnboardingUoW()
	service := NewRegisterTenantService(manager, uow, uow.store, enttenant.ContextBinder{}, time.Now)

	result, err := service.RegisterTenant(context.Background(), validRegisterInput())

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, uow.committed)
	assert.False(t, uow.rolledBack)

	store := uow.store
	require.NotNil(t, store.subscription)
	require.NotNil(t, store.orgParty)
	require.NotNil(t, store.orgProfile)
	require.NotNil(t, store.tenantRole)
	require.NotNil(t, store.adminParty)
	require.NotNil(t, store.personProfile)
	require.NotNil(t, store.adminRole)
	require.NotNil(t, store.contact)
	require.NotNil(t, store.relation)
	require.NotNil(t, store.credentials)

	assert.Equal(t, domain.PartyTypeOrganization, store.orgParty.PartyType)
	assert.Equal(t, store.subscription.ID, store.orgParty.TenantID)
	assert.Equal(t, store.orgParty.ID, store.orgProfile.PartyID)
	assert.Equal(t, store.tenantRoleID, store.tenantRole.RoleTypeID)
	assert.Equal(t, domain.PartyTypeIndividual, store.adminParty.PartyType)
	assert.Equal(t, store.adminParty.ID, store.personProfile.PartyID)
	assert.Equal(t, store.adminRoleID, store.adminRole.RoleTypeID)
	assert.Equal(t, store.adminParty.ID, store.credentials.PartyID)
	assert.Equal(t, store.relation.ID, store.credentials.PartyContactMechanismID)
	assert.Equal(t, "jane@example.com", store.contact.Value)
	assert.True(t, manager.VerifyPassword(store.credentials.PasswordHash, "password123"))

	assert.Equal(t, "acme-corp", result.Slug)
	assert.Equal(t, store.subscription.ID, result.TenantID)
	assert.Equal(t, store.orgParty.ID, result.OrganizationPartyID)
	assert.Equal(t, store.adminParty.ID, result.AdminPartyID)
	assert.NotEmpty(t, store.tenantIDs)
	for _, tenantID := range store.tenantIDs {
		assert.Equal(t, store.subscription.ID, tenantID)
	}
}

func TestRegisterTenantRollsBackWhenCredentialsFail(t *testing.T) {
	manager := onboardingTestManager(t)
	uow := newFakeOnboardingUoW()
	uow.store.credentialsErr = domain.ErrTenantAdminEmailTaken
	service := NewRegisterTenantService(manager, uow, uow.store, enttenant.ContextBinder{}, time.Now)

	result, err := service.RegisterTenant(context.Background(), validRegisterInput())

	require.ErrorIs(t, err, domain.ErrTenantAdminEmailTaken)
	assert.Nil(t, result)
	assert.True(t, uow.rolledBack)
	assert.False(t, uow.committed)
	// The operations were staged but never committed; a real UoW rolls them back.
	require.NotNil(t, uow.store.subscription)
	require.NotNil(t, uow.store.orgParty)
	require.NotNil(t, uow.store.adminParty)
}

func TestRegisterTenantRejectsInvalidInput(t *testing.T) {
	uow := newFakeOnboardingUoW()
	service := NewRegisterTenantService(onboardingTestManager(t), uow, uow.store,
		enttenant.ContextBinder{}, time.Now)

	cases := []RegisterTenantInput{
		{},
		{CompanyName: "Acme", TaxID: "J-1", Slug: "acme", AdminFirstName: "Jane", AdminLastName: "Doe", AdminEmail: "not-an-email", AdminPassword: "password123"},
		{CompanyName: "Acme", TaxID: "", Slug: "acme", AdminFirstName: "Jane", AdminLastName: "Doe", AdminEmail: "jane@example.com", AdminPassword: "password123"},
	}
	for _, input := range cases {
		_, err := service.RegisterTenant(context.Background(), input)
		require.ErrorIs(t, err, domain.ErrInvalidTenantRegistration)
	}
}

func TestRegisterTenantRejectsDuplicateSlug(t *testing.T) {
	uow := newFakeOnboardingUoW()
	uow.store.subscriptionErr = domain.ErrTenantSlugTaken
	service := NewRegisterTenantService(onboardingTestManager(t), uow, uow.store,
		enttenant.ContextBinder{}, time.Now)

	_, err := service.RegisterTenant(context.Background(), validRegisterInput())

	require.ErrorIs(t, err, domain.ErrTenantSlugTaken)
	assert.True(t, uow.rolledBack)
	assert.False(t, uow.committed)
}
