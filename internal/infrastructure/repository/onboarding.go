package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/contactmechanism"
	entparty "enterprise-system/ent/party"
	"enterprise-system/ent/roletype"
	entsubscription "enterprise-system/ent/subscription"
	"enterprise-system/internal/auth"
	"enterprise-system/internal/domain"
	"enterprise-system/internal/usecase"
	"github.com/google/uuid"
)

var _ usecase.TenantProvisioningTx = (*TenantProvisioningStore)(nil)

// TenantProvisioningStore persists the onboarding graph. It is stateless and
// always uses the ambient transaction opened by TransactionManager.
type TenantProvisioningStore struct {
	client *ent.Client
}

func NewTenantProvisioningStore(client *ent.Client) *TenantProvisioningStore {
	return &TenantProvisioningStore{client: client}
}

func (s *TenantProvisioningStore) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if s == nil {
		return errNilClient
	}
	return execute(ctx, s.client, fn)
}

func (s *TenantProvisioningStore) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	if sub == nil {
		return errors.New("provisioning: nil subscription")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		_, err := client.Subscription.Create().
			SetID(sub.ID).
			SetName(sub.Name).
			SetSlug(sub.Slug).
			SetStatus(entsubscription.Status(sub.Status)).
			SetCreatedAt(sub.CreatedAt).
			SetUpdatedAt(sub.UpdatedAt).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return domain.ErrTenantSlugTaken
		}
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) FindRoleTypeID(ctx context.Context, code string) (uuid.UUID, error) {
	var result uuid.UUID
	err := s.withClient(ctx, func(client *ent.Client) error {
		row, err := client.RoleType.Query().
			Where(roletype.Code(code), roletype.DeletedAtIsNil()).
			Only(ctx)
		if ent.IsNotFound(err) {
			return domain.ErrRoleTypeNotFound
		}
		if err != nil {
			return repoError(err)
		}
		result = row.ID
		return nil
	})
	return result, err
}

func (s *TenantProvisioningStore) CreateParty(ctx context.Context, p *domain.Party) error {
	if p == nil {
		return errors.New("provisioning: nil party")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		_, err := client.Party.Create().
			SetID(p.ID).
			SetTenantID(p.TenantID).
			SetPartyType(entparty.PartyType(p.PartyType)).
			SetDisplayName(p.DisplayName).
			SetRecordStatus(entparty.RecordStatus(p.RecordStatus)).
			SetCreatedAt(p.CreatedAt).
			SetUpdatedAt(p.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) CreateOrganizationProfile(ctx context.Context, p *domain.OrganizationProfile) error {
	if p == nil {
		return errors.New("provisioning: nil organization profile")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		create := client.OrganizationProfile.Create().
			SetID(p.ID).
			SetTenantID(p.TenantID).
			SetPartyID(p.PartyID).
			SetLegalName(p.LegalName).
			SetTaxID(p.TaxID).
			SetCreatedAt(p.CreatedAt).
			SetUpdatedAt(p.UpdatedAt)
		if p.TradeName != "" {
			create.SetTradeName(p.TradeName)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) CreatePersonProfile(ctx context.Context, p *domain.PersonProfile) error {
	if p == nil {
		return errors.New("provisioning: nil person profile")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		create := client.PersonProfile.Create().
			SetID(p.ID).
			SetTenantID(p.TenantID).
			SetPartyID(p.PartyID).
			SetFirstName(p.FirstName).
			SetLastName(p.LastName).
			SetCreatedAt(p.CreatedAt).
			SetUpdatedAt(p.UpdatedAt)
		if p.MiddleName != "" {
			create.SetMiddleName(p.MiddleName)
		}
		if p.SecondLastName != "" {
			create.SetSecondLastName(p.SecondLastName)
		}
		if p.BirthDate != nil {
			create.SetBirthDate(*p.BirthDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) CreatePartyRole(ctx context.Context, role *domain.PartyRole) error {
	if role == nil {
		return errors.New("provisioning: nil party role")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		_, err := client.PartyRole.Create().
			SetID(role.ID).
			SetTenantID(role.TenantID).
			SetPartyID(role.PartyID).
			SetRoleTypeID(role.RoleTypeID).
			SetCreatedAt(role.CreatedAt).
			SetUpdatedAt(role.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) FindOrCreateContactMechanism(ctx context.Context, contactType domain.ContactType, value string, now time.Time) (*domain.ContactMechanism, error) {
	contact, err := domain.NewContactMechanism(contactType, value)
	if err != nil {
		return nil, err
	}
	var result *domain.ContactMechanism
	err = s.withClient(ctx, func(client *ent.Client) error {
		err := client.ContactMechanism.Create().
			SetID(contact.ID).
			SetType(contactmechanism.Type(contact.Type)).
			SetValue(contact.Value).
			OnConflictColumns(contactmechanism.FieldType, contactmechanism.FieldValue).
			DoNothing().
			Exec(ctx)
		if err != nil {
			return repoError(err)
		}
		row, err := client.ContactMechanism.Query().
			Where(contactmechanism.TypeEQ(contactmechanism.Type(contactType)), contactmechanism.Value(value)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.ContactMechanism{ID: row.ID, Type: domain.ContactType(row.Type), Value: row.Value}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *TenantProvisioningStore) CreatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if relation == nil {
		return errors.New("provisioning: nil party contact mechanism")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		create := client.PartyContactMechanism.Create().
			SetID(relation.ID).
			SetTenantID(relation.TenantID).
			SetPartyID(relation.PartyID).
			SetContactMechanismID(relation.ContactMechanismID).
			SetIsPrimary(relation.IsPrimary).
			SetIsVerified(relation.IsVerified).
			SetContactMechanismOrder(relation.ContactMechanismOrder).
			SetStartDate(relation.StartDate).
			SetCreatedAt(relation.CreatedAt).
			SetUpdatedAt(relation.UpdatedAt)
		if relation.EndDate != nil {
			create.SetEndDate(*relation.EndDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (s *TenantProvisioningStore) CreateUserCredentials(ctx context.Context, c *auth.Credentials) error {
	if c == nil {
		return errors.New("provisioning: nil credentials")
	}
	return s.withClient(ctx, func(client *ent.Client) error {
		_, err := client.UserCredentials.Create().
			SetID(c.ID).
			SetTenantID(c.TenantID).
			SetPartyID(c.PartyID).
			SetPartyContactMechanismID(c.PartyContactMechanismID).
			SetPasswordHash(c.PasswordHash).
			SetTotpEnabled(c.TOTPEnabled).
			SetBackupCodesHash(c.BackupCodesHash).
			SetCreatedAt(c.CreatedAt).
			SetUpdatedAt(c.UpdatedAt).
			Save(ctx)
		if ent.IsConstraintError(err) {
			return domain.ErrTenantAdminEmailTaken
		}
		return repoError(err)
	})
}
