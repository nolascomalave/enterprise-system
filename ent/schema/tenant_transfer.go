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

// TenantTransfer tracks a subsidiary transfer between holdings.
type TenantTransfer struct {
	ent.Schema
}

func (TenantTransfer) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tenant_transfers", Schema: "global"},
	}
}

func (TenantTransfer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("relationship_id", uuid.UUID{}),
		field.UUID("requested_by_tenant_id", uuid.UUID{}),
		field.Enum("status").
			Values(
				"TRANSFER_REQUESTED",
				"TRANSFER_IN_PROGRESS",
				"TRANSFER_COMPLETED",
				"TRANSFER_FAILED",
				"TRANSFER_CANCELED",
			).
			Default("TRANSFER_REQUESTED"),
		field.Time("requested_at").
			Default(time.Now).
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("started_at").
			Optional().
			Nillable(),
		field.Time("completed_at").
			Optional().
			Nillable(),
		field.Time("canceled_at").
			Optional().
			Nillable(),
		field.UUID("canceled_by_tenant_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("transfer_reason").
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

func (TenantTransfer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("relationship", TenantRelationship.Type).
			Ref("transfers").
			Field("relationship_id").
			Unique().
			Required(),
	}
}

func (TenantTransfer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("relationship_id"),
		index.Fields("requested_by_tenant_id"),
		index.Fields("status"),
	}
}
