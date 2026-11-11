package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

type TenantProvisioningTx interface {
	CreateSubscription(ctx context.Context, subscription *domain.Subscription) error
	FindRoleTypeID(ctx context.Context, code string) (uuid.UUID, error)
	CreateParty(ctx context.Context, party *domain.Party) error
	CreateOrganizationProfile(ctx context.Context, profile *domain.OrganizationProfile) error
	CreatePersonProfile(ctx context.Context, profile *domain.PersonProfile) error
	CreatePartyRole(ctx context.Context, role *domain.PartyRole) error
	FindOrCreateContactMechanism(ctx context.Context, contactType domain.ContactType, value string, now time.Time) (*domain.ContactMechanism, error)
	CreatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error
	CreateUserCredentials(ctx context.Context, credentials *auth.Credentials) error
}

type RegisterTenantInput struct {
	CompanyName    string
	TaxID          string
	Slug           string
	AdminFirstName string
	AdminLastName  string
	AdminEmail     string
	AdminPassword  string
}

type RegisterTenantResult struct {
	TenantID            uuid.UUID
	OrganizationPartyID uuid.UUID
	AdminPartyID        uuid.UUID
	Slug                string
}

type RegisterTenantService struct {
	manager *auth.Manager
	tx      TransactionManager
	store   TenantProvisioningTx
	binder  TenantContextBinder
	now     func() time.Time
}

func NewRegisterTenantService(
	manager *auth.Manager,
	tx TransactionManager,
	store TenantProvisioningTx,
	binder TenantContextBinder,
	now func() time.Time,
) *RegisterTenantService {
	if now == nil {
		now = time.Now
	}
	return &RegisterTenantService{manager: manager, tx: tx, store: store, binder: binder, now: now}
}

func (s *RegisterTenantService) RegisterTenant(ctx context.Context, in RegisterTenantInput) (*RegisterTenantResult, error) {
	if s == nil || s.tx == nil || s.store == nil {
		return nil, internalError(errors.New("register tenant: nil dependencies"))
	}
	companyName := strings.TrimSpace(in.CompanyName)
	taxID := strings.TrimSpace(in.TaxID)
	adminFirstName := strings.TrimSpace(in.AdminFirstName)
	adminLastName := strings.TrimSpace(in.AdminLastName)
	adminEmail := strings.ToLower(strings.TrimSpace(in.AdminEmail))
	if companyName == "" || taxID == "" || adminFirstName == "" || adminLastName == "" || adminEmail == "" || in.AdminPassword == "" {
		return nil, validationError("invalid_tenant_registration",
			"company, tax_id, admin name/email and password are required", domain.ErrInvalidTenantRegistration)
	}
	parsedEmail, err := mail.ParseAddress(adminEmail)
	if err != nil || !strings.EqualFold(parsedEmail.Address, adminEmail) {
		return nil, validationError("invalid_tenant_registration", "invalid admin email",
			errors.Join(domain.ErrInvalidTenantRegistration, err))
	}

	now := s.now().UTC()
	subscription, err := domain.NewSubscription(companyName, in.Slug, now)
	if err != nil {
		return nil, validationError("invalid_tenant_registration", "invalid tenant registration", err)
	}
	tenantID := subscription.ID

	organizationParty, err := domain.NewParty(tenantID, domain.PartyTypeOrganization, companyName, now)
	if err != nil {
		return nil, validationError("invalid_tenant_registration", "invalid organization party", err)
	}
	organizationProfile, err := domain.NewOrganizationProfile(tenantID, organizationParty.ID, companyName, "", taxID, now)
	if err != nil {
		return nil, validationError("invalid_tenant_registration", "invalid organization profile", err)
	}
	adminParty, err := domain.NewParty(tenantID, domain.PartyTypeIndividual, adminFirstName+" "+adminLastName, now)
	if err != nil {
		return nil, validationError("invalid_tenant_registration", "invalid admin party", err)
	}
	personProfile, err := domain.NewPersonProfile(tenantID, adminParty.ID, adminFirstName, "", adminLastName, "", nil, now)
	if err != nil {
		return nil, validationError("invalid_tenant_registration", "invalid admin profile", err)
	}
	passwordHash, err := s.manager.HashPassword(in.AdminPassword)
	if err != nil {
		return nil, internalError(fmt.Errorf("hash admin password: %w", err))
	}
	credentialsID, err := uuid.NewV7()
	if err != nil {
		return nil, internalError(fmt.Errorf("generate credentials id: %w", err))
	}

	var result *RegisterTenantResult
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateSubscription(txCtx, subscription); err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}

		// The subscription is global and has no RLS. Bind the new tenant before
		// the first tenant-scoped write; sessionTx applies SET LOCAL lazily.
		tenantCtx := s.binder.BindTenant(txCtx, tenantID, uuid.Nil, uuid.Nil)

		if err := s.store.CreateParty(tenantCtx, organizationParty); err != nil {
			return fmt.Errorf("create organization party: %w", err)
		}
		if err := s.store.CreateOrganizationProfile(tenantCtx, organizationProfile); err != nil {
			return fmt.Errorf("create organization profile: %w", err)
		}
		tenantRoleID, err := s.store.FindRoleTypeID(tenantCtx, "TENANT")
		if err != nil {
			return fmt.Errorf("find TENANT role: %w", err)
		}
		tenantRole, err := domain.NewPartyRole(tenantID, organizationParty.ID, tenantRoleID, now)
		if err != nil {
			return validationError("invalid_tenant_registration", "invalid tenant role", err)
		}
		if err := s.store.CreatePartyRole(tenantCtx, tenantRole); err != nil {
			return fmt.Errorf("assign TENANT role: %w", err)
		}

		adminCtx := s.binder.BindTenant(txCtx, tenantID, adminParty.ID, uuid.Nil)
		if err := s.store.CreateParty(adminCtx, adminParty); err != nil {
			return fmt.Errorf("create admin party: %w", err)
		}
		if err := s.store.CreatePersonProfile(adminCtx, personProfile); err != nil {
			return fmt.Errorf("create person profile: %w", err)
		}
		adminRoleID, err := s.store.FindRoleTypeID(adminCtx, "ADMIN")
		if err != nil {
			return fmt.Errorf("find ADMIN role: %w", err)
		}
		adminRole, err := domain.NewPartyRole(tenantID, adminParty.ID, adminRoleID, now)
		if err != nil {
			return validationError("invalid_tenant_registration", "invalid admin role", err)
		}
		if err := s.store.CreatePartyRole(adminCtx, adminRole); err != nil {
			return fmt.Errorf("assign ADMIN role: %w", err)
		}

		contact, err := s.store.FindOrCreateContactMechanism(adminCtx, domain.ContactTypeEmail, adminEmail, now)
		if err != nil {
			return fmt.Errorf("find or create admin email: %w", err)
		}
		relation, err := domain.NewPartyContactMechanism(
			tenantID, adminParty.ID, contact.ID, true, false, 1, now, nil, now,
		)
		if err != nil {
			return validationError("invalid_tenant_registration", "invalid admin contact relation", err)
		}
		if err := s.store.CreatePartyContactMechanism(adminCtx, relation); err != nil {
			return fmt.Errorf("create admin contact relation: %w", err)
		}

		credentials := &auth.Credentials{
			ID: credentialsID, TenantID: tenantID, PartyID: adminParty.ID,
			PartyContactMechanismID: relation.ID, PasswordHash: passwordHash,
			BackupCodesHash: []string{}, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.store.CreateUserCredentials(adminCtx, credentials); err != nil {
			return fmt.Errorf("create admin credentials: %w", err)
		}

		result = &RegisterTenantResult{
			TenantID: tenantID, OrganizationPartyID: organizationParty.ID,
			AdminPartyID: adminParty.ID, Slug: subscription.Slug,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
