package usecase

import (
	"context"
	"errors"
	"fmt"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// CatalogService resolves localized global catalogs and department names with
// the fallback chain: requested locale -> default locale -> base value.
type CatalogService struct {
	identity    domain.DocumentTypeCatalogRepository
	roles       domain.RoleTypeRepository
	departments domain.OrgStructureRepository
	locales     domain.LocaleProvider
	tx          TransactionManager
}

func NewCatalogService(
	identity domain.DocumentTypeCatalogRepository,
	roles domain.RoleTypeRepository,
	departments domain.OrgStructureRepository,
	locales domain.LocaleProvider,
	tx TransactionManager,
) *CatalogService {
	return &CatalogService{
		identity: identity, roles: roles, departments: departments,
		locales: locales, tx: tx,
	}
}

func (s *CatalogService) ListIdentityDocumentTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error) {
	locale = normalizeLocale(locale)
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.LocalizedCatalogEntry, error) {
		types, err := s.identity.ListDocumentTypes(ctx)
		if err != nil {
			return nil, fmt.Errorf("list document types: %w", err)
		}
		defaultLocale, err := s.defaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*domain.LocalizedCatalogEntry, 0, len(types))
		for _, documentType := range types {
			name, used, err := s.resolveName(ctx, locale, defaultLocale, documentType.Code,
				func(locale string) (string, error) {
					return s.identity.GetDocumentTypeTranslation(ctx, documentType.ID, locale)
				})
			if err != nil {
				return nil, err
			}
			result = append(result, &domain.LocalizedCatalogEntry{
				ID: documentType.ID, Code: documentType.Code, Name: name, Locale: used,
			})
		}
		return result, nil
	})
}

func (s *CatalogService) ListRoleTypes(ctx context.Context, locale string) ([]*domain.LocalizedCatalogEntry, error) {
	locale = normalizeLocale(locale)
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.LocalizedCatalogEntry, error) {
		roles, err := s.roles.ListRoleTypes(ctx)
		if err != nil {
			return nil, fmt.Errorf("list role types: %w", err)
		}
		defaultLocale, err := s.defaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*domain.LocalizedCatalogEntry, 0, len(roles))
		for _, role := range roles {
			name, description, used, err := s.resolveRole(ctx, locale, defaultLocale, role.Code, role.ID)
			if err != nil {
				return nil, err
			}
			result = append(result, &domain.LocalizedCatalogEntry{
				ID: role.ID, Code: role.Code, Name: name,
				Description: description, Locale: used,
			})
		}
		return result, nil
	})
}

func (s *CatalogService) ListDepartments(ctx context.Context, locale string) ([]*domain.LocalizedDepartment, error) {
	locale = normalizeLocale(locale)
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.LocalizedDepartment, error) {
		departments, err := s.departments.ListDepartments(ctx)
		if err != nil {
			return nil, fmt.Errorf("list departments: %w", err)
		}
		defaultLocale, err := s.defaultLocale(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*domain.LocalizedDepartment, 0, len(departments))
		for _, department := range departments {
			name, used, err := s.resolveName(ctx, locale, defaultLocale, department.Name,
				func(locale string) (string, error) {
					return s.departments.GetDepartmentTranslation(ctx, department.ID, locale)
				})
			if err != nil {
				return nil, err
			}
			result = append(result, &domain.LocalizedDepartment{
				Department: department, Name: name, Locale: used,
			})
		}
		return result, nil
	})
}

func (s *CatalogService) resolveName(
	ctx context.Context,
	locale, defaultLocale, base string,
	lookup func(locale string) (string, error),
) (string, string, error) {
	if name, ok, err := s.tryTranslation(locale, lookup); err != nil {
		return "", "", err
	} else if ok {
		return name, locale, nil
	}
	if defaultLocale != "" && defaultLocale != locale {
		if name, ok, err := s.tryTranslation(defaultLocale, lookup); err != nil {
			return "", "", err
		} else if ok {
			return name, defaultLocale, nil
		}
	}
	return base, "", nil
}

func (s *CatalogService) tryTranslation(
	locale string,
	lookup func(locale string) (string, error),
) (string, bool, error) {
	if locale == "" {
		return "", false, nil
	}
	name, err := lookup(locale)
	if errors.Is(err, domain.ErrTranslationNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get translation: %w", err)
	}
	return name, true, nil
}

func (s *CatalogService) resolveRole(
	ctx context.Context,
	locale, defaultLocale, baseCode string,
	roleID uuid.UUID,
) (string, string, string, error) {
	if locale != "" {
		name, description, err := s.roles.GetRoleTypeTranslation(ctx, roleID, locale)
		if err == nil {
			return name, description, locale, nil
		}
		if !errors.Is(err, domain.ErrTranslationNotFound) {
			return "", "", "", fmt.Errorf("get role translation: %w", err)
		}
	}
	if defaultLocale != "" && defaultLocale != locale {
		name, description, err := s.roles.GetRoleTypeTranslation(ctx, roleID, defaultLocale)
		if err == nil {
			return name, description, defaultLocale, nil
		}
		if !errors.Is(err, domain.ErrTranslationNotFound) {
			return "", "", "", fmt.Errorf("get default role translation: %w", err)
		}
	}
	return baseCode, "", "", nil
}

func (s *CatalogService) defaultLocale(ctx context.Context) (string, error) {
	if s.locales == nil {
		return "es", nil
	}
	locale, err := s.locales.GetDefaultLocale(ctx)
	if errors.Is(err, domain.ErrDefaultLocaleNotFound) {
		return "es", nil
	}
	if err != nil {
		return "", fmt.Errorf("get default locale: %w", err)
	}
	return normalizeLocale(locale), nil
}
