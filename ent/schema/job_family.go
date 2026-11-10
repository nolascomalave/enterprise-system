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

// JobFamily is the tenant-scoped catalog of job families.
type JobFamily struct {
	ent.Schema
}

func (JobFamily) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "job_families", Schema: "tenants"},
	}
}

func (JobFamily) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("code").
			NotEmpty(),
		field.String("description").
			Optional(),
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

func (JobFamily) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("job_families").
			Field("tenant_id").
			Unique().
			Required(),
		edge.To("jobs", Job.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (JobFamily) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "name").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
