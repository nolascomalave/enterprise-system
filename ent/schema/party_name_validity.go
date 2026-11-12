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

// PartyNameValidity is a time window in which a party name applies.
type PartyNameValidity struct {
	ent.Schema
}

func (PartyNameValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_name_validities", Schema: "tenants"},
	}
}

func (PartyNameValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_name_id", uuid.UUID{}),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("name_type_id", uuid.UUID{}),
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
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (PartyNameValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_name_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party_name", PartyName.Type).
			Ref("validities").
			Field("party_name_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("name_validities").
			Field("party_id").
			Unique().
			Required(),
		edge.From("name_type", NameType.Type).
			Ref("validities").
			Field("name_type_id").
			Unique().
			Required(),
	}
}

func (PartyNameValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_name_id", "start_date"),
		index.Fields("tenant_id", "party_name_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
		index.Fields("tenant_id", "party_id", "name_type_id").
			Unique().
			Annotations(entsql.IndexWhere("is_primary AND end_date IS NULL")),
	}
}
