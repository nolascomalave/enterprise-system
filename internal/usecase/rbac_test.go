package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"enterprise-system/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRBACRepo struct {
	functions           map[string]*domain.SystemFunction
	templates           map[uuid.UUID]*domain.MenuTemplate
	menuCategories      map[uuid.UUID]*domain.MenuCategory
	menuOptions         map[uuid.UUID]*domain.MenuOption
	roles               map[uuid.UUID]*domain.TenantRole
	roleMenuOptions     map[string]*domain.RoleMenuOption
	userRoleAssignments map[string]*domain.UserRoleAssignment
	createErr           error

	// Permission resolver simulation
	userCredentials       map[uuid.UUID]uuid.UUID        // user_id -> party_id
	employees             map[uuid.UUID]*domain.Employee // party_id -> employee
	positionPermissions   map[uuid.UUID][]string         // job_id -> codes
	departmentPermissions map[uuid.UUID][]string         // department_id -> codes
	rolePermissions       map[string][]string            // tenant_id:user_id:app_client_id -> codes (pre-filtered granted)
	overridePermissions   map[uuid.UUID][]string         // party_id -> codes
}

func newFakeRBACRepo() *fakeRBACRepo {
	return &fakeRBACRepo{
		functions:             make(map[string]*domain.SystemFunction),
		templates:             make(map[uuid.UUID]*domain.MenuTemplate),
		menuCategories:        make(map[uuid.UUID]*domain.MenuCategory),
		menuOptions:           make(map[uuid.UUID]*domain.MenuOption),
		roles:                 make(map[uuid.UUID]*domain.TenantRole),
		roleMenuOptions:       make(map[string]*domain.RoleMenuOption),
		userRoleAssignments:   make(map[string]*domain.UserRoleAssignment),
		userCredentials:       make(map[uuid.UUID]uuid.UUID),
		employees:             make(map[uuid.UUID]*domain.Employee),
		positionPermissions:   make(map[uuid.UUID][]string),
		departmentPermissions: make(map[uuid.UUID][]string),
		rolePermissions:       make(map[string][]string),
		overridePermissions:   make(map[uuid.UUID][]string),
	}
}

func (r *fakeRBACRepo) CreateSystemFunction(ctx context.Context, fn *domain.SystemFunction) error {
	r.functions[fn.Code] = fn
	return nil
}

func (r *fakeRBACRepo) FindSystemFunctionByCode(ctx context.Context, code string) (*domain.SystemFunction, error) {
	fn, ok := r.functions[code]
	if !ok {
		return nil, domain.ErrSystemFunctionNotFound
	}
	return fn, nil
}

func (r *fakeRBACRepo) CreateMenuTemplate(ctx context.Context, template *domain.MenuTemplate) error {
	r.templates[template.ID] = template
	return nil
}

func (r *fakeRBACRepo) AssignMenuTemplateFunction(ctx context.Context, link *domain.MenuTemplateFunction) error {
	return nil
}

func (r *fakeRBACRepo) CreateMenuCategory(ctx context.Context, category *domain.MenuCategory) error {
	r.menuCategories[category.ID] = category
	return nil
}

func (r *fakeRBACRepo) UpdateMenuCategory(ctx context.Context, category *domain.MenuCategory) error {
	r.menuCategories[category.ID] = category
	return nil
}

func (r *fakeRBACRepo) FindMenuCategoryByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuCategory, error) {
	cat, ok := r.menuCategories[id]
	if !ok || cat.TenantID != tenantID {
		return nil, domain.ErrMenuCategoryNotFound
	}
	return cat, nil
}

func (r *fakeRBACRepo) ListMenuCategories(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuCategory, error) {
	var out []*domain.MenuCategory
	for _, cat := range r.menuCategories {
		if cat.TenantID == tenantID && cat.DeletedAt == nil {
			out = append(out, cat)
		}
	}
	return out, nil
}

func (r *fakeRBACRepo) CreateMenuOption(ctx context.Context, option *domain.MenuOption) error {
	r.menuOptions[option.ID] = option
	return nil
}

func (r *fakeRBACRepo) UpdateMenuOption(ctx context.Context, option *domain.MenuOption) error {
	r.menuOptions[option.ID] = option
	return nil
}

func (r *fakeRBACRepo) FindMenuOptionByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuOption, error) {
	opt, ok := r.menuOptions[id]
	if !ok || opt.TenantID != tenantID {
		return nil, domain.ErrMenuOptionNotFound
	}
	return opt, nil
}

func (r *fakeRBACRepo) ListMenuOptions(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuOption, error) {
	var out []*domain.MenuOption
	for _, opt := range r.menuOptions {
		if opt.TenantID == tenantID && opt.DeletedAt == nil {
			out = append(out, opt)
		}
	}
	return out, nil
}

func (r *fakeRBACRepo) AssignMenuOptionFunction(ctx context.Context, link *domain.MenuOptionFunction) error {
	return nil
}

func (r *fakeRBACRepo) AssignMenuOptionCategory(ctx context.Context, link *domain.MenuOptionCategory) error {
	return nil
}

func (r *fakeRBACRepo) CreateTenantRole(ctx context.Context, role *domain.TenantRole) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.roles[role.ID] = role
	return nil
}

func (r *fakeRBACRepo) UpdateTenantRole(ctx context.Context, role *domain.TenantRole) error {
	r.roles[role.ID] = role
	return nil
}

func (r *fakeRBACRepo) FindTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.TenantRole, error) {
	role, ok := r.roles[id]
	if !ok || role.TenantID != tenantID || role.DeletedAt != nil {
		return nil, domain.ErrTenantRoleNotFound
	}
	return role, nil
}

func (r *fakeRBACRepo) ListTenantRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRole, error) {
	var out []*domain.TenantRole
	for _, role := range r.roles {
		if role.TenantID == tenantID && role.DeletedAt == nil {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *fakeRBACRepo) SoftDeleteTenantRole(ctx context.Context, tenantID, id uuid.UUID, at time.Time) error {
	role, ok := r.roles[id]
	if !ok || role.TenantID != tenantID || role.DeletedAt != nil {
		return domain.ErrTenantRoleNotFound
	}
	role.DeletedAt = &at
	return nil
}

func (r *fakeRBACRepo) AssignRoleMenuOption(ctx context.Context, link *domain.RoleMenuOption) error {
	key := link.TenantID.String() + ":" + link.TenantRoleID.String() + ":" + link.MenuOptionID.String()
	r.roleMenuOptions[key] = link
	return nil
}

func (r *fakeRBACRepo) RevokeRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error {
	key := tenantID.String() + ":" + roleID.String() + ":" + optionID.String()
	delete(r.roleMenuOptions, key)
	return nil
}

func (r *fakeRBACRepo) AssignUserRole(ctx context.Context, assignment *domain.UserRoleAssignment) error {
	key := assignment.TenantID.String() + ":" + assignment.UserCredentialsID.String() + ":" + assignment.TenantRoleID.String()
	r.userRoleAssignments[key] = assignment
	return nil
}

func (r *fakeRBACRepo) RevokeUserRole(ctx context.Context, tenantID, userCredentialsID, roleID uuid.UUID) error {
	key := tenantID.String() + ":" + userCredentialsID.String() + ":" + roleID.String()
	delete(r.userRoleAssignments, key)
	return nil
}

func (r *fakeRBACRepo) ListUserRoleAssignments(ctx context.Context, tenantID, userCredentialsID uuid.UUID) ([]*domain.UserRoleAssignment, error) {
	var out []*domain.UserRoleAssignment
	prefix := tenantID.String() + ":" + userCredentialsID.String() + ":"
	for key, a := range r.userRoleAssignments {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *fakeRBACRepo) ResolvePermissionUniverse(ctx context.Context, tenantID, userCredentialsID, appClientID uuid.UUID) (*domain.PermissionUniverse, error) {
	codes := make(map[string]struct{})

	partyID, isEmployee := r.userCredentials[userCredentialsID]
	if isEmployee {
		if emp, ok := r.employees[partyID]; ok {
			for _, code := range r.positionPermissions[emp.JobID] {
				codes[code] = struct{}{}
			}
			for _, code := range r.departmentPermissions[emp.DepartmentID] {
				codes[code] = struct{}{}
			}
			for _, code := range r.overridePermissions[partyID] {
				codes[code] = struct{}{}
			}
		}
	}

	key := tenantID.String() + ":" + userCredentialsID.String() + ":" + appClientID.String()
	for _, code := range r.rolePermissions[key] {
		codes[code] = struct{}{}
	}

	result := make([]string, 0, len(codes))
	for code := range codes {
		result = append(result, code)
	}
	return &domain.PermissionUniverse{Functions: result}, nil
}

func (r *fakeRBACRepo) GetMenuTree(ctx context.Context, tenantID, userCredentialsID, appClientID uuid.UUID) ([]domain.MenuTreeNode, error) {
	return nil, nil
}

func rbacService(t *testing.T) *RBACService {
	return NewRBACService(newFakeRBACRepo(), newFakeTxManager(),
		func() time.Time { return time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC) })
}

func TestCreateTenantRoleRequiresTenantAndName(t *testing.T) {
	service := rbacService(t)

	_, err := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{})
	require.ErrorIs(t, err, domain.ErrInvalidTenantRole)

	_, err = service.CreateTenantRole(context.Background(), CreateTenantRoleInput{TenantID: uuid.New(), Name: "  "})
	require.ErrorIs(t, err, domain.ErrInvalidTenantRole)
}

func TestCreateTenantRoleStoresRole(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()

	role, err := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID:    tenantID,
		Name:        "Payroll Admin",
		Description: "Administra nómina",
	})

	require.NoError(t, err)
	assert.Equal(t, tenantID, role.TenantID)
	assert.Equal(t, "Payroll Admin", role.Name)
	assert.Equal(t, "Administra nómina", role.Description)
	assert.Nil(t, role.DeletedAt)
}

func TestCreateTenantRoleWrapsRepositoryError(t *testing.T) {
	repo := &fakeRBACRepo{createErr: errors.New("boom")}
	service := NewRBACService(repo, newFakeTxManager(), time.Now)

	_, err := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: uuid.New(), Name: "Role",
	})

	require.Error(t, err)
}

func TestSoftDeleteTenantRoleMarksDeleted(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	role, _ := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: tenantID, Name: "ToDelete",
	})

	err := service.SoftDeleteTenantRole(context.Background(), tenantID, role.ID)

	require.NoError(t, err)
	_, err = service.FindTenantRoleByID(context.Background(), tenantID, role.ID)
	require.ErrorIs(t, err, domain.ErrTenantRoleNotFound)
}

func TestSoftDeleteTenantRoleWrongTenant(t *testing.T) {
	service := rbacService(t)
	role, _ := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: uuid.New(), Name: "Role",
	})

	err := service.SoftDeleteTenantRole(context.Background(), uuid.New(), role.ID)

	require.ErrorIs(t, err, domain.ErrTenantRoleNotFound)
}

func TestAssignRoleMenuOption(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	role, _ := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: tenantID, Name: "Role",
	})
	option, _ := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, Code: "opt1", Name: "Option 1",
	})

	err := service.AssignRoleMenuOption(context.Background(), tenantID, role.ID, option.ID)

	require.NoError(t, err)
}

func TestAssignUserRoleRequiresValidInput(t *testing.T) {
	service := rbacService(t)

	_, err := service.AssignUserRole(context.Background(), AssignUserRoleInput{})
	require.ErrorIs(t, err, domain.ErrInvalidRoleAssignment)

	_, err = service.AssignUserRole(context.Background(), AssignUserRoleInput{
		TenantID: uuid.New(), UserCredentialsID: uuid.New(), TenantRoleID: uuid.Nil,
	})
	require.ErrorIs(t, err, domain.ErrInvalidRoleAssignment)
}

func TestAssignUserRoleStoresAssignment(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	userID := uuid.New()
	role, _ := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: tenantID, Name: "Role",
	})

	assignment, err := service.AssignUserRole(context.Background(), AssignUserRoleInput{
		TenantID: tenantID, UserCredentialsID: userID, TenantRoleID: role.ID, IsGranted: true,
	})

	require.NoError(t, err)
	assert.Equal(t, tenantID, assignment.TenantID)
	assert.Equal(t, userID, assignment.UserCredentialsID)
	assert.Equal(t, role.ID, assignment.TenantRoleID)
	assert.True(t, assignment.IsGranted)
}

func TestRevokeUserRoleRemovesAssignment(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	userID := uuid.New()
	role, _ := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: tenantID, Name: "Role",
	})
	_, _ = service.AssignUserRole(context.Background(), AssignUserRoleInput{
		TenantID: tenantID, UserCredentialsID: userID, TenantRoleID: role.ID, IsGranted: true,
	})

	err := service.RevokeUserRole(context.Background(), tenantID, userID, role.ID)

	require.NoError(t, err)
	assignments, err := service.ListUserRoles(context.Background(), tenantID, userID)
	require.NoError(t, err)
	assert.Empty(t, assignments)
}

func TestCreateMenuCategoryRequiresTenantAndName(t *testing.T) {
	service := rbacService(t)

	_, err := service.CreateMenuCategory(context.Background(), CreateMenuCategoryInput{})
	require.ErrorIs(t, err, domain.ErrInvalidMenuCategory)

	_, err = service.CreateMenuCategory(context.Background(), CreateMenuCategoryInput{
		TenantID: uuid.New(), Code: "cat", Name: "",
	})
	require.ErrorIs(t, err, domain.ErrInvalidMenuCategory)
}

func TestCreateMenuCategoryStoresCategory(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()

	cat, err := service.CreateMenuCategory(context.Background(), CreateMenuCategoryInput{
		TenantID: tenantID, Code: "cat", Name: "Category", SortOrder: 1,
	})

	require.NoError(t, err)
	assert.Equal(t, "cat", cat.Code)
	assert.Equal(t, "Category", cat.Name)
	assert.Equal(t, 1, cat.SortOrder)
}

func TestCreateMenuOptionRequiresTenantAndName(t *testing.T) {
	service := rbacService(t)

	_, err := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{})
	require.ErrorIs(t, err, domain.ErrInvalidMenuOption)

	_, err = service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: uuid.New(), Code: "opt", Name: "  ",
	})
	require.ErrorIs(t, err, domain.ErrInvalidMenuOption)
}

func TestCreateMenuOptionStoresOption(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()

	opt, err := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, Code: "opt", Name: "Option", Icon: "icon", Route: "/route",
	})

	require.NoError(t, err)
	assert.Equal(t, tenantID, opt.TenantID)
	assert.Equal(t, "opt", opt.Code)
	assert.Equal(t, "Option", opt.Name)
	assert.Equal(t, "icon", opt.Icon)
	assert.Equal(t, "/route", opt.Route)
}

func TestCreateMenuOptionWithTemplate(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	templateID := uuid.New()

	opt, err := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, TemplateID: &templateID, Code: "opt", Name: "Option",
	})

	require.NoError(t, err)
	require.NotNil(t, opt.TemplateID)
	assert.Equal(t, templateID, *opt.TemplateID)
}

func TestUpdateMenuOption(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	opt, _ := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, Code: "opt", Name: "Old",
	})

	updated, err := service.UpdateMenuOption(context.Background(), tenantID, opt.ID, UpdateMenuOptionInput{
		Name: "New", Route: "/new",
	})

	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, "/new", updated.Route)
}

func TestUpdateMenuOptionWrongTenant(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	opt, _ := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, Code: "opt", Name: "Old",
	})

	_, err := service.UpdateMenuOption(context.Background(), uuid.New(), opt.ID, UpdateMenuOptionInput{
		Name: "New",
	})

	require.ErrorIs(t, err, domain.ErrMenuOptionNotFound)
}

func TestRBACServiceUsesTransactionManager(t *testing.T) {
	repo := newFakeRBACRepo()
	tx := newFakeTxManager()
	service := NewRBACService(repo, tx, time.Now)

	_, err := service.CreateTenantRole(context.Background(), CreateTenantRoleInput{
		TenantID: uuid.New(), Name: "Role",
	})

	require.NoError(t, err)
	assert.GreaterOrEqual(t, tx.calls, 1)
}

func TestAssignMenuOptionFunction(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	opt, _ := service.CreateMenuOption(context.Background(), CreateMenuOptionInput{
		TenantID: tenantID, Code: "opt", Name: "Option",
	})
	fnID := uuid.New()

	err := service.AssignMenuOptionFunction(context.Background(), tenantID, opt.ID, fnID, true)

	require.NoError(t, err)
}

func TestValidationHelpers(t *testing.T) {
	assert.True(t, domain.IsValidClientType("FIRST_PARTY"))
	assert.True(t, domain.IsValidApplicationScope("PLATFORM"))
	assert.False(t, domain.IsValidClientType("OTHER"))
}

// Ensure fakeRBACRepo satisfies the interface.
var _ domain.RBACRepository = (*fakeRBACRepo)(nil)

// Avoid unused import warnings.

func TestResolvePermissionsUnitesAllSources(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	userID := uuid.New()
	partyID := uuid.New()
	jobID := uuid.New()
	departmentID := uuid.New()
	appClientID := uuid.New()

	repo := service.repo.(*fakeRBACRepo)
	repo.userCredentials[userID] = partyID
	repo.employees[partyID] = &domain.Employee{JobID: jobID, DepartmentID: departmentID}
	repo.positionPermissions[jobID] = []string{"party:read"}
	repo.departmentPermissions[departmentID] = []string{"org:read"}
	repo.rolePermissions[tenantID.String()+":"+userID.String()+":"+appClientID.String()] = []string{"employee:read"}
	repo.overridePermissions[partyID] = []string{"admin:write"}

	universe, err := service.ResolvePermissions(context.Background(), ResolvePermissionsInput{
		TenantID: tenantID, UserCredentialsID: userID, AppClientID: appClientID,
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"party:read", "org:read", "employee:read", "admin:write"}, universe.Functions)
}

func TestResolvePermissionsDeniesRevokedRole(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	userID := uuid.New()
	partyID := uuid.New()
	appClientID := uuid.New()

	repo := service.repo.(*fakeRBACRepo)
	repo.userCredentials[userID] = partyID
	repo.employees[partyID] = &domain.Employee{}
	repo.rolePermissions[tenantID.String()+":"+userID.String()+":"+appClientID.String()] = []string{"employee:read"}

	universe, err := service.ResolvePermissions(context.Background(), ResolvePermissionsInput{
		TenantID: tenantID, UserCredentialsID: userID, AppClientID: appClientID,
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"employee:read"}, universe.Functions)
}

func TestResolvePermissionsWithoutEmployeeReturnsRoles(t *testing.T) {
	service := rbacService(t)
	tenantID := uuid.New()
	userID := uuid.New()
	appClientID := uuid.New()

	repo := service.repo.(*fakeRBACRepo)
	repo.rolePermissions[tenantID.String()+":"+userID.String()+":"+appClientID.String()] = []string{"rbac:read"}

	universe, err := service.ResolvePermissions(context.Background(), ResolvePermissionsInput{
		TenantID: tenantID, UserCredentialsID: userID, AppClientID: appClientID,
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"rbac:read"}, universe.Functions)
}

func TestResolvePermissionsRequiresValidInput(t *testing.T) {
	service := rbacService(t)

	_, err := service.ResolvePermissions(context.Background(), ResolvePermissionsInput{})
	require.ErrorIs(t, err, domain.ErrInvalidPermission)
}
