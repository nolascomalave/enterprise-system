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

// PartyName stores a party name value. Dates live in PartyNameValidity so a
// value can be valid, then stop being valid and become valid again later.
type PartyName struct {
	ent.Schema
}

func (PartyName) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_names", Schema: "tenants"},
	}
}

func (PartyName) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("name_type_id", uuid.UUID{}),
		field.String("value").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.Bool("is_primary").
			Default(false),
		field.Int("ordering").
			NonNegative().
			Default(0),
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
			Nillable().
			Comment("Soft delete; null means active row"),
	}
}

func (PartyName) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_names").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("names").
			Field("party_id").
			Unique().
			Required(),
		edge.From("name_type", NameType.Type).
			Ref("party_names").
			Field("name_type_id").
			Unique().
			Required(),
		edge.To("validities", PartyNameValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (PartyName) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "name_type_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "name_type_id", "value").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
