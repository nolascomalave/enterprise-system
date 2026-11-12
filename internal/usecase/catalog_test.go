package usecase

import (
	"context"
	"testing"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCatalogIdentityRepo struct {
	types        []*domain.IdentityDocumentType
	translations map[uuid.UUID]map[string]string
}

func (r *fakeCatalogIdentityRepo) FindDocumentTypeByCode(ctx context.Context, code string) (*domain.IdentityDocumentType, error) {
	for _, documentType := range r.types {
		if documentType.Code == code {
			return documentType, nil
		}
	}
	return nil, domain.ErrIdentityDocumentTypeNotFound
}

func (r *fakeCatalogIdentityRepo) ListDocumentTypes(ctx context.Context) ([]*domain.IdentityDocumentType, error) {
	return r.types, nil
}

func (r *fakeCatalogIdentityRepo) GetDocumentTypeTranslation(ctx context.Context, documentTypeID uuid.UUID, locale string) (string, error) {
	if translations, ok := r.translations[documentTypeID]; ok {
		if name, ok := translations[locale]; ok {
			return name, nil
		}
	}
	return "", domain.ErrTranslationNotFound
}

type fakeRoleTypeRepo struct {
	roles        []*domain.RoleType
	translations map[uuid.UUID]map[string]domain.LocalizedCatalogEntry
}

func (r *fakeRoleTypeRepo) ListRoleTypes(ctx context.Context) ([]*domain.RoleType, error) {
	return r.roles, nil
}

func (r *fakeRoleTypeRepo) GetRoleTypeTranslation(ctx context.Context, roleTypeID uuid.UUID, locale string) (string, string, error) {
	if translations, ok := r.translations[roleTypeID]; ok {
		if entry, ok := translations[locale]; ok {
			return entry.Name, entry.Description, nil
		}
	}
	return "", "", domain.ErrTranslationNotFound
}

func catalogTestService() (*CatalogService, *fakeCatalogIdentityRepo, *fakeRoleTypeRepo, *fakeOrgRepo, uuid.UUID, uuid.UUID) {
	passportID := uuid.New()
	nationalID := uuid.New()
	identity := &fakeCatalogIdentityRepo{
		types: []*domain.IdentityDocumentType{
			{ID: passportID, Code: "PASSPORT", Scope: "PERSON"},
			{ID: nationalID, Code: "NATIONAL_ID", Scope: "PERSON"},
		},
		translations: map[uuid.UUID]map[string]string{
			passportID: {"en": "Passport", "es": "Pasaporte"},
			nationalID: {"en": "National ID"},
		},
	}
	adminID := uuid.New()
	roles := &fakeRoleTypeRepo{
		roles: []*domain.RoleType{{ID: adminID, Code: "ADMIN"}},
		translations: map[uuid.UUID]map[string]domain.LocalizedCatalogEntry{
			adminID: {
				"en": {Name: "Administrator", Description: "Tenant administrator"},
				"es": {Name: "Administrador", Description: "Administrador del tenant"},
			},
		},
	}
	departments := newFakeOrgRepo()
	tenantID := testTenantID()
	department, err := domain.NewDepartment(tenantID, nil, "Departamento Base", "DEP", domain.DepartmentUnitTypeDepartment, nil, orgTestNow())
	if err != nil {
		panic(err)
	}
	departments.departments[department.ID] = department
	departments.departmentTranslations[department.ID] = map[string]string{"en": "Engineering", "es": "Ingeniería"}
	locations := newFakeLocationRepo()
	locations.defaultLocale = "es"
	service := NewCatalogService(identity, roles, departments, locations, newFakeTxManager())
	return service, identity, roles, departments, passportID, nationalID
}

func TestListIdentityDocumentTypesLocalized(t *testing.T) {
	service, identity, _, _, passportID, _ := catalogTestService()

	english, err := service.ListIdentityDocumentTypes(context.Background(), "en-US")
	require.NoError(t, err)
	require.Len(t, english, 2)
	names := map[string]string{}
	for _, entry := range english {
		names[entry.Code] = entry.Name
	}
	assert.Equal(t, "Passport", names["PASSPORT"])
	assert.Equal(t, "National ID", names["NATIONAL_ID"])
	assert.Equal(t, "en", english[0].Locale)
	_ = passportID
	_ = identity

	spanish, err := service.ListIdentityDocumentTypes(context.Background(), "es")
	require.NoError(t, err)
	names = map[string]string{}
	for _, entry := range spanish {
		names[entry.Code] = entry.Name
	}
	assert.Equal(t, "Pasaporte", names["PASSPORT"])
	// No hay traducción es: cae al locale default (es), tampoco existe y usa code.
	assert.Equal(t, "NATIONAL_ID", names["NATIONAL_ID"])
}

func TestListIdentityDocumentTypesFallsBackToDefaultLocale(t *testing.T) {
	service, identity, _, _, passportID, _ := catalogTestService()
	delete(identity.translations[passportID], "en")

	result, err := service.ListIdentityDocumentTypes(context.Background(), "en")

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "Pasaporte", result[0].Name)
	assert.Equal(t, "es", result[0].Locale)
}

func TestListRoleTypesLocalized(t *testing.T) {
	service, _, _, _, _, _ := catalogTestService()

	english, err := service.ListRoleTypes(context.Background(), "en")
	require.NoError(t, err)
	require.Len(t, english, 1)
	assert.Equal(t, "Administrator", english[0].Name)
	assert.Equal(t, "Tenant administrator", english[0].Description)

	spanish, err := service.ListRoleTypes(context.Background(), "es")
	require.NoError(t, err)
	assert.Equal(t, "Administrador", spanish[0].Name)
}

func TestListDepartmentsLocalized(t *testing.T) {
	service, _, _, _, _, _ := catalogTestService()

	english, err := service.ListDepartments(context.Background(), "en")
	require.NoError(t, err)
	require.Len(t, english, 1)
	assert.Equal(t, "Engineering", english[0].Name)
	assert.Equal(t, "en", english[0].Locale)

	spanish, err := service.ListDepartments(context.Background(), "es")
	require.NoError(t, err)
	assert.Equal(t, "Ingeniería", spanish[0].Name)
}
