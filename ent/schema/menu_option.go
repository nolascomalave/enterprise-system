package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// MenuOption is a tenant-scoped menu option.
type MenuOption struct {
	ent.Schema
}

func (MenuOption) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_options", Schema: "tenants"},
	}
}

func (MenuOption) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("template_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("icon").
			Optional(),
		field.String("route").
			Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

func (MenuOption) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("menu_options").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("template", MenuTemplate.Type).
			Ref("menu_options").
			Field("template_id").
			Unique(),
		edge.To("menu_option_functions", MenuOptionFunction.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("menu_option_categories", MenuOptionCategory.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("role_menu_options", RoleMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_menu_options", DepartmentMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("position_menu_options", PositionMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (MenuOption) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "template_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "name").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
