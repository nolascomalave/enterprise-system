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

// OrganizationProfile is the 1:1 composition of Party for ORGANIZATION
// parties. It has its own UUIDv7 primary key; party_id is unique.
type OrganizationProfile struct {
	ent.Schema
}

func (OrganizationProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "organization_profiles", Schema: "tenants"},
	}
}

func (OrganizationProfile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}).
			Comment("1:1 FK to tenants.parties; UNIQUE is the documented exception to tenant-first indexes"),
		field.String("legal_name").
			NotEmpty(),
		field.String("trade_name").
			Optional(),
		field.String("tax_id").
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

func (OrganizationProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("organization_profiles").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("organization_profile").
			Field("party_id").
			Unique().
			Required(),
	}
}

func (OrganizationProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "tax_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
