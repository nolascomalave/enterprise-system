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

// TenantRelationship links a holding tenant with a subsidiary tenant.
type TenantRelationship struct {
	ent.Schema
}

func (TenantRelationship) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tenant_relationships", Schema: "global"},
	}
}

func (TenantRelationship) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id_holding", uuid.UUID{}),
		field.UUID("tenant_id_subsidiary", uuid.UUID{}),
		field.String("relationship_type").
			NotEmpty(),
		field.Enum("status").
			Values("PENDING", "APPROVED", "REJECTED", "CANCELED").
			Default("PENDING"),
		field.Time("requested_at").
			Default(time.Now).
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("approved_at").
			Optional().
			Nillable(),
		field.Time("rejected_at").
			Optional().
			Nillable(),
		field.Time("canceled_at").
			Optional().
			Nillable(),
		field.Time("ends_at").
			Optional().
			Nillable(),
		field.String("rejection_reason").
			Optional(),
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

func (TenantRelationship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("invites", TenantRelationshipInvite.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("transfers", TenantTransfer.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (TenantRelationship) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id_holding", "tenant_id_subsidiary", "relationship_type").
			Unique().
			Annotations(entsql.IndexWhere("status = 'PENDING'")),
		index.Fields("tenant_id_holding", "status"),
		index.Fields("tenant_id_subsidiary", "status"),
		index.Fields("status"),
	}
}
