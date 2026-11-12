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

// PartyContactMechanism is the tenant-scoped, time-bounded association between
// a Party and a global ContactMechanism.
type PartyContactMechanism struct {
	ent.Schema
}

func (PartyContactMechanism) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_contact_mechanisms", Schema: "tenants"},
	}
}

func (PartyContactMechanism) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("contact_mechanism_id", uuid.UUID{}),
		field.Bool("is_primary").
			Default(false),
		field.Bool("is_verified").
			Default(false),
		field.Int("contact_mechanism_order").
			NonNegative(),
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

func (PartyContactMechanism) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_contact_mechanisms").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("party_contact_mechanisms").
			Field("party_id").
			Unique().
			Required(),
		edge.From("contact_mechanism", ContactMechanism.Type).
			Ref("party_contact_mechanisms").
			Field("contact_mechanism_id").
			Unique().
			Required(),
		edge.To("credentials", UserCredentials.Type).
			Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("validities", PartyContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (PartyContactMechanism) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "contact_mechanism_order"),
		index.Fields("tenant_id", "contact_mechanism_id"),
	}
}
