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

// TenantRelationshipInvite is an invitation tied to a tenant relationship.
// It deliberately has no tenant_id; the tenant is inherited from its relationship.
type TenantRelationshipInvite struct {
	ent.Schema
}

func (TenantRelationshipInvite) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tenant_relationship_invites", Schema: "global"},
	}
}

func (TenantRelationshipInvite) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("relationship_id", uuid.UUID{}),
		field.Bytes("token_hash").
			NotEmpty(),
		field.Time("expires_at"),
		field.Time("consumed_at").
			Optional().
			Nillable(),
		field.Time("canceled_at").
			Optional().
			Nillable(),
		field.UUID("canceled_by_tenant_id", uuid.UUID{}).
			Optional().
			Nillable(),
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

func (TenantRelationshipInvite) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("relationship", TenantRelationship.Type).
			Ref("invites").
			Field("relationship_id").
			Unique().
			Required(),
	}
}

func (TenantRelationshipInvite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash").
			Unique(),
		index.Fields("relationship_id").
			Annotations(entsql.IndexWhere("canceled_at IS NULL AND consumed_at IS NULL")),
		index.Fields("relationship_id").
			Annotations(entsql.IndexWhere("canceled_at IS NOT NULL")),
	}
}
