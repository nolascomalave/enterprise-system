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

// RoleTypeTranslation is a localized name/description for a global role type.
type RoleTypeTranslation struct {
	ent.Schema
}

func (RoleTypeTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "role_type_translations", Schema: "global"},
	}
}

func (RoleTypeTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("role_type_id", uuid.UUID{}),
		field.String("locale_code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("description").
			Optional().
			Nillable().
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

func (RoleTypeTranslation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("role_type", RoleType.Type).
			Ref("translations").
			Field("role_type_id").
			Unique().
			Required(),
		edge.From("locale", Locale.Type).
			Ref("role_type_translations").
			Field("locale_code").
			Unique().
			Required(),
	}
}

func (RoleTypeTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("role_type_id", "locale_code").Unique(),
		index.Fields("locale_code"),
	}
}
