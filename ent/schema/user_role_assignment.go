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

// UserRoleAssignment links a user to a tenant role.
type UserRoleAssignment struct {
	ent.Schema
}

func (UserRoleAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_role_assignments", Schema: "tenants"},
	}
}

func (UserRoleAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("user_credentials_id", uuid.UUID{}),
		field.UUID("tenant_role_id", uuid.UUID{}),
		field.Bool("is_granted").
			Default(true),
		field.Time("assigned_at").
			Default(time.Now),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (UserRoleAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("user_role_assignments").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("user_credentials", UserCredentials.Type).
			Ref("user_role_assignments").
			Field("user_credentials_id").
			Unique().
			Required(),
		edge.From("tenant_role", TenantRole.Type).
			Ref("user_role_assignments").
			Field("tenant_role_id").
			Unique().
			Required(),
	}
}

func (UserRoleAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "user_credentials_id", "tenant_role_id").
			Unique(),
	}
}
