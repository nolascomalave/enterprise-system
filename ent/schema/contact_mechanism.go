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

// ContactMechanism is a tenant-scoped email, phone, mobile, fax or website.
type ContactMechanism struct {
	ent.Schema
}

func (ContactMechanism) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "contact_mechanisms", Schema: "tenants"},
	}
}

func (ContactMechanism) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.Enum("type").
			Values("EMAIL", "PHONE", "MOBILE", "FAX", "WEBSITE"),
		field.String("value").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.Bool("is_primary").
			Default(false),
		field.Bool("is_verified").
			Default(false),
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

func (ContactMechanism) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("contact_mechanisms").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("contact_mechanisms").
			Field("party_id").
			Unique().
			Required(),
	}
}

func (ContactMechanism) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "type").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "type").
			Unique().
			Annotations(entsql.IndexWhere("is_primary AND deleted_at IS NULL")),
	}
}
