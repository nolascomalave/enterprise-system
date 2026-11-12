package domain

import (
	"context"

	"github.com/google/uuid"
)

// LocalizedCatalogEntry is a catalog item resolved for a locale.
type LocalizedCatalogEntry struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	Locale      string
}

// LocalizedDepartment is a department resolved for a locale.
type LocalizedDepartment struct {
	Department *Department
	Name       string
	Locale     string
}

// LocaleProvider resolves the default locale.
type LocaleProvider interface {
	GetDefaultLocale(ctx context.Context) (string, error)
}

// DocumentTypeCatalogRepository reads the global identity document type
// catalog and its translations.
type DocumentTypeCatalogRepository interface {
	ListDocumentTypes(ctx context.Context) ([]*IdentityDocumentType, error)
	GetDocumentTypeTranslation(ctx context.Context, documentTypeID uuid.UUID, locale string) (string, error)
}

// RoleTypeRepository reads the global role type catalog.
type RoleTypeRepository interface {
	ListRoleTypes(ctx context.Context) ([]*RoleType, error)
	GetRoleTypeTranslation(ctx context.Context, roleTypeID uuid.UUID, locale string) (name, description string, err error)
}
