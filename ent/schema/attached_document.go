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

// AttachedDocument stores metadata for a file kept in an external content
// addressed storage (CAS). The content itself is never stored in PostgreSQL:
// storage_key is the SHA-256 hex digest and also the file name.
type AttachedDocument struct {
	ent.Schema
}

func (AttachedDocument) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "attached_documents", Schema: "tenants"},
	}
}

func (AttachedDocument) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.String("origin_table").
			NotEmpty().
			Comment("Business table that owns the document (polymorphic reference)"),
		field.UUID("origin_id", uuid.UUID{}).
			Comment("Row in origin_table (polymorphic reference, no FK)"),
		field.String("original_name").
			NotEmpty().
			Comment("Original file name supplied by the client"),
		field.String("storage_key").
			NotEmpty().
			Immutable().
			Comment("SHA-256 hex digest used as the storage object key"),
		field.String("storage_provider").
			NotEmpty().
			Comment("Storage backend identifier, e.g. local"),
		field.String("mime_type").
			NotEmpty().
			Comment("MIME type detected from the file magic bytes"),
		field.Int64("size_bytes").
			NonNegative(),
		field.Bytes("document_hash").
			NotEmpty().
			Immutable().
			Comment("Raw SHA-256 bytes; HASH partition key and dedup key"),
		field.String("description").
			Optional().
			Nillable(),
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

func (AttachedDocument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("attached_documents").
			Field("tenant_id").
			Unique().
			Required(),
	}
}

func (AttachedDocument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "origin_table", "origin_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "document_hash").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
