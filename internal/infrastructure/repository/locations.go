package repository

import (
	"context"
	"errors"

	"enterprise-system/ent"
	"enterprise-system/ent/location"
	"enterprise-system/internal/domain"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
)

var _ domain.LocationRepository = (*LocationRepository)(nil)

type LocationRepository struct {
	client *ent.Client
}

func NewLocationRepository(client *ent.Client) *LocationRepository {
	return &LocationRepository{client: client}
}

func (r *LocationRepository) CreateLocation(ctx context.Context, e *domain.Location) error {
	if e == nil {
		return errors.New("repository: nil location")
	}
	return execute(ctx, r.client, func(client *ent.Client) error {
		create := client.Location.Create().
			SetID(e.ID).
			SetLocationType(location.LocationType(e.LocationType)).
			SetCountryCode(e.CountryCode).
			SetDefaultName(e.DefaultName).
			SetPath(e.Path).
			SetCreatedAt(e.CreatedAt).
			SetUpdatedAt(e.UpdatedAt)
		if e.ParentID != nil {
			create.SetParentID(*e.ParentID)
		}
		if e.ISOCode != "" {
			create.SetIsoCode(e.ISOCode)
		}
		if e.ExternalID != "" {
			create.SetExternalID(e.ExternalID)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *LocationRepository) GetLocation(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	var result *domain.Location
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.Location.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		result = toDomainLocation(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrLocationNotFound
	}
	return result, err
}

func (r *LocationRepository) ListLocations(
	ctx context.Context,
	countryCode string,
	parentID *uuid.UUID,
) ([]*domain.Location, error) {
	var result []*domain.Location
	err := execute(ctx, r.client, func(client *ent.Client) error {
		query := client.Location.Query().
			Where(location.CountryCode(countryCode))
		if parentID == nil {
			query = query.Where(location.ParentIDIsNil())
		} else {
			query = query.Where(location.ParentID(*parentID))
		}
		rows, err := query.Order(ent.Asc(location.FieldDefaultName)).All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Location, 0, len(rows))
		for _, row := range rows {
			result = append(result, toDomainLocation(row))
		}
		return nil
	})
	return result, err
}

func (r *LocationRepository) Ancestors(ctx context.Context, id uuid.UUID) ([]*domain.Location, error) {
	var result []*domain.Location
	err := execute(ctx, r.client, func(client *ent.Client) error {
		target, err := client.Location.Get(ctx, id)
		if err != nil {
			return repoError(err)
		}
		rows, err := client.Location.Query().
			Where(func(s *sql.Selector) {
				s.Where(sql.ExprP("path @> ?::ltree", target.Path))
			}).
			Order(func(s *sql.Selector) {
				s.OrderBy("nlevel(path)")
			}).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.Location, 0, len(rows))
		for _, row := range rows {
			result = append(result, toDomainLocation(row))
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrLocationNotFound
	}
	return result, err
}

func toDomainLocation(e *ent.Location) *domain.Location {
	return &domain.Location{
		ID: e.ID, LocationType: domain.LocationType(e.LocationType),
		ParentID: e.ParentID, CountryCode: e.CountryCode,
		ISOCode: derefString(e.IsoCode), ExternalID: derefString(e.ExternalID),
		DefaultName: e.DefaultName, Path: e.Path,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
