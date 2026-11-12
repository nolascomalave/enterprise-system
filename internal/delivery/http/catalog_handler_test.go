package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCatalogUseCase struct {
	entries []*domain.LocalizedCatalogEntry
}

func (f *fakeCatalogUseCase) ListIdentityDocumentTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error) {
	result := make([]*domain.LocalizedCatalogEntry, 0, len(f.entries))
	for _, entry := range f.entries {
		copy := *entry
		if locale == "es" {
			copy.Name = "Pasaporte"
		} else {
			copy.Name = "Passport"
		}
		copy.Locale = locale
		result = append(result, &copy)
	}
	return result, nil
}

func (f *fakeCatalogUseCase) ListRoleTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error) {
	return nil, nil
}

func (f *fakeCatalogUseCase) ListDepartments(ctx context.Context, locale string) ([]*domain.LocalizedDepartment, error) {
	return nil, nil
}

func TestListIdentityDocumentTypesLocalizedByHeader(t *testing.T) {
	uc := &fakeCatalogUseCase{entries: []*domain.LocalizedCatalogEntry{{
		ID: uuid.New(), Code: "PASSPORT",
	}}}
	router := NewRouter(Dependencies{CatalogUseCase: uc})

	for _, tc := range []struct {
		header string
		want   string
	}{
		{"en-US,en;q=0.9", "Passport"},
		{"es", "Pasaporte"},
		{"", "Pasaporte"},
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/identity-document-types", nil)
		if tc.header != "" {
			request.Header.Set("Accept-Language", tc.header)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		data := decodeBody(t, recorder)["data"].([]any)
		require.Len(t, data, 1)
		assert.Equal(t, tc.want, data[0].(map[string]any)["name"])
	}
}
