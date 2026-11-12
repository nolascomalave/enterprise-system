package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/nametype"
	"enterprise-system/ent/partyname"
	"enterprise-system/ent/partynamevalidity"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.PartyNameRepository = (*PartyNameRepository)(nil)

type PartyNameRepository struct {
	client *ent.Client
}

func NewPartyNameRepository(client *ent.Client) *PartyNameRepository {
	return &PartyNameRepository{client: client}
}

func (r *PartyNameRepository) FindNameTypeByCode(ctx context.Context, code string) (*domain.NameType, error) {
	var result *domain.NameType
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.NameType.Query().
			Where(nametype.CodeEQ(code)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.NameType{ID: row.ID, Code: row.Code, Name: row.Name, System: row.IsSystem}
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrNameTypeNotFound
	}
	return result, err
}

func (r *PartyNameRepository) FindPartyName(
	ctx context.Context,
	tenantID, partyID, nameTypeID uuid.UUID,
	value string,
) (*domain.PartyName, error) {
	var result *domain.PartyName
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyName.Query().
			Where(
				partyname.TenantID(tenantID),
				partyname.PartyID(partyID),
				partyname.NameTypeID(nameTypeID),
				partyname.ValueEQ(value),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainPartyName(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyNameNotFound
	}
	return result, err
}

func (r *PartyNameRepository) GetPartyName(ctx context.Context, tenantID, nameID uuid.UUID) (*domain.PartyName, error) {
	var result *domain.PartyName
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyName.Query().
			Where(
				partyname.ID(nameID),
				partyname.TenantID(tenantID),
				partyname.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainPartyName(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyNameNotFound
	}
	return result, err
}

func (r *PartyNameRepository) CreatePartyName(ctx context.Context, name *domain.PartyName) error {
	if name == nil {
		return errors.New("repository: nil party name")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.PartyName.Create().
			SetID(name.ID).
			SetTenantID(name.TenantID).
			SetPartyID(name.PartyID).
			SetNameTypeID(name.NameTypeID).
			SetValue(name.Value).
			SetIsPrimary(name.IsPrimary).
			SetOrdering(name.Ordering).
			SetCreatedAt(name.CreatedAt).
			SetUpdatedAt(name.UpdatedAt)
		if name.DeletedAt != nil {
			create.SetDeletedAt(*name.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *PartyNameRepository) UpdatePartyName(ctx context.Context, name *domain.PartyName) error {
	if name == nil {
		return errors.New("repository: nil party name")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		update := client.PartyName.UpdateOneID(name.ID).
			SetValue(name.Value).
			SetIsPrimary(name.IsPrimary).
			SetOrdering(name.Ordering).
			SetUpdatedAt(name.UpdatedAt)
		if name.DeletedAt != nil {
			update.SetDeletedAt(*name.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *PartyNameRepository) GetOpenValidity(
	ctx context.Context,
	tenantID, partyNameID uuid.UUID,
) (*domain.PartyNameValidity, error) {
	var result *domain.PartyNameValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyNameValidity.Query().
			Where(
				partynamevalidity.TenantID(tenantID),
				partynamevalidity.PartyNameID(partyNameID),
				partynamevalidity.EndDateIsNil(),
			).
			Order(ent.Desc(partynamevalidity.FieldStartDate)).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainPartyNameValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyNameValidityNotFound
	}
	return result, err
}

func (r *PartyNameRepository) GetOpenPrimaryValidity(
	ctx context.Context,
	tenantID, partyID, nameTypeID uuid.UUID,
) (*domain.PartyNameValidity, error) {
	var result *domain.PartyNameValidity
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.PartyNameValidity.Query().
			Where(
				partynamevalidity.TenantID(tenantID),
				partynamevalidity.PartyID(partyID),
				partynamevalidity.NameTypeID(nameTypeID),
				partynamevalidity.IsPrimary(true),
				partynamevalidity.EndDateIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainPartyNameValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyNameValidityNotFound
	}
	return result, err
}

func (r *PartyNameRepository) CreateValidity(ctx context.Context, validity *domain.PartyNameValidity) error {
	if validity == nil {
		return errors.New("repository: nil party name validity")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.PartyNameValidity.Create().
			SetID(validity.ID).
			SetTenantID(validity.TenantID).
			SetPartyNameID(validity.PartyNameID).
			SetPartyID(validity.PartyID).
			SetNameTypeID(validity.NameTypeID).
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

func (r *PartyNameRepository) CloseValidity(
	ctx context.Context,
	tenantID, validityID uuid.UUID,
	at time.Time,
) error {
	var affected int
	err := execute(ctx, r.client, func(client *ent.Client) error {
		var err error
		affected, err = client.PartyNameValidity.Update().
			Where(
				partynamevalidity.ID(validityID),
				partynamevalidity.TenantID(tenantID),
				partynamevalidity.EndDateIsNil(),
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
		return domain.ErrPartyNameValidityNotFound
	}
	return nil
}

func toDomainPartyName(e *ent.PartyName) *domain.PartyName {
	return &domain.PartyName{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, NameTypeID: e.NameTypeID,
		Value: e.Value, IsPrimary: e.IsPrimary, Ordering: e.Ordering,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainPartyNameValidity(e *ent.PartyNameValidity) *domain.PartyNameValidity {
	return &domain.PartyNameValidity{
		ID: e.ID, TenantID: e.TenantID, PartyNameID: e.PartyNameID,
		PartyID: e.PartyID, NameTypeID: e.NameTypeID, IsPrimary: e.IsPrimary,
		StartDate: e.StartDate, EndDate: e.EndDate,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
