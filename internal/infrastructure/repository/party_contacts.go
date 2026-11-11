package repository

import (
	"context"
	"errors"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/address"
	"enterprise-system/ent/contactmechanism"
	"enterprise-system/ent/partycontactmechanism"
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
			return err
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
			return err
		}
		row, err := client.ContactMechanism.Query().
			Where(contactmechanism.TypeEQ(contactmechanism.Type(contact.Type)), contactmechanism.Value(contact.Value)).
			Only(ctx)
		if err != nil {
			return err
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
			SetStartDate(relation.StartDate).
			SetCreatedAt(relation.CreatedAt).
			SetUpdatedAt(relation.UpdatedAt)
		if relation.EndDate != nil {
			create.SetEndDate(*relation.EndDate)
		}
		_, err := create.Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) GetPartyContact(ctx context.Context, tenantID, relationID uuid.UUID) (*domain.PartyContact, error) {
	var result *domain.PartyContact
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.PartyContactMechanism.Query().
			Where(partycontactmechanism.ID(relationID), partycontactmechanism.TenantID(tenantID)).
			WithContactMechanism().
			Only(ctx)
		if err != nil {
			return err
		}
		result = &domain.PartyContact{
			Relation: toDomainPartyContactMechanism(row),
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
				partycontactmechanism.EndDateIsNil(),
			).
			WithContactMechanism().
			Order(
				ent.Asc(partycontactmechanism.FieldContactMechanismOrder),
				ent.Asc(partycontactmechanism.FieldStartDate),
			).
			All(ctx)
		if err != nil {
			return err
		}
		result = make([]*domain.PartyContact, 0, len(rows))
		for _, row := range rows {
			result = append(result, &domain.PartyContact{
				Relation: toDomainPartyContactMechanism(row),
				Contact:  toDomainContactMechanism(row.Edges.ContactMechanism),
			})
		}
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
			SetStartDate(relation.StartDate).
			SetUpdatedAt(relation.UpdatedAt)
		if relation.EndDate != nil {
			update.SetEndDate(*relation.EndDate)
		} else {
			update.ClearEndDate()
		}
		_, err := update.Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) FindActivePartyContact(ctx context.Context, tenantID, contactMechanismID uuid.UUID, at time.Time) (*domain.PartyContactMechanism, error) {
	var result *domain.PartyContactMechanism
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.PartyContactMechanism.Query().
			Where(
				partycontactmechanism.TenantID(tenantID),
				partycontactmechanism.ContactMechanismID(contactMechanismID),
				partycontactmechanism.EndDateIsNil(),
				partycontactmechanism.StartDateLTE(at.UTC()),
			).
			Order(ent.Asc(partycontactmechanism.FieldContactMechanismOrder)).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toDomainPartyContactMechanism(row)
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
			SetCity(e.City).
			SetState(e.State).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetIsPrimary(e.IsPrimary).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt).
			Save(ctx)
		return err
	})
}

func (r *PartyContactRepository) GetAddress(ctx context.Context, id uuid.UUID) (*domain.Address, error) {
	var result *domain.Address
	err := r.withClient(ctx, func(client *ent.Client) error {
		e, err := client.Address.Query().
			Where(address.ID(id), address.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			return err
		}
		result = toDomainAddress(e)
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
			return err
		}
		result = make([]*domain.Address, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainAddress(e))
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
			SetCity(e.City).
			SetState(e.State).
			SetPostalCode(e.PostalCode).
			SetCountry(e.Country).
			SetIsPrimary(e.IsPrimary).
			SetUpdatedAt(e.UpdatedAt)
		if e.DeletedAt != nil {
			update.SetDeletedAt(*e.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return err
	})
}

func toDomainContactMechanism(e *ent.ContactMechanism) *domain.ContactMechanism {
	if e == nil {
		return nil
	}
	return &domain.ContactMechanism{ID: e.ID, Type: domain.ContactType(e.Type), Value: e.Value}
}

func toDomainPartyContactMechanism(e *ent.PartyContactMechanism) *domain.PartyContactMechanism {
	return &domain.PartyContactMechanism{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID, ContactMechanismID: e.ContactMechanismID,
		IsPrimary: e.IsPrimary, IsVerified: e.IsVerified, ContactMechanismOrder: e.ContactMechanismOrder,
		StartDate: e.StartDate, EndDate: e.EndDate, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func toDomainAddress(e *ent.Address) *domain.Address {
	return &domain.Address{
		ID: e.ID, TenantID: e.TenantID, PartyID: e.PartyID,
		Type: domain.AddressType(e.Type), Line1: e.Line1, Line2: e.Line2,
		City: e.City, State: e.State, PostalCode: e.PostalCode, Country: e.Country,
		IsPrimary: e.IsPrimary, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}
