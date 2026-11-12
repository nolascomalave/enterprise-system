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

// TenantRole is a tenant-scoped role for RBAC.
type TenantRole struct {
	ent.Schema
}

func (TenantRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tenant_roles", Schema: "tenants"},
	}
}

func (TenantRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("client_application_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("description").
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

func (TenantRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("tenant_roles").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("client_application", ClientApplication.Type).
			Ref("tenant_roles").
			Field("client_application_id").
			Unique(),
		edge.To("role_menu_options", RoleMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("user_role_assignments", UserRoleAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (TenantRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "client_application_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
