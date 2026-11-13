package schema

import (
	"time"

	"enterprise-system/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Job is a tenant-scoped position inside a job family.
type Job struct {
	ent.Schema
}

func (Job) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "jobs", Schema: "tenants"},
	}
}

func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("job_family_id", uuid.UUID{}),
		field.String("title").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("code").
			NotEmpty(),
		field.String("description").
			Optional(),
		field.Other("min_salary", &domain.Money{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(14,2)"}).
			Optional().
			Nillable(),
		field.Other("max_salary", &domain.Money{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(14,2)"}).
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

func (Job) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("jobs").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("job_family", JobFamily.Type).
			Ref("jobs").
			Field("job_family_id").
			Unique().
			Required(),
		edge.To("employees", Employee.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("assignments", EmployeeAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("position_menu_options", PositionMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "job_family_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "title").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
