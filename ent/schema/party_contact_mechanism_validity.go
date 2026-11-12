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

// PartyContactMechanismValidity is a time window in which a party contact
// mechanism applies. Dates were removed from PartyContactMechanism.
type PartyContactMechanismValidity struct {
	ent.Schema
}

func (PartyContactMechanismValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_contact_mechanism_validities", Schema: "tenants"},
	}
}

func (PartyContactMechanismValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_contact_mechanism_id", uuid.UUID{}),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("contact_mechanism_id", uuid.UUID{}),
		field.Enum("contact_type").
			Values("EMAIL", "PHONE", "MOBILE", "FAX", "WEBSITE"),
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

func (PartyContactMechanismValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_contact_mechanism_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party_contact_mechanism", PartyContactMechanism.Type).
			Ref("validities").
			Field("party_contact_mechanism_id").
			Unique().
			Required(),
	}
}

func (PartyContactMechanismValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_contact_mechanism_id", "start_date"),
		index.Fields("tenant_id", "party_contact_mechanism_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
		index.Fields("tenant_id", "party_id", "contact_type").
			Unique().
			Annotations(entsql.IndexWhere("is_primary AND end_date IS NULL")),
		index.Fields("tenant_id", "contact_mechanism_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
	}
}
