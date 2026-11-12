package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// NameType is the global catalog of name kinds (FIRST, LAST, LEGAL, ...).
type NameType struct {
	ent.Schema
}

func (NameType) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "name_types", Schema: "global"},
	}
}

func (NameType) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.String("code").
			NotEmpty().
			Unique(),
		field.String("name").
			NotEmpty(),
		field.String("description").
			Optional().
			Nillable(),
		field.Bool("is_system").
			Default(true),
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

func (NameType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("party_names", PartyName.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("validities", PartyNameValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}
