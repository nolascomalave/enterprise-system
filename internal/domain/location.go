package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type LocationType string

const (
	LocationTypeRegion       LocationType = "REGION"
	LocationTypeSubregion    LocationType = "SUBREGION"
	LocationTypeState        LocationType = "STATE"
	LocationTypeProvince     LocationType = "PROVINCE"
	LocationTypeCity         LocationType = "CITY"
	LocationTypeMunicipality LocationType = "MUNICIPALITY"
	LocationTypeCounty       LocationType = "COUNTY"
)

func (t LocationType) Valid() bool {
	switch t {
	case LocationTypeRegion, LocationTypeSubregion, LocationTypeState,
		LocationTypeProvince, LocationTypeCity, LocationTypeMunicipality, LocationTypeCounty:
		return true
	default:
		return false
	}
}

var locationCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// Location is a node of the global geographic hierarchy. The path is an ltree
// materialized value such as "world.ve.miranda.caracas".
type Location struct {
	ID           uuid.UUID
	LocationType LocationType
	ParentID     *uuid.UUID
	CountryCode  string
	ISOCode      string
	ExternalID   string
	DefaultName  string
	Path         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewLocation(
	locationType LocationType,
	parentID *uuid.UUID,
	countryCode, isoCode, externalID, defaultName, path string,
	now time.Time,
) (*Location, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate location id: %w", err)
	}
	now = now.UTC()
	location := &Location{
		ID: id, LocationType: locationType, ParentID: parentID,
		CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)),
		ISOCode:     strings.TrimSpace(isoCode),
		ExternalID:  strings.TrimSpace(externalID),
		DefaultName: strings.TrimSpace(defaultName),
		Path:        strings.TrimSpace(path),
		CreatedAt:   now, UpdatedAt: now,
	}
	if err := location.Validate(); err != nil {
		return nil, err
	}
	return location, nil
}

func (l *Location) Validate() error {
	if l == nil {
		return fmt.Errorf("%w: nil entity", ErrInvalidLocation)
	}
	if l.ID == uuid.Nil {
		return fmt.Errorf("%w: id is required", ErrInvalidLocation)
	}
	if !l.LocationType.Valid() {
		return fmt.Errorf("%w: invalid location_type", ErrInvalidLocation)
	}
	if l.ParentID != nil && (*l.ParentID == uuid.Nil || *l.ParentID == l.ID) {
		return fmt.Errorf("%w: invalid parent_id", ErrInvalidLocation)
	}
	if !locationCountryPattern.MatchString(l.CountryCode) {
		return fmt.Errorf("%w: country_code must be ISO 3166-1 alpha-2", ErrInvalidLocation)
	}
	if strings.TrimSpace(l.DefaultName) == "" {
		return fmt.Errorf("%w: default_name is required", ErrInvalidLocation)
	}
	if strings.TrimSpace(l.Path) == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidLocation)
	}
	if l.CreatedAt.IsZero() || l.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: timestamps are required", ErrInvalidLocation)
	}
	return nil
}

// LocationRepository persists the global geographic hierarchy.
type LocationRepository interface {
	CreateLocation(ctx context.Context, location *Location) error
	GetLocation(ctx context.Context, id uuid.UUID) (*Location, error)
	ListLocations(ctx context.Context, countryCode string, parentID *uuid.UUID) ([]*Location, error)
	// Ancestors returns the hierarchy from root to the given location
	// (including the location itself), using ltree `@>`.
	Ancestors(ctx context.Context, id uuid.UUID) ([]*Location, error)
}
