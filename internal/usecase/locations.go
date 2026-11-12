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

// GetTranslatedLocation resolves a localized name with the fallback chain:
// requested locale -> default locale -> locations.default_name.
func (s *LocationService) GetTranslatedLocation(
	ctx context.Context,
	locationID uuid.UUID,
	locale string,
) (*domain.TranslatedLocation, error) {
	if locationID == uuid.Nil {
		return nil, fmt.Errorf("%w: id is required", domain.ErrInvalidLocation)
	}
	locale = normalizeLocale(locale)
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.TranslatedLocation, error) {
		location, err := s.repo.GetLocation(ctx, locationID)
		if err != nil {
			return nil, err
		}
		defaultLocale, err := s.defaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		return s.resolveTranslation(ctx, location, locale, defaultLocale)
	})
}

// ListLocalized lists children and applies the translation fallback to each
// one using a single transaction.
func (s *LocationService) ListLocalized(
	ctx context.Context,
	countryCode string,
	parentID *uuid.UUID,
	locale string,
) ([]*domain.TranslatedLocation, error) {
	country := strings.ToUpper(strings.TrimSpace(countryCode))
	if !locationCountryCode.MatchString(country) {
		return nil, fmt.Errorf("%w: country must be ISO 3166-1 alpha-2", domain.ErrInvalidLocation)
	}
	locale = normalizeLocale(locale)
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.TranslatedLocation, error) {
		locations, err := s.repo.ListLocations(ctx, country, parentID)
		if err != nil {
			return nil, err
		}
		defaultLocale, err := s.defaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*domain.TranslatedLocation, 0, len(locations))
		for _, location := range locations {
			translated, err := s.resolveTranslation(ctx, location, locale, defaultLocale)
			if err != nil {
				return nil, err
			}
			result = append(result, translated)
		}
		return result, nil
	})
}

func (s *LocationService) defaultLocale(ctx context.Context) (string, error) {
	locale, err := s.repo.GetDefaultLocale(ctx)
	if errors.Is(err, domain.ErrDefaultLocaleNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get default locale: %w", err)
	}
	return normalizeLocale(locale), nil
}

func (s *LocationService) resolveTranslation(
	ctx context.Context,
	location *domain.Location,
	locale, defaultLocale string,
) (*domain.TranslatedLocation, error) {
	if locale != "" {
		name, err := s.repo.GetLocationTranslation(ctx, location.ID, locale)
		switch {
		case err == nil:
			return &domain.TranslatedLocation{Location: location, Name: name, Locale: locale}, nil
		case !errors.Is(err, domain.ErrTranslationNotFound):
			return nil, fmt.Errorf("get location translation: %w", err)
		}
	}
	if defaultLocale != "" && defaultLocale != locale {
		name, err := s.repo.GetLocationTranslation(ctx, location.ID, defaultLocale)
		switch {
		case err == nil:
			return &domain.TranslatedLocation{Location: location, Name: name, Locale: defaultLocale}, nil
		case !errors.Is(err, domain.ErrTranslationNotFound):
			return nil, fmt.Errorf("get default location translation: %w", err)
		}
	}
	return &domain.TranslatedLocation{Location: location, Name: location.DefaultName}, nil
}

// normalizeLocale maps common BCP 47 variants to the locales we support.
func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return ""
	}
	lower := strings.ToLower(locale)
	if lower == "pt-br" || strings.HasPrefix(lower, "pt-") || lower == "pt" {
		return "pt-BR"
	}
	switch {
	case strings.HasPrefix(lower, "es"):
		return "es"
	case strings.HasPrefix(lower, "en"):
		return "en"
	default:
		return locale
	}
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
