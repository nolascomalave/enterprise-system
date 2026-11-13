package repository

import (
	"context"
	"sort"
	"time"

	"enterprise-system/ent"
	"enterprise-system/ent/departmentmenuoptionvalidity"
	"enterprise-system/ent/employee"
	"enterprise-system/ent/employeemenuoptionoverride"
	"enterprise-system/ent/menucategory"
	"enterprise-system/ent/menuoption"
	"enterprise-system/ent/menuoptioncategory"
	"enterprise-system/ent/menutemplate"
	"enterprise-system/ent/positionmenuoptionvalidity"
	"enterprise-system/ent/rolemenuoption"
	"enterprise-system/ent/systemfunction"
	"enterprise-system/ent/tenantrole"
	"enterprise-system/ent/userroleassignment"
	"enterprise-system/internal/domain"

	"github.com/google/uuid"
)

var _ domain.RBACRepository = (*RBACRepository)(nil)

type RBACRepository struct {
	client *ent.Client
}

func NewRBACRepository(client *ent.Client) *RBACRepository {
	return &RBACRepository{client: client}
}

func (r *RBACRepository) withClient(ctx context.Context, fn func(*ent.Client) error) error {
	if r == nil {
		return errNilClient
	}
	return execute(ctx, r.client, fn)
}

// ----------------------------------------------------------------------------
// System functions
// ----------------------------------------------------------------------------

func (r *RBACRepository) CreateSystemFunction(ctx context.Context, fn *domain.SystemFunction) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.SystemFunction.Create().
			SetID(fn.ID).
			SetCode(fn.Code).
			SetName(fn.Name).
			SetFunctionScope(systemfunction.FunctionScope(fn.FunctionScope)).
			SetCreatedAt(fn.CreatedAt).
			SetUpdatedAt(fn.UpdatedAt)
		if fn.Description != "" {
			create.SetDescription(fn.Description)
		}
		if fn.DeletedAt != nil {
			create.SetDeletedAt(*fn.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) FindSystemFunctionByCode(ctx context.Context, code string) (*domain.SystemFunction, error) {
	var result *domain.SystemFunction
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.SystemFunction.Query().
			Where(systemfunction.Code(code), systemfunction.DeletedAtIsNil()).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainSystemFunction(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrSystemFunctionNotFound
	}
	return result, err
}

// ----------------------------------------------------------------------------
// Menu templates
// ----------------------------------------------------------------------------

func (r *RBACRepository) CreateMenuTemplate(ctx context.Context, template *domain.MenuTemplate) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.MenuTemplate.Create().
			SetID(template.ID).
			SetCode(template.Code).
			SetName(template.Name).
			SetCreatedAt(template.CreatedAt).
			SetUpdatedAt(template.UpdatedAt)
		if template.ClientType != "" {
			create.SetClientType(menutemplate.ClientType(template.ClientType))
		}
		if template.ApplicationScope != "" {
			create.SetApplicationScope(menutemplate.ApplicationScope(template.ApplicationScope))
		}
		if template.DeletedAt != nil {
			create.SetDeletedAt(*template.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) AssignMenuTemplateFunction(ctx context.Context, link *domain.MenuTemplateFunction) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.MenuTemplateFunction.Create().
			SetID(link.ID).
			SetTemplateID(link.TemplateID).
			SetSystemFunctionID(link.SystemFunctionID).
			SetIsRequired(link.IsRequired).
			SetCreatedAt(link.CreatedAt).
			Save(ctx)
		return repoError(err)
	})
}

// ----------------------------------------------------------------------------
// Menu categories
// ----------------------------------------------------------------------------

func (r *RBACRepository) CreateMenuCategory(ctx context.Context, category *domain.MenuCategory) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.MenuCategory.Create().
			SetID(category.ID).
			SetTenantID(category.TenantID).
			SetCode(category.Code).
			SetName(category.Name).
			SetSortOrder(category.SortOrder).
			SetCreatedAt(category.CreatedAt).
			SetUpdatedAt(category.UpdatedAt)
		if category.ParentID != nil {
			create.SetParentID(*category.ParentID)
		}
		if category.DeletedAt != nil {
			create.SetDeletedAt(*category.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) UpdateMenuCategory(ctx context.Context, category *domain.MenuCategory) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.MenuCategory.UpdateOneID(category.ID).
			SetCode(category.Code).
			SetName(category.Name).
			SetSortOrder(category.SortOrder).
			SetUpdatedAt(category.UpdatedAt)
		if category.ParentID != nil {
			update.SetParentID(*category.ParentID)
		} else {
			update.ClearParentID()
		}
		if category.DeletedAt != nil {
			update.SetDeletedAt(*category.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) FindMenuCategoryByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuCategory, error) {
	var result *domain.MenuCategory
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.MenuCategory.Query().
			Where(
				menucategory.ID(id),
				menucategory.TenantID(tenantID),
				menucategory.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainMenuCategory(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrMenuCategoryNotFound
	}
	return result, err
}

func (r *RBACRepository) ListMenuCategories(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuCategory, error) {
	var result []*domain.MenuCategory
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.MenuCategory.Query().
			Where(
				menucategory.TenantID(tenantID),
				menucategory.DeletedAtIsNil(),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.MenuCategory, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainMenuCategory(e))
		}
		return nil
	})
	return result, err
}

// ----------------------------------------------------------------------------
// Menu options
// ----------------------------------------------------------------------------

func (r *RBACRepository) CreateMenuOption(ctx context.Context, option *domain.MenuOption) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.MenuOption.Create().
			SetID(option.ID).
			SetTenantID(option.TenantID).
			SetCode(option.Code).
			SetName(option.Name).
			SetCreatedAt(option.CreatedAt).
			SetUpdatedAt(option.UpdatedAt)
		if option.TemplateID != nil {
			create.SetTemplateID(*option.TemplateID)
		}
		if option.Icon != "" {
			create.SetIcon(option.Icon)
		}
		if option.Route != "" {
			create.SetRoute(option.Route)
		}
		if option.DeletedAt != nil {
			create.SetDeletedAt(*option.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) UpdateMenuOption(ctx context.Context, option *domain.MenuOption) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.MenuOption.UpdateOneID(option.ID).
			SetCode(option.Code).
			SetName(option.Name).
			SetUpdatedAt(option.UpdatedAt)
		if option.TemplateID != nil {
			update.SetTemplateID(*option.TemplateID)
		} else {
			update.ClearTemplateID()
		}
		if option.Icon != "" {
			update.SetIcon(option.Icon)
		} else {
			update.ClearIcon()
		}
		if option.Route != "" {
			update.SetRoute(option.Route)
		} else {
			update.ClearRoute()
		}
		if option.DeletedAt != nil {
			update.SetDeletedAt(*option.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) FindMenuOptionByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.MenuOption, error) {
	var result *domain.MenuOption
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.MenuOption.Query().
			Where(
				menuoption.ID(id),
				menuoption.TenantID(tenantID),
				menuoption.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainMenuOption(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrMenuOptionNotFound
	}
	return result, err
}

func (r *RBACRepository) ListMenuOptions(ctx context.Context, tenantID uuid.UUID) ([]*domain.MenuOption, error) {
	var result []*domain.MenuOption
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.MenuOption.Query().
			Where(
				menuoption.TenantID(tenantID),
				menuoption.DeletedAtIsNil(),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.MenuOption, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainMenuOption(e))
		}
		return nil
	})
	return result, err
}

func (r *RBACRepository) AssignMenuOptionFunction(ctx context.Context, link *domain.MenuOptionFunction) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.MenuOptionFunction.Create().
			SetID(link.ID).
			SetTenantID(link.TenantID).
			SetMenuOptionID(link.MenuOptionID).
			SetSystemFunctionID(link.SystemFunctionID).
			SetIsRequired(link.IsRequired).
			SetCreatedAt(link.CreatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) AssignMenuOptionCategory(ctx context.Context, link *domain.MenuOptionCategory) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.MenuOptionCategory.Create().
			SetID(link.ID).
			SetTenantID(link.TenantID).
			SetMenuCategoryID(link.MenuCategoryID).
			SetMenuOptionID(link.MenuOptionID).
			SetSortOrder(link.SortOrder).
			SetCreatedAt(link.CreatedAt).
			Save(ctx)
		return repoError(err)
	})
}

// ----------------------------------------------------------------------------
// Tenant roles
// ----------------------------------------------------------------------------

func (r *RBACRepository) CreateTenantRole(ctx context.Context, role *domain.TenantRole) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		create := client.TenantRole.Create().
			SetID(role.ID).
			SetTenantID(role.TenantID).
			SetName(role.Name).
			SetCreatedAt(role.CreatedAt).
			SetUpdatedAt(role.UpdatedAt)
		if role.ClientApplicationID != nil {
			create.SetClientApplicationID(*role.ClientApplicationID)
		}
		if role.Description != "" {
			create.SetDescription(role.Description)
		}
		if role.DeletedAt != nil {
			create.SetDeletedAt(*role.DeletedAt)
		}
		_, err := create.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) UpdateTenantRole(ctx context.Context, role *domain.TenantRole) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		update := client.TenantRole.UpdateOneID(role.ID).
			SetName(role.Name).
			SetUpdatedAt(role.UpdatedAt)
		if role.ClientApplicationID != nil {
			update.SetClientApplicationID(*role.ClientApplicationID)
		} else {
			update.ClearClientApplicationID()
		}
		if role.Description != "" {
			update.SetDescription(role.Description)
		} else {
			update.ClearDescription()
		}
		if role.DeletedAt != nil {
			update.SetDeletedAt(*role.DeletedAt)
		} else {
			update.ClearDeletedAt()
		}
		_, err := update.Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) FindTenantRoleByID(ctx context.Context, tenantID, id uuid.UUID) (*domain.TenantRole, error) {
	var result *domain.TenantRole
	err := r.withClient(ctx, func(client *ent.Client) error {
		row, err := client.TenantRole.Query().
			Where(
				tenantrole.ID(id),
				tenantrole.TenantID(tenantID),
				tenantrole.DeletedAtIsNil(),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		result = toDomainTenantRole(row)
		return nil
	})
	if ent.IsNotFound(err) {
		return nil, domain.ErrTenantRoleNotFound
	}
	return result, err
}

func (r *RBACRepository) ListTenantRoles(ctx context.Context, tenantID uuid.UUID) ([]*domain.TenantRole, error) {
	var result []*domain.TenantRole
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.TenantRole.Query().
			Where(
				tenantrole.TenantID(tenantID),
				tenantrole.DeletedAtIsNil(),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.TenantRole, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainTenantRole(e))
		}
		return nil
	})
	return result, err
}

func (r *RBACRepository) SoftDeleteTenantRole(ctx context.Context, tenantID, id uuid.UUID, at time.Time) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.TenantRole.UpdateOneID(id).
			Where(
				tenantrole.TenantID(tenantID),
				tenantrole.DeletedAtIsNil(),
			).
			SetDeletedAt(at).
			Save(ctx)
		if ent.IsNotFound(err) {
			return domain.ErrTenantRoleNotFound
		}
		return repoError(err)
	})
}

func (r *RBACRepository) AssignRoleMenuOption(ctx context.Context, link *domain.RoleMenuOption) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.RoleMenuOption.Create().
			SetID(link.ID).
			SetTenantID(link.TenantID).
			SetTenantRoleID(link.TenantRoleID).
			SetMenuOptionID(link.MenuOptionID).
			SetCreatedAt(link.CreatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) RevokeRoleMenuOption(ctx context.Context, tenantID, roleID, optionID uuid.UUID) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.RoleMenuOption.Delete().
			Where(
				rolemenuoption.TenantID(tenantID),
				rolemenuoption.TenantRoleID(roleID),
				rolemenuoption.MenuOptionID(optionID),
			).
			Exec(ctx)
		return repoError(err)
	})
}

// ----------------------------------------------------------------------------
// User role assignments
// ----------------------------------------------------------------------------

func (r *RBACRepository) AssignUserRole(ctx context.Context, assignment *domain.UserRoleAssignment) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.UserRoleAssignment.Create().
			SetID(assignment.ID).
			SetTenantID(assignment.TenantID).
			SetUserCredentialsID(assignment.UserCredentialsID).
			SetTenantRoleID(assignment.TenantRoleID).
			SetIsGranted(assignment.IsGranted).
			SetAssignedAt(assignment.AssignedAt).
			SetCreatedAt(assignment.CreatedAt).
			SetUpdatedAt(assignment.UpdatedAt).
			Save(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) RevokeUserRole(ctx context.Context, tenantID, userCredentialsID, roleID uuid.UUID) error {
	return r.withClient(ctx, func(client *ent.Client) error {
		_, err := client.UserRoleAssignment.Delete().
			Where(
				userroleassignment.TenantID(tenantID),
				userroleassignment.UserCredentialsID(userCredentialsID),
				userroleassignment.TenantRoleID(roleID),
			).
			Exec(ctx)
		return repoError(err)
	})
}

func (r *RBACRepository) ListUserRoleAssignments(ctx context.Context, tenantID, userCredentialsID uuid.UUID) ([]*domain.UserRoleAssignment, error) {
	var result []*domain.UserRoleAssignment
	err := r.withClient(ctx, func(client *ent.Client) error {
		rows, err := client.UserRoleAssignment.Query().
			Where(
				userroleassignment.TenantID(tenantID),
				userroleassignment.UserCredentialsID(userCredentialsID),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.UserRoleAssignment, 0, len(rows))
		for _, e := range rows {
			result = append(result, toDomainUserRoleAssignment(e))
		}
		return nil
	})
	return result, err
}

// ----------------------------------------------------------------------------
// Permission resolution (stubs)
// ----------------------------------------------------------------------------

func (r *RBACRepository) ResolvePermissionUniverse(ctx context.Context, tenantID, userCredentialsID, appClientID uuid.UUID) (*domain.PermissionUniverse, error) {
	var result []string
	err := r.withClient(ctx, func(client *ent.Client) error {
		creds, err := client.UserCredentials.Get(ctx, userCredentialsID)
		if err != nil {
			if ent.IsNotFound(err) {
				return domain.ErrPermissionNotFound
			}
			return repoError(err)
		}

		var (
			employeeFound bool
			jobID         uuid.UUID
			deptID        uuid.UUID
		)
		emp, err := client.Employee.Query().
			Where(
				employee.ID(creds.PartyID),
				employee.TenantID(tenantID),
			).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return repoError(err)
		}
		if err == nil {
			employeeFound = true
			jobID = emp.JobID
			deptID = emp.DepartmentID
		}

		codes := make(map[string]struct{})
		now := time.Now().UTC()

		if employeeFound && jobID != uuid.Nil {
			validities, err := client.PositionMenuOptionValidity.Query().
				Where(
					positionmenuoptionvalidity.TenantID(tenantID),
					positionmenuoptionvalidity.JobID(jobID),
					positionmenuoptionvalidity.StartDateLTE(now),
					positionmenuoptionvalidity.Or(
						positionmenuoptionvalidity.EndDateIsNil(),
						positionmenuoptionvalidity.EndDateGT(now),
					),
				).
				WithPositionMenuOption(func(q *ent.PositionMenuOptionQuery) {
					q.WithMenuOption(func(q *ent.MenuOptionQuery) {
						q.Where(menuoption.DeletedAtIsNil()).
							WithMenuOptionFunctions(func(q *ent.MenuOptionFunctionQuery) {
								q.WithSystemFunction(func(q *ent.SystemFunctionQuery) {
									q.Where(systemfunction.DeletedAtIsNil())
								})
							})
					})
				}).
				All(ctx)
			if err != nil {
				return repoError(err)
			}
			for _, v := range validities {
				if v.Edges.PositionMenuOption == nil || v.Edges.PositionMenuOption.Edges.MenuOption == nil {
					continue
				}
				for _, fn := range v.Edges.PositionMenuOption.Edges.MenuOption.Edges.MenuOptionFunctions {
					if fn.Edges.SystemFunction == nil {
						continue
					}
					codes[fn.Edges.SystemFunction.Code] = struct{}{}
				}
			}
		}

		if employeeFound && deptID != uuid.Nil {
			validities, err := client.DepartmentMenuOptionValidity.Query().
				Where(
					departmentmenuoptionvalidity.TenantID(tenantID),
					departmentmenuoptionvalidity.DepartmentID(deptID),
					departmentmenuoptionvalidity.StartDateLTE(now),
					departmentmenuoptionvalidity.Or(
						departmentmenuoptionvalidity.EndDateIsNil(),
						departmentmenuoptionvalidity.EndDateGT(now),
					),
				).
				WithDepartmentMenuOption(func(q *ent.DepartmentMenuOptionQuery) {
					q.WithMenuOption(func(q *ent.MenuOptionQuery) {
						q.Where(menuoption.DeletedAtIsNil()).
							WithMenuOptionFunctions(func(q *ent.MenuOptionFunctionQuery) {
								q.WithSystemFunction(func(q *ent.SystemFunctionQuery) {
									q.Where(systemfunction.DeletedAtIsNil())
								})
							})
					})
				}).
				All(ctx)
			if err != nil {
				return repoError(err)
			}
			for _, v := range validities {
				if v.Edges.DepartmentMenuOption == nil || v.Edges.DepartmentMenuOption.Edges.MenuOption == nil {
					continue
				}
				for _, fn := range v.Edges.DepartmentMenuOption.Edges.MenuOption.Edges.MenuOptionFunctions {
					if fn.Edges.SystemFunction == nil {
						continue
					}
					codes[fn.Edges.SystemFunction.Code] = struct{}{}
				}
			}
		}

		assignments, err := client.UserRoleAssignment.Query().
			Where(
				userroleassignment.TenantID(tenantID),
				userroleassignment.UserCredentialsID(userCredentialsID),
				userroleassignment.IsGranted(true),
			).
			WithTenantRole(func(q *ent.TenantRoleQuery) {
				q.Where(
					tenantrole.DeletedAtIsNil(),
					tenantrole.Or(
						tenantrole.ClientApplicationID(appClientID),
						tenantrole.ClientApplicationIDIsNil(),
					),
				).
					WithRoleMenuOptions(func(q *ent.RoleMenuOptionQuery) {
						q.WithMenuOption(func(q *ent.MenuOptionQuery) {
							q.Where(menuoption.DeletedAtIsNil()).
								WithMenuOptionFunctions(func(q *ent.MenuOptionFunctionQuery) {
									q.WithSystemFunction(func(q *ent.SystemFunctionQuery) {
										q.Where(systemfunction.DeletedAtIsNil())
									})
								})
						})
					})
			}).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		for _, a := range assignments {
			if a.Edges.TenantRole == nil {
				continue
			}
			for _, rmo := range a.Edges.TenantRole.Edges.RoleMenuOptions {
				if rmo.Edges.MenuOption == nil {
					continue
				}
				for _, fn := range rmo.Edges.MenuOption.Edges.MenuOptionFunctions {
					if fn.Edges.SystemFunction == nil {
						continue
					}
					codes[fn.Edges.SystemFunction.Code] = struct{}{}
				}
			}
		}

		if employeeFound {
			overrides, err := client.EmployeeMenuOptionOverride.Query().
				Where(
					employeemenuoptionoverride.TenantID(tenantID),
					employeemenuoptionoverride.EmployeePartyID(creds.PartyID),
					employeemenuoptionoverride.IsGranted(true),
				).
				WithSystemFunction(func(q *ent.SystemFunctionQuery) {
					q.Where(systemfunction.DeletedAtIsNil())
				}).
				All(ctx)
			if err != nil {
				return repoError(err)
			}
			for _, o := range overrides {
				if o.Edges.SystemFunction == nil {
					continue
				}
				codes[o.Edges.SystemFunction.Code] = struct{}{}
			}
		}

		result = make([]string, 0, len(codes))
		for code := range codes {
			result = append(result, code)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &domain.PermissionUniverse{Functions: result}, nil
}

type menuOptionLink struct {
	optionID  uuid.UUID
	sortOrder int
}

func (r *RBACRepository) GetMenuTree(ctx context.Context, tenantID, userCredentialsID, appClientID uuid.UUID) ([]domain.MenuTreeNode, error) {
	var tree []domain.MenuTreeNode

	err := r.withClient(ctx, func(client *ent.Client) error {
		universe, err := r.ResolvePermissionUniverse(ctx, tenantID, userCredentialsID, appClientID)
		if err != nil {
			return err
		}

		allowed := make(map[string]struct{}, len(universe.Functions))
		for _, code := range universe.Functions {
			allowed[code] = struct{}{}
		}

		options, err := client.MenuOption.Query().
			Where(
				menuoption.TenantID(tenantID),
				menuoption.DeletedAtIsNil(),
			).
			WithMenuOptionFunctions(func(q *ent.MenuOptionFunctionQuery) {
				q.WithSystemFunction(func(q *ent.SystemFunctionQuery) {
					q.Where(systemfunction.DeletedAtIsNil())
				})
			}).
			All(ctx)
		if err != nil {
			return repoError(err)
		}

		visibleOptions := make(map[uuid.UUID]*ent.MenuOption, len(options))
		for _, opt := range options {
			for _, fn := range opt.Edges.MenuOptionFunctions {
				if fn.Edges.SystemFunction == nil {
					continue
				}
				if _, ok := allowed[fn.Edges.SystemFunction.Code]; ok {
					visibleOptions[opt.ID] = opt
					break
				}
			}
		}

		categoryLinks, err := client.MenuOptionCategory.Query().
			Where(menuoptioncategory.TenantID(tenantID)).
			All(ctx)
		if err != nil {
			return repoError(err)
		}

		categoryOptions := make(map[uuid.UUID][]menuOptionLink, len(categoryLinks))
		for _, link := range categoryLinks {
			opt, ok := visibleOptions[link.MenuOptionID]
			if !ok {
				continue
			}
			categoryOptions[link.MenuCategoryID] = append(categoryOptions[link.MenuCategoryID], menuOptionLink{
				optionID:  opt.ID,
				sortOrder: link.SortOrder,
			})
		}

		for catID, links := range categoryOptions {
			sort.Slice(links, func(i, j int) bool {
				return links[i].sortOrder < links[j].sortOrder
			})
			categoryOptions[catID] = links
		}

		categories, err := client.MenuCategory.Query().
			Where(
				menucategory.TenantID(tenantID),
				menucategory.DeletedAtIsNil(),
			).
			All(ctx)
		if err != nil {
			return repoError(err)
		}

		categoryByID := make(map[uuid.UUID]*ent.MenuCategory, len(categories))
		childrenByParent := make(map[uuid.UUID][]*ent.MenuCategory, len(categories))
		for _, cat := range categories {
			categoryByID[cat.ID] = cat
			if cat.ParentID != nil {
				childrenByParent[*cat.ParentID] = append(childrenByParent[*cat.ParentID], cat)
			}
		}

		var build func(cat *ent.MenuCategory) domain.MenuTreeNode
		build = func(cat *ent.MenuCategory) domain.MenuTreeNode {
			node := domain.MenuTreeNode{
				ID:        cat.ID,
				Code:      cat.Code,
				Name:      cat.Name,
				SortOrder: cat.SortOrder,
			}

			links := categoryOptions[cat.ID]
			node.Options = make([]domain.MenuTreeOption, 0, len(links))
			for _, link := range links {
				opt := visibleOptions[link.optionID]
				node.Options = append(node.Options, domain.MenuTreeOption{
					ID:        opt.ID,
					Code:      opt.Code,
					Name:      opt.Name,
					Icon:      opt.Icon,
					Route:     opt.Route,
					SortOrder: link.sortOrder,
				})
			}

			children := childrenByParent[cat.ID]
			sort.Slice(children, func(i, j int) bool {
				return children[i].SortOrder < children[j].SortOrder
			})
			node.Children = make([]domain.MenuTreeNode, 0, len(children))
			for _, child := range children {
				node.Children = append(node.Children, build(child))
			}

			return node
		}

		roots := make([]*ent.MenuCategory, 0)
		for _, cat := range categories {
			if cat.ParentID == nil {
				roots = append(roots, cat)
			}
		}
		sort.Slice(roots, func(i, j int) bool {
			return roots[i].SortOrder < roots[j].SortOrder
		})

		tree = make([]domain.MenuTreeNode, 0, len(roots))
		for _, root := range roots {
			tree = append(tree, build(root))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// ----------------------------------------------------------------------------
// Mapping helpers
// ----------------------------------------------------------------------------

func toDomainSystemFunction(e *ent.SystemFunction) *domain.SystemFunction {
	return &domain.SystemFunction{
		ID:            e.ID,
		Code:          e.Code,
		Name:          e.Name,
		Description:   e.Description,
		FunctionScope: domain.FunctionScope(e.FunctionScope),
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
		DeletedAt:     e.DeletedAt,
	}
}

func toDomainMenuCategory(e *ent.MenuCategory) *domain.MenuCategory {
	return &domain.MenuCategory{
		ID:        e.ID,
		TenantID:  e.TenantID,
		ParentID:  e.ParentID,
		Code:      e.Code,
		Name:      e.Name,
		SortOrder: e.SortOrder,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
		DeletedAt: e.DeletedAt,
	}
}

func toDomainMenuOption(e *ent.MenuOption) *domain.MenuOption {
	return &domain.MenuOption{
		ID:         e.ID,
		TenantID:   e.TenantID,
		TemplateID: e.TemplateID,
		Code:       e.Code,
		Name:       e.Name,
		Icon:       e.Icon,
		Route:      e.Route,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
		DeletedAt:  e.DeletedAt,
	}
}

func toDomainTenantRole(e *ent.TenantRole) *domain.TenantRole {
	return &domain.TenantRole{
		ID:                  e.ID,
		TenantID:            e.TenantID,
		ClientApplicationID: e.ClientApplicationID,
		Name:                e.Name,
		Description:         e.Description,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
		DeletedAt:           e.DeletedAt,
	}
}

func toDomainUserRoleAssignment(e *ent.UserRoleAssignment) *domain.UserRoleAssignment {
	return &domain.UserRoleAssignment{
		ID:                e.ID,
		TenantID:          e.TenantID,
		UserCredentialsID: e.UserCredentialsID,
		TenantRoleID:      e.TenantRoleID,
		IsGranted:         e.IsGranted,
		AssignedAt:        e.AssignedAt,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}
}
