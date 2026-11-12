package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLocationUseCase struct {
	country         string
	parentID        *uuid.UUID
	locale          string
	result          []*domain.Location
	localizedResult []*domain.TranslatedLocation
	err             error
}

func (f *fakeLocationUseCase) List(ctx context.Context, countryCode string, parentID *uuid.UUID) ([]*domain.Location, error) {
	f.country = countryCode
	f.parentID = parentID
	return f.result, f.err
}

func (f *fakeLocationUseCase) ListLocalized(ctx context.Context, countryCode string, parentID *uuid.UUID, locale string) ([]*domain.TranslatedLocation, error) {
	f.country = countryCode
	f.parentID = parentID
	f.locale = locale
	return f.localizedResult, f.err
}

func TestListLocationsHandler(t *testing.T) {
	parentID := uuid.New()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	location := &domain.Location{
		ID: uuid.New(), LocationType: domain.LocationTypeCity, ParentID: &parentID,
		CountryCode: "VE", DefaultName: "Caracas", Path: "world.ve.miranda.caracas",
		CreatedAt: now, UpdatedAt: now,
	}
	uc := &fakeLocationUseCase{localizedResult: []*domain.TranslatedLocation{{
		Location: location, Name: "Caracas (EN)", Locale: "en",
	}}}
	router := NewRouter(Dependencies{LocationUseCase: uc})

	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/locations?country=ve&parent_id="+parentID.String(), nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "VE", uc.country)
	assert.Equal(t, "en", uc.locale)
	require.NotNil(t, uc.parentID)
	assert.Equal(t, parentID, *uc.parentID)
	body := decodeBody(t, recorder)
	data := body["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "Caracas (EN)", data[0].(map[string]any)["name"])
	assert.Equal(t, "Caracas", data[0].(map[string]any)["default_name"])
}

func TestListLocationsHandlerRequiresCountry(t *testing.T) {
	router := NewRouter(Dependencies{LocationUseCase: &fakeLocationUseCase{}})

	recorder := requestWithToken(router, http.MethodGet, "/api/v1/locations", nil, "")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestListLocationsHandlerRejectsInvalidParent(t *testing.T) {
	router := NewRouter(Dependencies{LocationUseCase: &fakeLocationUseCase{}})

	recorder := requestWithToken(router, http.MethodGet,
		"/api/v1/locations?country=VE&parent_id=not-a-uuid", nil, "")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
