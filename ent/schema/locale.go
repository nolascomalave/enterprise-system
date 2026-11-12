package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Locale is a supported BCP 47 locale. Exactly one row is the default.
type Locale struct {
	ent.Schema
}

func (Locale) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "locales", Schema: "global"},
	}
}

func (Locale) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			StorageKey("code").
			NotEmpty().
			Immutable(),
		field.String("name").
			NotEmpty(),
		field.Bool("is_default").
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

func (Locale) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("location_translations", LocationTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("identity_document_type_translations", IdentityDocumentTypeTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("role_type_translations", RoleTypeTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_translations", DepartmentTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}
