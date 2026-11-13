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

// RoleMenuOption links a tenant role to a menu option.
type RoleMenuOption struct {
	ent.Schema
}

func (RoleMenuOption) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "role_menu_options", Schema: "tenants"},
	}
}

func (RoleMenuOption) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("tenant_role_id", uuid.UUID{}),
		field.UUID("menu_option_id", uuid.UUID{}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (RoleMenuOption) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("role_menu_options").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("tenant_role", TenantRole.Type).
			Ref("role_menu_options").
			Field("tenant_role_id").
			Unique().
			Required(),
		edge.From("menu_option", MenuOption.Type).
			Ref("role_menu_options").
			Field("menu_option_id").
			Unique().
			Required(),
	}
}

func (RoleMenuOption) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "tenant_role_id", "menu_option_id").
			Unique(),
	}
}
