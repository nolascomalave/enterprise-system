package repository

import (
	"context"

	"enterprise-system/ent"
	"enterprise-system/ent/roletype"
	"enterprise-system/ent/roletypetranslation"
	"enterprise-system/internal/domain"
	"github.com/google/uuid"
)

var _ domain.RoleTypeRepository = (*RoleTypeRepository)(nil)

type RoleTypeRepository struct {
	client *ent.Client
}

func NewRoleTypeRepository(client *ent.Client) *RoleTypeRepository {
	return &RoleTypeRepository{client: client}
}

func (r *RoleTypeRepository) ListRoleTypes(ctx context.Context) ([]*domain.RoleType, error) {
	var result []*domain.RoleType
	err := execute(ctx, r.client, func(client *ent.Client) error {
		rows, err := client.RoleType.Query().
			Where(roletype.DeletedAtIsNil()).
			Order(ent.Asc(roletype.FieldCode)).
			All(ctx)
		if err != nil {
			return repoError(err)
		}
		result = make([]*domain.RoleType, 0, len(rows))
		for _, row := range rows {
			result = append(result, &domain.RoleType{
				ID: row.ID, Code: row.Code, IsSystem: row.IsSystem,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
			})
		}
		return nil
	})
	return result, err
}

func (r *RoleTypeRepository) GetRoleTypeTranslation(
	ctx context.Context,
	roleTypeID uuid.UUID,
	locale string,
) (string, string, error) {
	var name, description string
	err := execute(ctx, r.client, func(client *ent.Client) error {
		row, err := client.RoleTypeTranslation.Query().
			Where(
				roletypetranslation.RoleTypeID(roleTypeID),
				roletypetranslation.LocaleCodeEQ(locale),
			).
			Only(ctx)
		if err != nil {
			return repoError(err)
		}
		name = row.Name
		if row.Description != nil {
			description = *row.Description
		}
		return nil
	})
	if ent.IsNotFound(err) {
		return "", "", domain.ErrTranslationNotFound
	}
	return name, description, err
}
