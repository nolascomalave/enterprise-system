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

// AddressValidity is a time window for a party address. It replaces the
// is_primary column that used to live in addresses.
type AddressValidity struct {
	ent.Schema
}

func (AddressValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "address_validities", Schema: "tenants"},
	}
}

func (AddressValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("address_id", uuid.UUID{}),
		field.Enum("type").
			Values("LEGAL", "FISCAL", "POSTAL", "DELIVERY"),
		field.Bool("is_primary").
			Default(false),
		field.Time("start_date"),
		field.Time("end_date").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

func (AddressValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("address_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("address_validities").
			Field("party_id").
			Unique().
			Required(),
		edge.From("address", Address.Type).
			Ref("validities").
			Field("address_id").
			Unique().
			Required(),
	}
}

func (AddressValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "type", "start_date"),
		index.Fields("tenant_id", "address_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL AND deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "type").
			Unique().
			Annotations(entsql.IndexWhere("is_primary AND end_date IS NULL AND deleted_at IS NULL")),
	}
}
