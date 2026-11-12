package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// Catálogos globales
// ----------------------------------------------------------------------------

type FunctionScope string

const (
	FunctionScopePlatform FunctionScope = "PLATFORM"
	FunctionScopeTenant   FunctionScope = "TENANT"
)

func (s FunctionScope) Valid() bool {
	return s == FunctionScopePlatform || s == FunctionScopeTenant
}

// SystemFunction is a globally shared business capability.
type SystemFunction struct {
	ID            uuid.UUID
	Code          string
	Name          string
	Description   string
	FunctionScope FunctionScope
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

func NewSystemFunction(code, name, description string, scope FunctionScope) (*SystemFunction, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" || !scope.Valid() {
		return nil, fmt.Errorf("%w: invalid system function", ErrInvalidSystemFunction)
	}
	now := time.Now().UTC()
	return &SystemFunction{
		ID:            uuid.Must(uuid.NewV7()),
		Code:          code,
		Name:          name,
		Description:   strings.TrimSpace(description),
		FunctionScope: scope,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// MenuTemplate is a shared menu layout for a class of client applications.
type MenuTemplate struct {
	ID               uuid.UUID
	Code             string
	Name             string
	ClientType       ClientType
	ApplicationScope ApplicationScope
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func NewMenuTemplate(code, name string, clientType ClientType, applicationScope ApplicationScope) (*MenuTemplate, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return nil, fmt.Errorf("%w: invalid menu template", ErrInvalidMenuTemplate)
	}
	now := time.Now().UTC()
	return &MenuTemplate{
		ID:               uuid.Must(uuid.NewV7()),
		Code:             code,
		Name:             name,
		ClientType:       clientType,
		ApplicationScope: applicationScope,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// MenuTemplateFunction links a template to a required system function.
type MenuTemplateFunction struct {
	ID               uuid.UUID
	TemplateID       uuid.UUID
	SystemFunctionID uuid.UUID
	IsRequired       bool
	CreatedAt        time.Time
}

// ----------------------------------------------------------------------------
// Tenant-scoped menu entities
// ----------------------------------------------------------------------------

// MenuCategory is a tenant-owned hierarchical menu group.
type MenuCategory struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	ParentID  *uuid.UUID
	Code      string
	Name      string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewMenuCategory(tenantID uuid.UUID, parentID *uuid.UUID, code, name string, sortOrder int) (*MenuCategory, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if tenantID == uuid.Nil || code == "" || name == "" || sortOrder < 0 {
		return nil, fmt.Errorf("%w: invalid menu category", ErrInvalidMenuCategory)
	}
	now := time.Now().UTC()
	return &MenuCategory{
		ID:        uuid.Must(uuid.NewV7()),
		TenantID:  tenantID,
		ParentID:  parentID,
		Code:      code,
		Name:      name,
		SortOrder: sortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// MenuOption is a tenant-owned menu entry.
type MenuOption struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	TemplateID *uuid.UUID
	Code       string
	Name       string
	Icon       string
	Route      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func NewMenuOption(tenantID uuid.UUID, templateID *uuid.UUID, code, name, icon, route string) (*MenuOption, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if tenantID == uuid.Nil || code == "" || name == "" {
		return nil, fmt.Errorf("%w: invalid menu option", ErrInvalidMenuOption)
	}
	now := time.Now().UTC()
	return &MenuOption{
		ID:         uuid.Must(uuid.NewV7()),
		TenantID:   tenantID,
		TemplateID: templateID,
		Code:       code,
		Name:       name,
		Icon:       icon,
		Route:      route,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// MenuOptionFunction links a menu option to a required system function.
type MenuOptionFunction struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	MenuOptionID     uuid.UUID
	SystemFunctionID uuid.UUID
	IsRequired       bool
	CreatedAt        time.Time
}

// MenuOptionCategory links a menu option to a category.
type MenuOptionCategory struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	MenuCategoryID uuid.UUID
	MenuOptionID   uuid.UUID
	SortOrder      int
	CreatedAt      time.Time
}

// ----------------------------------------------------------------------------
// Tenant roles and assignments
// ----------------------------------------------------------------------------

// TenantRole is a tenant-owned collection of menu options.
type TenantRole struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	ClientApplicationID *uuid.UUID
	Name                string
	Description         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           *time.Time
}

func NewTenantRole(tenantID uuid.UUID, clientApplicationID *uuid.UUID, name, description string) (*TenantRole, error) {
	name = strings.TrimSpace(name)
	if tenantID == uuid.Nil || name == "" {
		return nil, fmt.Errorf("%w: invalid tenant role", ErrInvalidTenantRole)
	}
	now := time.Now().UTC()
	return &TenantRole{
		ID:                  uuid.Must(uuid.NewV7()),
		TenantID:            tenantID,
		ClientApplicationID: clientApplicationID,
		Name:                name,
		Description:         strings.TrimSpace(description),
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// RoleMenuOption grants a menu option to a role.
type RoleMenuOption struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	TenantRoleID uuid.UUID
	MenuOptionID uuid.UUID
	CreatedAt    time.Time
}

// UserRoleAssignment links a user to a tenant role.
type UserRoleAssignment struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	UserCredentialsID uuid.UUID
	TenantRoleID      uuid.UUID
	IsGranted         bool
	AssignedAt        time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewUserRoleAssignment(tenantID, userCredentialsID, tenantRoleID uuid.UUID, isGranted bool) (*UserRoleAssignment, error) {
	if tenantID == uuid.Nil || userCredentialsID == uuid.Nil || tenantRoleID == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid role assignment", ErrInvalidRoleAssignment)
	}
	now := time.Now().UTC()
	return &UserRoleAssignment{
		ID:                uuid.Must(uuid.NewV7()),
		TenantID:          tenantID,
		UserCredentialsID: userCredentialsID,
		TenantRoleID:      tenantRoleID,
		IsGranted:         isGranted,
		AssignedAt:        now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// ----------------------------------------------------------------------------
// Organizational permissions and overrides
// ----------------------------------------------------------------------------

// DepartmentMenuOption links a department to a menu option.
type DepartmentMenuOption struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	DepartmentID uuid.UUID
	MenuOptionID uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DepartmentMenuOptionValidity is a validity window for a department menu option.
type DepartmentMenuOptionValidity struct {
	ID                     uuid.UUID
	TenantID               uuid.UUID
	DepartmentMenuOptionID uuid.UUID
	DepartmentID           uuid.UUID
	MenuOptionID           uuid.UUID
	StartDate              time.Time
	EndDate                *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// PositionMenuOption links a job to a menu option.
type PositionMenuOption struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	JobID        uuid.UUID
	MenuOptionID uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PositionMenuOptionValidity is a validity window for a position menu option.
type PositionMenuOptionValidity struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	PositionMenuOptionID uuid.UUID
	JobID                uuid.UUID
	MenuOptionID         uuid.UUID
	StartDate            time.Time
	EndDate              *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// EmployeeMenuOptionOverride grants or denies a function directly to an employee.
type EmployeeMenuOptionOverride struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	EmployeePartyID  uuid.UUID
	SystemFunctionID uuid.UUID
	IsGranted        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ----------------------------------------------------------------------------
// Permission universe and menu tree
// ----------------------------------------------------------------------------

// PermissionUniverse is the set of system function codes a user has.
type PermissionUniverse struct {
	Functions []string
}

// MenuTreeNode is a category node with nested options.
type MenuTreeNode struct {
	ID        uuid.UUID
	Code      string
	Name      string
	SortOrder int
	Options   []MenuTreeOption
	Children  []MenuTreeNode
}

// MenuTreeOption is a visible menu option.
type MenuTreeOption struct {
	ID        uuid.UUID
	Code      string
	Name      string
	Icon      string
	Route     string
	SortOrder int
}

// RBACRepository persists and queries RBAC entities.
type RBACRepository interface {
	// System functions
	CreateSystemFunction(ctx context.Context, fn *SystemFunction) error
	FindSystemFunctionByCode(ctx context.Context, code string) (*SystemFunction, error)

	// Menu templates
	CreateMenuTemplate(ctx context.Context, template *MenuTemplate) error
	AssignMenuTemplateFunction(ctx context.Context, link *MenuTemplateFunction) error

	// Tenant menu
	CreateMenuCategory(ctx context.Context, category *MenuCategory) error
	UpdateMenuCategory(ctx context.Context, category *MenuCategory) error
	FindMenuCategoryByID(ctx context.Context, tenantID, id uuid.UUID) (*MenuCategory, error)
	ListMenuCategories(ctx context.Context, tenantID uuid.UUID) ([]*MenuCategory, error)

	CreateMenuOption(ctx context.Context, option *MenuOption) error
	UpdateMenuOption(ctx context.Context, option *MenuOption) error
	FindMenuOptionByID(ctx context.Context, tenantID, id uuid.UUID) (*MenuOption, error)
	ListMenuOptions(ctx context.Context, tenantID uuid.UUID) ([]*MenuOption, error)
	AssignMenuOptionFunction(ctx context.Context, link *MenuOptionFunction) error
	AssignMenuOptionCategory(ctx context.Context, link *MenuOptionCategory) error

	// Tenant roles
	CreateTenantRole(ctx context.Context, role *TenantRole) error
	UpdateTenantRole(ctx context.Context, role *TenantRole) error
	FindTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*TenantRole, error)
	ListTenantRoles(ctx context.Context, tenantID uuid.UUID) ([]*TenantRole, error)
	SoftDeleteTenantRole(ctx context.Context, tenantID, id uuid.UUID, at time.Time) error
	AssignRoleMenuOption(ctx context.Context, link *RoleMenuOption) error
	RevokeRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error

	// User role assignments
	AssignUserRole(ctx context.Context, assignment *UserRoleAssignment) error
	RevokeUserRole(ctx context.Context, tenantID, userCredentialsID, roleID uuid.UUID) error
	ListUserRoleAssignments(ctx context.Context, tenantID, userCredentialsID uuid.UUID) ([]*UserRoleAssignment, error)

	// Permission resolution (Fase 2)
	ResolvePermissionUniverse(ctx context.Context, tenantID, userCredentialsID uuid.UUID) (*PermissionUniverse, error)

	// Menu tree (Fase 3)
	GetMenuTree(ctx context.Context, tenantID, userCredentialsID, appClientID uuid.UUID) ([]MenuTreeNode, error)
}

// Sentinel validation helpers.
func IsValidClientType(value string) bool {
	return value == "FIRST_PARTY" || value == "THIRD_PARTY"
}

func IsValidApplicationScope(value string) bool {
	return value == "PLATFORM" || value == "TENANT_SPECIFIC"
}
