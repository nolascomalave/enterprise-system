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

// PersonProfile is the 1:1 composition of Party for INDIVIDUAL parties. It has
// its own UUIDv7 primary key; party_id is unique.
type PersonProfile struct {
	ent.Schema
}

func (PersonProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "person_profiles", Schema: "tenants"},
	}
}

func (PersonProfile) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}).
			Comment("1:1 FK to tenants.parties; UNIQUE is the documented exception to tenant-first indexes"),
		field.String("first_name").
			NotEmpty(),
		field.String("middle_name").
			Optional(),
		field.String("last_name").
			NotEmpty(),
		field.String("second_last_name").
			Optional(),
		field.Time("birth_date").
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
			Nillable(),
	}
}

func (PersonProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("person_profiles").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("person_profile").
			Field("party_id").
			Unique().
			Required(),
	}
}

func (PersonProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
