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

// PartyIdentityDocument is the master identity document row. Its dates live in
// PartyIdentityDocumentValidity.
type PartyIdentityDocument struct {
	ent.Schema
}

func (PartyIdentityDocument) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_identity_documents", Schema: "tenants"},
	}
}

func (PartyIdentityDocument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("document_type_id", uuid.UUID{}),
		field.String("issuing_country_code").
			NotEmpty(),
		field.UUID("issuing_jurisdiction_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("document_number").
			NotEmpty(),
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
			Nillable(),
	}
}

func (PartyIdentityDocument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_identity_documents").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("identity_documents").
			Field("party_id").
			Unique().
			Required(),
		edge.From("document_type", IdentityDocumentType.Type).
			Ref("documents").
			Field("document_type_id").
			Unique().
			Required(),
		edge.From("issuing_jurisdiction", Location.Type).
			Ref("identity_documents").
			Field("issuing_jurisdiction_id").
			Unique(),
		edge.To("validities", PartyIdentityDocumentValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (PartyIdentityDocument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "document_type_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "issuing_country_code").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "issuing_jurisdiction_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "document_type_id", "issuing_country_code", "document_number").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
