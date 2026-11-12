package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

var nationalityCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// PartyNationality is a tenant-scoped nationality of a party.
type PartyNationality struct {
	ent.Schema
}

func (PartyNationality) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_nationalities", Schema: "tenants"},
	}
}

func (PartyNationality) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.String("country_code").
			NotEmpty().
			Match(nationalityCountryPattern),
		field.Bool("is_primary").
			Default(false),
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

func (PartyNationality) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_nationalities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("nationalities").
			Field("party_id").
			Unique().
			Required(),
	}
}

func (PartyNationality) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "country_code").Unique(),
		index.Fields("tenant_id", "party_id").
			Unique().
			Annotations(entsql.IndexWhere("is_primary")),
		index.Fields("tenant_id", "party_id"),
	}
}
