package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/address"
	"enterprise-system/ent/addressvalidity"
	"enterprise-system/ent/contactmechanism"
	"enterprise-system/ent/partycontactmechanism"
	"enterprise-system/ent/partycontactmechanismvalidity"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.PartyContactRepository = (*PartyContactRepository)(nil)

type PartyContactRepository struct {
	client *ent.Client
}

func NewPartyContactRepository(client *ent.Client) *PartyContactRepository {
	return &PartyContactRepository{client: client}
}

func (r *PartyContactRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

func (r *PartyContactRepository) FindContactMechanismByTypeValue(ctx context.Context, contactType domain.ContactType, value string) (*domain.ContactMechanism, error) {
	var result *domain.ContactMechanism
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.ContactMechanism.Query().
			Where(contactmechanism.TypeEQ(contactmechanism.Type(contactType)), contactmechanism.Value(value)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainContactMechanism(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrContactMechanismNotFound
	}
	return result, err
}

func (r *PartyContactRepository) FindOrCreateContactMechanism(ctx context.Context, contact *domain.ContactMechanism) (*domain.ContactMechanism, error) {
	if contact == nil {
		return nil, errors.New("repository: nil contact mechanism")
	}
	var result *domain.ContactMechanism
	err := r.withClient(ctx, func(client *ent.Client) error {
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
			Where(contactmechanism.TypeEQ(contactmechanism.Type(contact.Type)), contactmechanism.Value(contact.Value)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainContactMechanism(row)
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) CreatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if relation == nil {
		return errors.New("repository: nil party contact mechanism")
	}
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.PartyContactMechanism.Create().
			SetID(relation.ID).
			SetTenantID(relation.TenantID).
			SetPartyID(relation.PartyID).
			SetContactMechanismID(relation.ContactMechanismID).
			SetIsPrimary(relation.IsPrimary).
			SetIsVerified(relation.IsVerified).
			SetContactMechanismOrder(relation.ContactMechanismOrder).
			SetCreatedAt(relation.CreatedAt).
			SetUpdatedAt(relation.UpdatedAt)
		if _, err := create.Save(ctx); err != nil {
			return repoError(err)
		}

		contact, err := client.ContactMechanism.Get(ctx, relation.ContactMechanismID)
		if err != nil {
			return repoError(err)
		}
		validityID, err := uuid.NewV7()
		if err != nil {
			return repoError(err)
		}
		validity := client.PartyContactMechanismValidity.Create().
			SetID(validityID).
			SetTenantID(relation.TenantID).
			SetPartyContactMechanismID(relation.ID).
			SetPartyID(relation.PartyID).
			SetContactMechanismID(relation.ContactMechanismID).
			SetContactType(partycontactmechanismvalidity.ContactType(contact.Type)).
			SetIsPrimary(relation.IsPrimary).
			SetStartDate(relation.StartDate).
			SetCreatedAt(relation.CreatedAt).
			SetUpdatedAt(relation.UpdatedAt)
		if relation.EndDate != nil {
			validity.SetEndDate(*relation.EndDate)
		}
		_, err = validity.Save(ctx)
		return repoError(err)
	})
}

func (r *PartyContactRepository) GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*domain.PartyContact, error) {
	var result *domain.PartyContact
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.PartyContactMechanism.Query().
			Where(partycontactmechanism.ID(relationID), partycontactmechanism.TenantID(tenantID)).
			WithContactMechanism().
			WithValidities(func(q *ent.PartyContactMechanismValidityQuery) {
				q.Where(partycontactmechanismvalidity.EndDateIsNil()).
					Order(ent.Desc(partycontactmechanismvalidity.FieldStartDate))
			}).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = &domain.PartyContact{
			Relation: toDomainPartyContactMechanism(row, firstValidity(row.Edges.Validities)),
			Contact:  toDomainContactMechanism(row.Edges.ContactMechanism),
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyContactNotFound
	}
	return result, err
}

func (r *PartyContactRepository) ListPartyContacts(ctx context.Context, tenantID, partyID uuid.UUID) ([]*domain.PartyContact, error) {
	var result []*domain.PartyContact
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.PartyContactMechanism.Query().
			Where(
				partycontactmechanism.TenantID(tenantID),
				partycontactmechanism.PartyID(partyID),
				partycontactmechanism.HasValiditiesWith(
					partycontactmechanismvalidity.EndDateIsNil(),
				),
			).
			WithContactMechanism().
			WithValidities(func(q *ent.PartyContactMechanismValidityQuery) {
				q.Where(partycontactmechanismvalidity.EndDateIsNil())
			}).
			Order(ent.Asc(partycontactmechanism.FieldContactMechanismOrder)).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.PartyContact, 0, len(rows))
		for _, row := range rows {
			result = append(result, &domain.PartyContact{
				Relation: toDomainPartyContactMechanism(row, firstValidity(row.Edges.Validities)),
				Contact:  toDomainContactMechanism(row.Edges.ContactMechanism),
			})
		}
		sort.SliceStable(result, func(i, j int) bool {
			left, right := result[i].Relation, result[j].Relation
			if left.ContactMechanismOrder != right.ContactMechanismOrder {
				return left.ContactMechanismOrder < right.ContactMechanismOrder
			}
			return left.StartDate.Before(right.StartDate)
		})
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) UpdatePartyContactMechanism(ctx context.Context, relation *domain.PartyContactMechanism) error {
	if relation == nil {
		return errors.New("repository: nil party contact mechanism")
	}
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.PartyContactMechanism.UpdateOneID(relation.ID).
			SetIsPrimary(relation.IsPrimary).
			SetIsVerified(relation.IsVerified).
			SetContactMechanismOrder(relation.ContactMechanismOrder).
			SetUpdatedAt(relation.UpdatedAt)
		if _, err := update.Save(ctx); err != nil {
			return repoError(err)
		}
		if relation.EndDate != nil {
			_, err := client.PartyContactMechanismValidity.Update().
				Where(
					partycontactmechanismvalidity.TenantID(relation.TenantID),
					partycontactmechanismvalidity.PartyContactMechanismID(relation.ID),
					partycontactmechanismvalidity.EndDateIsNil(),
				).
				SetEndDate(*relation.EndDate).
				SetUpdatedAt(relation.UpdatedAt).
				Save(ctx)
			if err != nil {
				return repoError(err)
			}
		}
		return nil
	})
}

func (r *PartyContactRepository) FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*domain.PartyContactMechanism, error) {
	var result *domain.PartyContactMechanism
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.PartyContactMechanism.Query().
			Where(
				partycontactmechanism.TenantID(tenantID),
				partycontactmechanism.ContactMechanismID(contactMechanismID),
				partycontactmechanism.HasValiditiesWith(
					partycontactmechanismvalidity.StartDateLTE(at.UTC()),
					partycontactmechanismvalidity.EndDateIsNil(),
				),
			).
			WithValidities(func(q *ent.PartyContactMechanismValidityQuery) {
				q.Where(
					partycontactmechanismvalidity.StartDateLTE(at.UTC()),
					partycontactmechanismvalidity.EndDateIsNil(),
				)
			}).
			Order(ent.Asc(partycontactmechanism.FieldContactMechanismOrder)).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainPartyContactMechanism(row, firstValidity(row.Edges.Validities))
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrPartyContactNotFound
	}
	return result, err
}

func (r *PartyContactRepository) CreateAddress(ctx context.Context, e *domain.Address) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.Address.Create().
			SetID(e.ID).
			SetTenantID(e.TenantID).
			SetPartyID(e.PartyID).
			SetType(address.Type(e.Type)).
			SetLine1(e.Line1).
			SetLine2(e.Line2).
			SetCityLocationID(e.CityLocationID).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *PartyContactRepository) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	var result *domain.Address
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.Address.Query().
			Where(address.ID(id), address.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainAddress(e)
		validity, err := client.AddressValidity.Query().
			Where(
				addressvalidity.TenantID(e.TenantID),
				addressvalidity.AddressID(id),
				addressvalidity.EndDateIsNil(),
				addressvalidity.DeletedAtIsNil(),
			).
			First(ctx)
		switch {
		case err == nil:
			result.IsPrimary = validity.IsPrimary
		case !ent.IsNotFound(err):
			return repoError(err)
		}
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) ListAddresses(ctx context.Context, partyID uuid.UUID) ([]*domain.Address, error) {
	var result []*domain.Address
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.Address.Query().
			Where(address.PartyID(partyID), address.DeletedAtIsNil()).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Address, 0, len(rows))
		ids := make([]uuid.UUID, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainAddress(e))
			ids = append(ids, e.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		validities, err := client.AddressValidity.Query().
			Where(
				addressvalidity.AddressIDIn(ids...),
				addressvalidity.EndDateIsNil(),
				addressvalidity.DeletedAtIsNil(),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		primary := make(map[uuid.UUID]bool, len(validities))
		for _, validity := range validities {
			primary[validity.AddressID] = validity.IsPrimary
		}
		for _, address := range result {
			address.IsPrimary = primary[address.ID]
		}
		return nil
	})
	return result, err
}

func (r *PartyContactRepository) UpdateAddress(ctx context.Context, e *domain.Address) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.Address.UpdateOneID(e.ID).
			SetType(address.Type(e.Type)).
			SetLine1(e.Line1).
			SetLine2(e.Line2).
			SetCityLocationID(e.CityLocationID).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetUpdatedAt(e.UpdatedAt)
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *PartyContactRepository) GetOpenAddressValidity(
	ctx context.Context,
	tenantID, addressID uuid.UUID,
) (*domain.AddressValidity, error) {
	var result *domain.AddressValidity
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.AddressValidity.Query().
			Where(
				addressvalidity.TenantID(tenantID),
				addressvalidity.AddressID(addressID),
				addressvalidity.EndDateIsNil(),
				addressvalidity.DeletedAtIsNil(),
			).
			First(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainAddressValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrAddressValidityNotFound
	}
	return result, err
}

func (r *PartyContactRepository) GetOpenPrimaryAddressValidity(
	ctx context.Context,
	tenantID, partyID uuid.UUID,
	addressType domain.AddressType,
) (*domain.AddressValidity, error) {
	var result *domain.AddressValidity
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.AddressValidity.Query().
			Where(
				addressvalidity.TenantID(tenantID),
				addressvalidity.PartyID(partyID),
				addressvalidity.TypeEQ(addressvalidity.Type(addressType)),
				addressvalidity.IsPrimary(true),
				addressvalidity.EndDateIsNil(),
				addressvalidity.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainAddressValidity(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrAddressValidityNotFound
	}
	return result, err
}

func (r *PartyContactRepository) CreateAddressValidity(ctx context.Context, validity *domain.AddressValidity) error {
	if validity == nil {
		return errors.New("repository: nil address validity")
	}
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.AddressValidity.Create().
			SetID(validity.ID).
			SetTenantID(validity.TenantID).
			SetPartyID(validity.PartyID).
			SetAddressID(validity.AddressID).
			SetType(addressvalidity.Type(validity.Type)).
			SetIsPrimary(validity.IsPrimary).
			SetStartDate(validity.StartDate).
			SetCreatedAt(validity.CreatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *PartyContactRepository) CloseAddressValidity(
	ctx context.Context,
	tenantID, validityID uuid.UUID,
	at time.Time,
) error {
	var affected int
	err := r.withClient(ctx, func(client *ent.Client) error {
		var err error
		affected, err = client.AddressValidity.Update().
			Where(
				addressvalidity.ID(validityID),
				addressvalidity.TenantID(tenantID),
				addressvalidity.EndDateIsNil(),
				addressvalidity.DeletedAtIsNil(),
			).
			SetEndDate(at).
			Save(ctx)
		return repoError(err)
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrAddressValidityNotFound
	}
	return nil
}

func toDomainAddressValidity(e *ent.AddressValidity) *domain.AddressValidity {
	return &domain.AddressValidity{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, AddressID: e.AddressID,
		Type: domain.AddressType(e.Type), IsPrimary: e.IsPrimary,
		StartDate: e.StartDate, EndDate: e.EndDate,
		CreatedAt: e.CreatedAt, DeletedAt: e.DeletedAt,
	}
}

func toDomainContactMechanism(e *ent.ContactMechanism) *domain.ContactMechanism {
	if e == nil {
		return nil
	}
	return &domain.ContactMechanism{ID: e.ID, Type: domain.ContactType(e.Type), Value: e.Value}
}

func toDomainPartyContactMechanism(e *ent.PartyContactMechanism, validity *ent.PartyContactMechanismValidity) *domain.PartyContactMechanism {
	relation := &domain.PartyContactMechanism{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, ContactMechanismID: e.ContactMechanismID,
		IsPrimary: e.IsPrimary, IsVerified: e.IsVerified, ContactMechanismOrder: e.ContactMechanismOrder,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
	if validity != nil {
		relation.StartDate = validity.StartDate
		relation.EndDate = validity.EndDate
	}
	return relation
}

func firstValidity(validities []*ent.PartyContactMechanismValidity) *ent.PartyContactMechanismValidity {
	if len(validities) == 0 {
		return nil
	}
	return validities[0]
}

func toDomainAddress(e *ent.Address) *domain.Address {
	return &domain.Address{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		Type: domain.AddressType(e.Type), Line1: e.Line1, Line2: e.Line2,
		CityLocationID: e.CityLocationID, PostalCode: e.PostalCode, Country: e.Country,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}
