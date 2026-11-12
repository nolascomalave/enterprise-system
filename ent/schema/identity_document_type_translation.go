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

// IdentityDocumentTypeTranslation is a localized name/description for a
// global identity document type.
type IdentityDocumentTypeTranslation struct {
	ent.Schema
}

func (IdentityDocumentTypeTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "identity_document_type_translations", Schema: "global"},
	}
}

func (IdentityDocumentTypeTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("identity_document_type_id", uuid.UUID{}),
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

func (IdentityDocumentTypeTranslation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("identity_document_type", IdentityDocumentType.Type).
			Ref("translations").
			Field("identity_document_type_id").
			Unique().
			Required(),
		edge.From("locale", Locale.Type).
			Ref("identity_document_type_translations").
			Field("locale_code").
			Unique().
			Required(),
	}
}

func (IdentityDocumentTypeTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("identity_document_type_id", "locale_code").Unique(),
		index.Fields("locale_code"),
	}
}
