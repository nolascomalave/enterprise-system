package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ContactMechanism is global master data. A (type, value) pair is unique across
// all tenants; tenant/party-specific data lives in PartyContactMechanism.
type ContactMechanism struct {
	ent.Schema
}

func (ContactMechanism) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "contact_mechanisms", Schema: "global"},
	}
}

func (ContactMechanism) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.Enum("type").
			Values("EMAIL", "PHONE", "MOBILE", "FAX", "WEBSITE"),
		field.String("value").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
	}
}

func (ContactMechanism) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("party_contact_mechanisms", PartyContactMechanism.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (ContactMechanism) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("type", "value").Unique(),
	}
}
