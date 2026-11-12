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

// LocationTranslation is a localized name for a global location.
type LocationTranslation struct {
	ent.Schema
}

func (LocationTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "location_translations", Schema: "global"},
	}
}

func (LocationTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("location_id", uuid.UUID{}),
		field.String("language_code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
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

func (LocationTranslation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("location", Location.Type).
			Ref("translations").
			Field("location_id").
			Unique().
			Required(),
	}
}

func (LocationTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("location_id", "language_code").Unique(),
		index.Fields("language_code"),
		index.Fields("name"),
	}
}
