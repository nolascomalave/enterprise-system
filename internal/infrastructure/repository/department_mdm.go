package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/contactmechanism"
	"enterprise-system/ent/departmentaddress"
	"enterprise-system/ent/departmentaddressvalidity"
	"enterprise-system/ent/departmentcontactmechanism"
	"enterprise-system/ent/departmentcontactmechanismvalidity"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.DepartmentMDMRepository = (*DepartmentMDMRepository)(nil)

type DepartmentMDMRepository struct {
	client *ent.Client
}

func NewDepartmentMDMRepository(client *ent.Client) *DepartmentMDMRepository {
	return &DepartmentMDMRepository{client: client}
}

func (r *DepartmentMDMRepository) FindOrCreateContactMechanism(
	ctx context.Context,
	contact *domain.ContactMechanism,
) (*domain.ContactMechanism, error) {
	if contact == nil {
		return nil, errors.New("repository: nil contact mechanism")
	}
	var result *domain.ContactMechanism
	err := execute(ctx, r.client, func(client *ent.Client) error {
		if err := client.ContactMechanism.Create().
			SetID(contact.ID).
			SetType(contactmechanism.Type(contact.Type)).
			SetValue(contact.Value).
			OnConflictColumns(contactmechanism.FieldType, contactmechanism.FieldValue).
			DoNothing().
			Exec(ctx); err != nil {
			return repoError(err)
		}
		row, err := client.ContactMechanism.Query().
			Where(
				contactmechanism.TypeEQ(contactmechanism.Type(contact.Type)),
				contactmechanism.Value(contact.Value),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.ContactMechanism{ID: row.ID, Type: domain.ContactType(row.Type), Value: row.Value}
		return nil
	})
	return result, err
}

func (r *DepartmentMDMRepository) CreateDepartmentContact(
	ctx context.Context,
	contact *domain.DepartmentContactMechanism,
) error {
	if contact == nil {
		return errors.New("repository: nil department contact")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		_, err := client.DepartmentContactMechanism.Create().
			SetID(contact.ID).
			SetTenantID(contact.TenantID).
			SetDepartmentID(contact.DepartmentID).
			SetContactMechanismID(contact.ContactMechanismID).
			SetContactMechanismOrder(contact.ContactMechanismOrder).
			SetIsPrimary(contact.IsPrimary).
			SetCreatedAt(contact.CreatedAt).
			SetUpdatedAt(contact.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *DepartmentMDMRepository) GetDepartmentContact(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (*domain.DepartmentContactMechanism, error) {
	var result *domain.DepartmentContactMechanism
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentContactMechanism.Query().
			Where(
				departmentcontactmechanism.ID(id),
				departmentcontactmechanism.TenantID(tenantID),
				departmentcontactmechanism.DeletedAtIsNil(),
			).
			WithContactMechanism().
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.DepartmentContactMechanism{
			ID: row.ID, TenantID: row.TenantID, DepartmentID: row.DepartmentID,
			ContactMechanismID:    row.ContactMechanismID,
			ContactType:           domain.ContactType(row.Edges.ContactMechanism.Type),
			ContactMechanismOrder: row.ContactMechanismOrder,
			IsPrimary:             row.IsPrimary,
			CreatedAt:             row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentContactNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) GetOpenDepartmentContactValidity(
	ctx context.Context,
	tenantID, contactID uuid.UUID,
) (*domain.DepartmentContactMechanismValidity, error) {
	var result *domain.DepartmentContactMechanismValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentContactMechanismValidity.Query().
			Where(
				departmentcontactmechanismvalidity.TenantID(tenantID),
				departmentcontactmechanismvalidity.DepartmentContactMechanismID(contactID),
				departmentcontactmechanismvalidity.EndDateIsNil(),
			).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartmentContactValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentContactNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) GetOpenPrimaryDepartmentContactValidity(
	ctx context.Context,
	tenantID, departmentID uuid.UUID,
	contactType domain.ContactType,
) (*domain.DepartmentContactMechanismValidity, error) {
	var result *domain.DepartmentContactMechanismValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentContactMechanismValidity.Query().
			Where(
				departmentcontactmechanismvalidity.TenantID(tenantID),
				departmentcontactmechanismvalidity.DepartmentID(departmentID),
				departmentcontactmechanismvalidity.ContactTypeEQ(
					departmentcontactmechanismvalidity.ContactType(contactType),
				),
				departmentcontactmechanismvalidity.IsPrimary(true),
				departmentcontactmechanismvalidity.EndDateIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartmentContactValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentContactNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) CreateDepartmentContactValidity(
	ctx context.Context,
	validity *domain.DepartmentContactMechanismValidity,
) error {
	if validity == nil {
		return errors.New("repository: nil department contact validity")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.DepartmentContactMechanismValidity.Create().
			SetID(validity.ID).
			SetTenantID(validity.TenantID).
			SetDepartmentContactMechanismID(validity.DepartmentContactMechanismID).
			SetDepartmentID(validity.DepartmentID).
			SetContactType(departmentcontactmechanismvalidity.ContactType(validity.ContactType)).
			SetIsPrimary(validity.IsPrimary).
			SetStartDate(validity.StartDate).
			SetCreatedAt(validity.CreatedAt).
			SetUpdatedAt(validity.UpdatedAt)
		if validity.EndDate != nil {
			create.SetEndDate(*validity.EndDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *DepartmentMDMRepository) CloseDepartmentContactValidity(
	ctx context.Context,
	tenantID, validityID uuid.UUID,
	at time.Time,
) error {
	var affected int
	err := execute(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.DepartmentContactMechanismValidity.Update().
			Where(
				departmentcontactmechanismvalidity.ID(validityID),
				departmentcontactmechanismvalidity.TenantID(tenantID),
				departmentcontactmechanismvalidity.EndDateIsNil(),
			).
			SetEndDate(at).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrDepartmentContactNotFound
	}
	return nil
}

func (r *DepartmentMDMRepository) CreateDepartmentAddress(
	ctx context.Context,
	address *domain.DepartmentAddress,
) error {
	if address == nil {
		return errors.New("repository: nil department address")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.DepartmentAddress.Create().
			SetID(address.ID).
			SetTenantID(address.TenantID).
			SetDepartmentID(address.DepartmentID).
			SetCityLocationID(address.CityLocationID).
			SetType(departmentaddress.Type(address.Type)).
			SetLine1(address.Line1).
			SetCreatedAt(address.CreatedAt).
			SetUpdatedAt(address.UpdatedAt)
		if address.Line2 != "" {
			create.SetLine2(address.Line2)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *DepartmentMDMRepository) GetDepartmentAddress(
	ctx context.Context,
	tenantID, id uuid.UUID,
) (*domain.DepartmentAddress, error) {
	var result *domain.DepartmentAddress
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentAddress.Query().
			Where(
				departmentaddress.ID(id),
				departmentaddress.TenantID(tenantID),
				departmentaddress.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartmentAddress(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentAddressNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) GetOpenDepartmentAddressValidity(
	ctx context.Context,
	tenantID, addressID uuid.UUID,
) (*domain.DepartmentAddressValidity, error) {
	var result *domain.DepartmentAddressValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentAddressValidity.Query().
			Where(
				departmentaddressvalidity.TenantID(tenantID),
				departmentaddressvalidity.DepartmentAddressID(addressID),
				departmentaddressvalidity.EndDateIsNil(),
			).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartmentAddressValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentAddressNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) GetOpenPrimaryDepartmentAddressValidity(
	ctx context.Context,
	tenantID, departmentID uuid.UUID,
	addressType domain.AddressType,
) (*domain.DepartmentAddressValidity, error) {
	var result *domain.DepartmentAddressValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.DepartmentAddressValidity.Query().
			Where(
				departmentaddressvalidity.TenantID(tenantID),
				departmentaddressvalidity.DepartmentID(departmentID),
				departmentaddressvalidity.TypeEQ(departmentaddressvalidity.Type(addressType)),
				departmentaddressvalidity.IsPrimary(true),
				departmentaddressvalidity.EndDateIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainDepartmentAddressValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrDepartmentAddressNotFound
	}
	return result, err
}

func (r *DepartmentMDMRepository) CreateDepartmentAddressValidity(
	ctx context.Context,
	validity *domain.DepartmentAddressValidity,
) error {
	if validity == nil {
		return errors.New("repository: nil department address validity")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.DepartmentAddressValidity.Create().
			SetID(validity.ID).
			SetTenantID(validity.TenantID).
			SetDepartmentAddressID(validity.DepartmentAddressID).
			SetDepartmentID(validity.DepartmentID).
			SetType(departmentaddressvalidity.Type(validity.Type)).
			SetIsPrimary(validity.IsPrimary).
			SetStartDate(validity.StartDate).
			SetCreatedAt(validity.CreatedAt).
			SetUpdatedAt(validity.UpdatedAt)
		if validity.EndDate != nil {
			create.SetEndDate(*validity.EndDate)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *DepartmentMDMRepository) CloseDepartmentAddressValidity(
	ctx context.Context,
	tenantID, validityID uuid.UUID,
	at time.Time,
) error {
	var affected int
	err := execute(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.DepartmentAddressValidity.Update().
			Where(
				departmentaddressvalidity.ID(validityID),
				departmentaddressvalidity.TenantID(tenantID),
				departmentaddressvalidity.EndDateIsNil(),
			).
			SetEndDate(at).
			SetUpdatedAt(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrDepartmentAddressNotFound
	}
	return nil
}

func toDomainDepartmentContactValidity(e *ent.DepartmentContactMechanismValidity) *domain.DepartmentContactMechanismValidity {
	return &domain.DepartmentContactMechanismValidity{
		ID: e.ID, TenantID: e.TenantID,
		DepartmentContactMechanismID: e.DepartmentContactMechanismID,
		DepartmentID:                 e.DepartmentID,
		ContactType:                  domain.ContactType(e.ContactType),
		IsPrimary:                    e.IsPrimary,
		StartDate:                    e.StartDate, EndDate: e.EndDate,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func toDomainDepartmentAddress(e *ent.DepartmentAddress) *domain.DepartmentAddress {
	return &domain.DepartmentAddress{
		ID: e.ID, TenantID: e.TenantID, DepartmentID: e.DepartmentID,
		CityLocationID: e.CityLocationID, Type: domain.AddressType(e.Type),
		Line1: e.Line1, Line2: e.Line2,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainDepartmentAddressValidity(e *ent.DepartmentAddressValidity) *domain.DepartmentAddressValidity {
	return &domain.DepartmentAddressValidity{
		ID: e.ID, TenantID: e.TenantID,
		DepartmentAddressID: e.DepartmentAddressID,
		DepartmentID:        e.DepartmentID,
		Type:                domain.AddressType(e.Type),
		IsPrimary:           e.IsPrimary,
		StartDate:           e.StartDate, EndDate: e.EndDate,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
