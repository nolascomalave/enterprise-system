package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLocationRepo struct {
	locations     map[uuid.UUID]*domain.Location
	translations  map[uuid.UUID]map[string]string
	defaultLocale string
	err           error
	created       int
}

func newFakeLocationRepo() *fakeLocationRepo {
	return &fakeLocationRepo{
		locations:     make(map[uuid.UUID]*domain.Location),
		translations:  make(map[uuid.UUID]map[string]string),
		defaultLocale: "es",
	}
}

func (r *fakeLocationRepo) CreateLocation(ctx context.Context, location *domain.Location) error {
	if r.err != nil {
		return r.err
	}
	r.created++
	r.locations[location.ID] = location
	return nil
}

func (r *fakeLocationRepo) GetLocation(ctx context.Context, id uuid.UUID) (*domain.Location, error) {
	location, ok := r.locations[id]
	if !ok {
		return nil, domain.ErrLocationNotFound
	}
	return location, nil
}

func (r *fakeLocationRepo) ListLocations(ctx context.Context, countryCode string, parentID *uuid.UUID) ([]*domain.Location, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]*domain.Location, 0)
	for _, location := range r.locations {
		if location.CountryCode != countryCode {
			continue
		}
		if parentID == nil {
			if location.ParentID == nil {
				out = append(out, location)
			}
			continue
		}
		if location.ParentID != nil && *location.ParentID == *parentID {
			out = append(out, location)
		}
	}
	return out, nil
}

func (r *fakeLocationRepo) Ancestors(ctx context.Context, id uuid.UUID) ([]*domain.Location, error) {
	target, ok := r.locations[id]
	if !ok {
		return nil, domain.ErrLocationNotFound
	}
	out := make([]*domain.Location, 0)
	for _, location := range r.locations {
		if location.Path == target.Path || strings.HasPrefix(target.Path, location.Path+".") {
			out = append(out, location)
		}
	}
	// root -> leaf
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if len(out[j].Path) < len(out[i].Path) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func locationTestNow() time.Time {
	return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
}

func TestCreateLocationRootGeneratesPath(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)

	location, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeState,
		CountryCode:  "ve",
		DefaultName:  "Distrito Capital",
	})

	require.NoError(t, err)
	assert.Equal(t, "world.ve.distrito_capital", location.Path)
	assert.Nil(t, location.ParentID)
	assert.Equal(t, "VE", location.CountryCode)
	assert.Equal(t, 1, repo.created)
}

func TestCreateLocationChildGeneratesPathFromParent(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	parent, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeState,
		CountryCode:  "VE",
		DefaultName:  "Miranda",
	})
	require.NoError(t, err)

	location, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity,
		ParentID:     &parent.ID,
		CountryCode:  "VE",
		DefaultName:  "Andalucía",
	})

	require.NoError(t, err)
	assert.Equal(t, "world.ve.miranda.andalucia", location.Path)
	require.NotNil(t, location.ParentID)
	assert.Equal(t, parent.ID, *location.ParentID)
}

func TestCreateLocationRejectsParentFromAnotherCountry(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	parent, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeState, CountryCode: "VE", DefaultName: "Miranda",
	})
	require.NoError(t, err)

	_, err = uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, ParentID: &parent.ID,
		CountryCode: "CO", DefaultName: "Bogotá",
	})

	require.ErrorIs(t, err, domain.ErrLocationCountryMismatch)
	assert.Equal(t, 1, repo.created)
}

func TestCreateLocationRejectsInvalidInput(t *testing.T) {
	uc := NewLocationService(newFakeLocationRepo(), newFakeTxManager(), locationTestNow)

	_, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: "COUNTRY", CountryCode: "VE", DefaultName: "Caracas",
	})
	require.ErrorIs(t, err, domain.ErrInvalidLocation)

	_, err = uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "!!",
	})
	require.ErrorIs(t, err, domain.ErrInvalidLocation)
}

func TestGetLocationHierarchyReturnsAncestors(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	country, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeRegion, CountryCode: "VE", DefaultName: "Venezuela",
	})
	require.NoError(t, err)
	state, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeState, ParentID: &country.ID,
		CountryCode: "VE", DefaultName: "Miranda",
	})
	require.NoError(t, err)
	city, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, ParentID: &state.ID,
		CountryCode: "VE", DefaultName: "Caracas",
	})
	require.NoError(t, err)

	hierarchy, err := uc.Hierarchy(context.Background(), city.ID)

	require.NoError(t, err)
	require.Len(t, hierarchy, 3)
	assert.Equal(t, country.ID, hierarchy[0].ID)
	assert.Equal(t, state.ID, hierarchy[1].ID)
	assert.Equal(t, city.ID, hierarchy[2].ID)
}

func (r *fakeLocationRepo) GetLocationTranslation(ctx context.Context, locationID uuid.UUID, locale string) (string, error) {
	if translations, ok := r.translations[locationID]; ok {
		if name, ok := translations[locale]; ok {
			return name, nil
		}
	}
	return "", domain.ErrTranslationNotFound
}

func (r *fakeLocationRepo) GetDefaultLocale(ctx context.Context) (string, error) {
	if r.defaultLocale == "" {
		return "", domain.ErrDefaultLocaleNotFound
	}
	return r.defaultLocale, nil
}

func TestGetTranslatedLocationDirect(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	location, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "Caracas",
	})
	require.NoError(t, err)
	repo.translations[location.ID] = map[string]string{"en": "Caracas (EN)"}

	translated, err := uc.GetTranslatedLocation(context.Background(), location.ID, "en")

	require.NoError(t, err)
	assert.Equal(t, "Caracas (EN)", translated.Name)
	assert.Equal(t, "en", translated.Locale)
}

func TestGetTranslatedLocationFallsBackToDefaultLocale(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	location, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "Caracas",
	})
	require.NoError(t, err)
	repo.translations[location.ID] = map[string]string{"es": "Caracas (ES)"}

	translated, err := uc.GetTranslatedLocation(context.Background(), location.ID, "en")

	require.NoError(t, err)
	assert.Equal(t, "Caracas (ES)", translated.Name)
	assert.Equal(t, "es", translated.Locale)
}

func TestGetTranslatedLocationFallsBackToDefaultName(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	location, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "Caracas",
	})
	require.NoError(t, err)

	translated, err := uc.GetTranslatedLocation(context.Background(), location.ID, "en")

	require.NoError(t, err)
	assert.Equal(t, "Caracas", translated.Name)
	assert.Empty(t, translated.Locale)
}

func TestListLocalizedAppliesFallbackPerLocation(t *testing.T) {
	repo := newFakeLocationRepo()
	uc := NewLocationService(repo, newFakeTxManager(), locationTestNow)
	withTranslation, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "Caracas",
	})
	require.NoError(t, err)
	withoutTranslation, err := uc.Create(context.Background(), CreateLocationInput{
		LocationType: domain.LocationTypeCity, CountryCode: "VE", DefaultName: "Valencia",
	})
	require.NoError(t, err)
	repo.translations[withTranslation.ID] = map[string]string{"en": "Caracas (EN)"}

	list, err := uc.ListLocalized(context.Background(), "VE", nil, "en")

	require.NoError(t, err)
	require.Len(t, list, 2)
	names := map[string]string{}
	for _, item := range list {
		names[item.Location.DefaultName] = item.Name
	}
	assert.Equal(t, "Caracas (EN)", names["Caracas"])
	assert.Equal(t, "Valencia", names["Valencia"])
	_ = withoutTranslation
}
