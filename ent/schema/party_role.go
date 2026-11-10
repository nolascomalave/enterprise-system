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

// PartyRole links a tenant-scoped Party with a global RoleType.
type PartyRole struct {
	ent.Schema
}

func (PartyRole) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_roles", Schema: "tenants"},
	}
}

func (PartyRole) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("role_type_id", uuid.UUID{}).
			Comment("FK to global.role_types"),
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

func (PartyRole) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_roles").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("party_roles").
			Field("party_id").
			Unique().
			Required(),
		edge.From("role_type", RoleType.Type).
			Ref("party_roles").
			Field("role_type_id").
			Unique().
			Required(),
	}
}

func (PartyRole) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "role_type_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "role_type_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
