package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

var locationCountryCode = regexp.MustCompile(`^[A-Z]{2}$`)

type CreateLocationInput struct {
	LocationType domain.LocationType
	ParentID     *uuid.UUID
	CountryCode  string
	ISOCode      string
	ExternalID   string
	DefaultName  string
}

type LocationService struct {
	repo domain.LocationRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewLocationService(repo domain.LocationRepository, tx TransactionManager, now func() time.Time) *LocationService {
	if now == nil {
		now = time.Now
	}
	return &LocationService{repo: repo, tx: tx, now: now}
}

func (s *LocationService) withinTx(ctx context.Context, fn func(context.Context) error) error {
	if s == nil || s.repo == nil || s.tx == nil {
		return errors.New("locations: nil dependencies")
	}
	return s.tx.WithinTransaction(ctx, fn)
}

// Create builds the ltree path from the parent (if any) and persists the node.
func (s *LocationService) Create(ctx context.Context, in CreateLocationInput) (*domain.Location, error) {
	if !in.LocationType.Valid() {
		return nil, fmt.Errorf("%w: invalid location_type", domain.ErrInvalidLocation)
	}
	defaultName := strings.TrimSpace(in.DefaultName)
	if defaultName == "" {
		return nil, fmt.Errorf("%w: default_name is required", domain.ErrInvalidLocation)
	}
	slug := locationSlug(defaultName)
	if slug == "" {
		return nil, fmt.Errorf("%w: default_name cannot be converted to an ltree label", domain.ErrInvalidLocation)
	}

	var location *domain.Location
	err := s.withinTx(ctx, func(ctx context.Context) error {
		var err error
		country := strings.ToUpper(strings.TrimSpace(in.CountryCode))
		path := "world"
		if in.ParentID != nil {
			parent, err := s.repo.GetLocation(ctx, *in.ParentID)
			if err != nil {
				return fmt.Errorf("get parent location: %w", err)
			}
			if country == "" {
				country = parent.CountryCode
			}
			if country != parent.CountryCode {
				return domain.ErrLocationCountryMismatch
			}
			path = parent.Path + "." + slug
		} else {
			if country == "" {
				return fmt.Errorf("%w: country_code is required for a root location", domain.ErrInvalidLocation)
			}
			path = "world." + strings.ToLower(country) + "." + slug
		}
		if !locationCountryCode.MatchString(country) {
			return fmt.Errorf("%w: country_code must be ISO 3166-1 alpha-2", domain.ErrInvalidLocation)
		}

		location, err = domain.NewLocation(
			in.LocationType, in.ParentID, country, in.ISOCode, in.ExternalID,
			defaultName, path, s.now(),
		)
		if err != nil {
			return err
		}
		if err := s.repo.CreateLocation(ctx, location); err != nil {
			return fmt.Errorf("create location: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return location, nil
}

// Hierarchy returns root -> leaf ancestors using the ltree path.
func (s *LocationService) Hierarchy(ctx context.Context, id uuid.UUID) ([]*domain.Location, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: id is required", domain.ErrInvalidLocation)
	}
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.Location, error) {
		return s.repo.Ancestors(ctx, id)
	})
}

// List returns the children of parentID (or the roots) for a country.
func (s *LocationService) List(ctx context.Context, countryCode string, parentID *uuid.UUID) ([]*domain.Location, error) {
	country := strings.ToUpper(strings.TrimSpace(countryCode))
	if !locationCountryCode.MatchString(country) {
		return nil, fmt.Errorf("%w: country must be ISO 3166-1 alpha-2", domain.ErrInvalidLocation)
	}
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.Location, error) {
		return s.repo.ListLocations(ctx, country, parentID)
	})
}

// locationSlug converts a display name into a valid ltree label: lowercase,
// without diacritics, using underscores instead of spaces/punctuation.
func locationSlug(value string) string {
	decomposed := norm.NFD.String(strings.ToLower(strings.TrimSpace(value)))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			builder.WriteRune(r)
			lastUnderscore = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastUnderscore && builder.Len() > 0 {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}
