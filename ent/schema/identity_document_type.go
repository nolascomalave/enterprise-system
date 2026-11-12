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

// IdentityDocumentType is the global catalog of identity document kinds.
type IdentityDocumentType struct {
	ent.Schema
}

func (IdentityDocumentType) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "identity_document_types", Schema: "global"},
	}
}

func (IdentityDocumentType) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.String("code").
			NotEmpty().
			Unique(),
		field.Enum("scope").
			Values("PERSON", "ORGANIZATION").
			Default("PERSON"),
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

func (IdentityDocumentType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("documents", PartyIdentityDocument.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("translations", IdentityDocumentTypeTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}
