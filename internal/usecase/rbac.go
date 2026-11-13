package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

// RBACUseCase is the application boundary for the RBAC catalog and role phase.
type RBACUseCase interface {
	CreateTenantRole(ctx context.Context, input CreateTenantRoleInput) (*domain.TenantRole, error)
	UpdateTenantRole(ctx context.Context, tenantID, id uuid.UUID, input UpdateTenantRoleInput) (*domain.TenantRole, error)
	FindTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.TenantRole, error)
	ListTenantRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRole, error)
	SoftDeleteTenantRole(ctx context.Context, tenantID, id uuid.UUID) error
	AssignRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error
	RevokeRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error

	CreateMenuCategory(ctx context.Context, input CreateMenuCategoryInput) (*domain.MenuCategory, error)
	UpdateMenuCategory(ctx context.Context, tenantID, id uuid.UUID, input UpdateMenuCategoryInput) (*domain.MenuCategory, error)
	FindMenuCategoryByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuCategory, error)
	ListMenuCategories(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuCategory, error)

	CreateMenuOption(ctx context.Context, input CreateMenuOptionInput) (*domain.MenuOption, error)
	UpdateMenuOption(ctx context.Context, tenantID, id uuid.UUID, input UpdateMenuOptionInput) (*domain.MenuOption, error)
	FindMenuOptionByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuOption, error)
	ListMenuOptions(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuOption, error)
	AssignMenuOptionFunction(ctx context.Context, tenantID, optionID, functionID uuid.UUID, isRequired bool) error
	AssignMenuOptionCategory(ctx context.Context, tenantID, optionID, categoryID uuid.UUID, sortOrder int) error

	AssignUserRole(ctx context.Context, input AssignUserRoleInput) (*domain.UserRoleAssignment, error)
	RevokeUserRole(ctx context.Context, tenantID, userCredentialsID, roleID uuid.UUID) error
	ListUserRoles(ctx context.Context, tenantID, userCredentialsID uuid.UUID) ([]*domain.UserRoleAssignment, error)

	ResolvePermissions(ctx context.Context, input ResolvePermissionsInput) (*domain.PermissionUniverse, error)
	GetUserMenu(ctx context.Context, input GetUserMenuInput) ([]domain.MenuTreeNode, error)
}

// RBACService implements RBACUseCase.
type RBACService struct {
	repo domain.RBACRepository
	tx   TransactionManager
	now  func() time.Time
}

func NewRBACService(repo domain.RBACRepository, tx TransactionManager, now func() time.Time) *RBACService {
	if now == nil {
		now = time.Now
	}
	return &RBACService{repo: repo, tx: tx, now: now}
}

// ----------------------------------------------------------------------------
// Inputs
// ----------------------------------------------------------------------------

type CreateTenantRoleInput struct {
	TenantID            uuid.UUID
	ClientApplicationID *uuid.UUID
	Name                string
	Description         string
}

type UpdateTenantRoleInput struct {
	Name        string
	Description string
}

type CreateMenuCategoryInput struct {
	TenantID  uuid.UUID
	ParentID  *uuid.UUID
	Code      string
	Name      string
	SortOrder int
}

type UpdateMenuCategoryInput struct {
	Name      string
	SortOrder *int
}

type CreateMenuOptionInput struct {
	TenantID   uuid.UUID
	TemplateID *uuid.UUID
	Code       string
	Name       string
	Icon       string
	Route      string
}

type UpdateMenuOptionInput struct {
	Name  string
	Icon  string
	Route string
}

type AssignUserRoleInput struct {
	TenantID          uuid.UUID
	UserCredentialsID uuid.UUID
	TenantRoleID      uuid.UUID
	IsGranted         bool
}

type ResolvePermissionsInput struct {
	TenantID          uuid.UUID
	UserCredentialsID uuid.UUID
	AppClientID       uuid.UUID
}

type GetUserMenuInput struct {
	TenantID          uuid.UUID
	UserCredentialsID uuid.UUID
	AppClientID       uuid.UUID
}

// ----------------------------------------------------------------------------
// Tenant roles
// ----------------------------------------------------------------------------

func (s *RBACService) CreateTenantRole(ctx context.Context, input CreateTenantRoleInput) (*domain.TenantRole, error) {
	role, err := domain.NewTenantRole(input.TenantID, input.ClientApplicationID, input.Name, input.Description)
	if err != nil {
		return nil, validationError("invalid_tenant_role", err.Error(), err)
	}
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.CreateTenantRole(ctx, role)
	}); err != nil {
		return nil, fmt.Errorf("create tenant role: %w", err)
	}
	return role, nil
}

func (s *RBACService) UpdateTenantRole(ctx context.Context, tenantID, id uuid.UUID, input UpdateTenantRoleInput) (*domain.TenantRole, error) {
	role, err := s.FindTenantRoleByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, validationError("invalid_tenant_role", "name is required", domain.ErrInvalidTenantRole)
	}
	role.Name = name
	role.Description = strings.TrimSpace(input.Description)
	role.UpdatedAt = s.now().UTC()
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.UpdateTenantRole(ctx, role)
	}); err != nil {
		return nil, fmt.Errorf("update tenant role: %w", err)
	}
	return role, nil
}

func (s *RBACService) FindTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.TenantRole, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.TenantRole, error) {
		role, err := s.repo.FindTenantRoleByID(ctx, tenantID, id)
		if err != nil {
			if id == uuid.Nil {
				return nil, validationError("invalid_role_id", "invalid role id", domain.ErrInvalidTenantRole)
			}
			return nil, err
		}
		return role, nil
	})
}

func (s *RBACService) ListTenantRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRole, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.TenantRole, error) {
		return s.repo.ListTenantRoles(ctx, tenantID)
	})
}

func (s *RBACService) SoftDeleteTenantRole(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.repo.SoftDeleteTenantRole(ctx, tenantID, id, s.now().UTC()); err != nil {
			return fmt.Errorf("soft delete tenant role: %w", err)
		}
		return nil
	})
}

func (s *RBACService) AssignRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error {
	if tenantID == uuid.Nil || roleID == uuid.Nil || optionID == uuid.Nil {
		return validationError("invalid_input", "tenant_id, role_id and option_id are required", domain.ErrInvalidPermission)
	}
	link := &domain.RoleMenuOption{
		ID:           uuid.Must(uuid.NewV7()),
		TenantID:     tenantID,
		TenantRoleID: roleID,
		MenuOptionID: optionID,
		CreatedAt:    s.now().UTC(),
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.AssignRoleMenuOption(ctx, link)
	})
}

func (s *RBACService) RevokeRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error {
	if tenantID == uuid.Nil || roleID == uuid.Nil || optionID == uuid.Nil {
		return validationError("invalid_input", "tenant_id, role_id and option_id are required", domain.ErrInvalidPermission)
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.RevokeRoleMenuOption(ctx, tenantID, roleID, optionID)
	})
}

// ----------------------------------------------------------------------------
// Menu categories
// ----------------------------------------------------------------------------

func (s *RBACService) CreateMenuCategory(ctx context.Context, input CreateMenuCategoryInput) (*domain.MenuCategory, error) {
	category, err := domain.NewMenuCategory(input.TenantID, input.ParentID, input.Code, input.Name, input.SortOrder)
	if err != nil {
		return nil, validationError("invalid_menu_category", err.Error(), err)
	}
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.CreateMenuCategory(ctx, category)
	}); err != nil {
		return nil, fmt.Errorf("create menu category: %w", err)
	}
	return category, nil
}

func (s *RBACService) UpdateMenuCategory(ctx context.Context, tenantID, id uuid.UUID, input UpdateMenuCategoryInput) (*domain.MenuCategory, error) {
	category, err := s.FindMenuCategoryByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, validationError("invalid_menu_category", "name is required", domain.ErrInvalidMenuCategory)
	}
	category.Name = name
	if input.SortOrder != nil {
		category.SortOrder = *input.SortOrder
	}
	category.UpdatedAt = s.now().UTC()
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.UpdateMenuCategory(ctx, category)
	}); err != nil {
		return nil, fmt.Errorf("update menu category: %w", err)
	}
	return category, nil
}

func (s *RBACService) FindMenuCategoryByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuCategory, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.MenuCategory, error) {
		return s.repo.FindMenuCategoryByID(ctx, tenantID, id)
	})
}

func (s *RBACService) ListMenuCategories(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuCategory, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.MenuCategory, error) {
		return s.repo.ListMenuCategories(ctx, tenantID)
	})
}

// ----------------------------------------------------------------------------
// Menu options
// ----------------------------------------------------------------------------

func (s *RBACService) CreateMenuOption(ctx context.Context, input CreateMenuOptionInput) (*domain.MenuOption, error) {
	option, err := domain.NewMenuOption(input.TenantID, input.TemplateID, input.Code, input.Name, input.Icon, input.Route)
	if err != nil {
		return nil, validationError("invalid_menu_option", err.Error(), err)
	}
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.CreateMenuOption(ctx, option)
	}); err != nil {
		return nil, fmt.Errorf("create menu option: %w", err)
	}
	return option, nil
}

func (s *RBACService) UpdateMenuOption(ctx context.Context, tenantID, id uuid.UUID, input UpdateMenuOptionInput) (*domain.MenuOption, error) {
	option, err := s.FindMenuOptionByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, validationError("invalid_menu_option", "name is required", domain.ErrInvalidMenuOption)
	}
	option.Name = name
	option.Icon = input.Icon
	option.Route = input.Route
	option.UpdatedAt = s.now().UTC()
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.UpdateMenuOption(ctx, option)
	}); err != nil {
		return nil, fmt.Errorf("update menu option: %w", err)
	}
	return option, nil
}

func (s *RBACService) FindMenuOptionByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuOption, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.MenuOption, error) {
		return s.repo.FindMenuOptionByID(ctx, tenantID, id)
	})
}

func (s *RBACService) ListMenuOptions(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuOption, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.MenuOption, error) {
		return s.repo.ListMenuOptions(ctx, tenantID)
	})
}

func (s *RBACService) AssignMenuOptionFunction(ctx context.Context, tenantID, optionID, functionID uuid.UUID, isRequired bool) error {
	if tenantID == uuid.Nil || optionID == uuid.Nil || functionID == uuid.Nil {
		return validationError("invalid_input", "tenant_id, option_id and function_id are required", domain.ErrInvalidMenuOptionFunction)
	}
	link := &domain.MenuOptionFunction{
		ID:               uuid.Must(uuid.NewV7()),
		TenantID:         tenantID,
		MenuOptionID:     optionID,
		SystemFunctionID: functionID,
		IsRequired:       isRequired,
		CreatedAt:        s.now().UTC(),
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.AssignMenuOptionFunction(ctx, link)
	})
}

func (s *RBACService) AssignMenuOptionCategory(ctx context.Context, tenantID, optionID, categoryID uuid.UUID, sortOrder int) error {
	if tenantID == uuid.Nil || optionID == uuid.Nil || categoryID == uuid.Nil {
		return validationError("invalid_input", "tenant_id, option_id and category_id are required", domain.ErrInvalidMenuCategory)
	}
	link := &domain.MenuOptionCategory{
		ID:             uuid.Must(uuid.NewV7()),
		TenantID:       tenantID,
		MenuCategoryID: categoryID,
		MenuOptionID:   optionID,
		SortOrder:      sortOrder,
		CreatedAt:      s.now().UTC(),
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.AssignMenuOptionCategory(ctx, link)
	})
}

// ----------------------------------------------------------------------------
// User role assignments
// ----------------------------------------------------------------------------

func (s *RBACService) AssignUserRole(ctx context.Context, input AssignUserRoleInput) (*domain.UserRoleAssignment, error) {
	assignment, err := domain.NewUserRoleAssignment(input.TenantID, input.UserCredentialsID, input.TenantRoleID, input.IsGranted)
	if err != nil {
		return nil, validationError("invalid_role_assignment", err.Error(), err)
	}
	if err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.AssignUserRole(ctx, assignment)
	}); err != nil {
		return nil, fmt.Errorf("assign user role: %w", err)
	}
	return assignment, nil
}

func (s *RBACService) RevokeUserRole(ctx context.Context, tenantID, userCredentialsID, roleID uuid.UUID) error {
	if tenantID == uuid.Nil || userCredentialsID == uuid.Nil || roleID == uuid.Nil {
		return validationError("invalid_input", "tenant_id, user_id and role_id are required", domain.ErrInvalidRoleAssignment)
	}
	return s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		return s.repo.RevokeUserRole(ctx, tenantID, userCredentialsID, roleID)
	})
}

func (s *RBACService) ListUserRoles(ctx context.Context, tenantID, userCredentialsID uuid.UUID) ([]*domain.UserRoleAssignment, error) {
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]*domain.UserRoleAssignment, error) {
		return s.repo.ListUserRoleAssignments(ctx, tenantID, userCredentialsID)
	})
}

func (s *RBACService) ResolvePermissions(ctx context.Context, input ResolvePermissionsInput) (*domain.PermissionUniverse, error) {
	if input.TenantID == uuid.Nil || input.UserCredentialsID == uuid.Nil || input.AppClientID == uuid.Nil {
		return nil, validationError("invalid_permission_input", "tenant_id, user_id and app_client_id are required", domain.ErrInvalidPermission)
	}
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) (*domain.PermissionUniverse, error) {
		return s.repo.ResolvePermissionUniverse(ctx, input.TenantID, input.UserCredentialsID, input.AppClientID)
	})
}

func (s *RBACService) GetUserMenu(ctx context.Context, input GetUserMenuInput) ([]domain.MenuTreeNode, error) {
	if input.TenantID == uuid.Nil || input.UserCredentialsID == uuid.Nil || input.AppClientID == uuid.Nil {
		return nil, validationError("invalid_menu_input", "tenant_id, user_id and app_client_id are required", domain.ErrInvalidPermission)
	}
	return readWithinTransaction(ctx, s.tx, func(ctx context.Context) ([]domain.MenuTreeNode, error) {
		return s.repo.GetMenuTree(ctx, input.TenantID, input.UserCredentialsID, input.AppClientID)
	})
}
