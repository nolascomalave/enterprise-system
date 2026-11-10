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

// EmployeeAssignment is the tenant-scoped employment history. The overlap and
// one-open-assignment guarantees are database constraints:
//   - EXCLUDE USING GIST over tstzrange(start_date, COALESCE(end_date,'infinity'),'[)')
//   - UNIQUE (tenant_id, employee_party_id) WHERE end_date IS NULL
type EmployeeAssignment struct {
	ent.Schema
}

func (EmployeeAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "employee_assignments", Schema: "tenants"},
	}
}

func (EmployeeAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("employee_party_id", uuid.UUID{}),
		field.UUID("job_id", uuid.UUID{}),
		field.UUID("department_id", uuid.UUID{}),
		field.Time("start_date"),
		field.Time("end_date").
			Optional().
			Nillable(),
		field.Other("salary", &domain.Money{}).
			SchemaType(map[string]string{dialect.Postgres: "numeric(14,2)"}),
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

func (EmployeeAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("employee_assignments").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("employee", Employee.Type).
			Ref("assignments").
			Field("employee_party_id").
			Unique().
			Required(),
		edge.From("job", Job.Type).
			Ref("assignments").
			Field("job_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("assignments").
			Field("department_id").
			Unique().
			Required(),
	}
}

func (EmployeeAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "employee_party_id", "start_date"),
		index.Fields("tenant_id", "job_id"),
		index.Fields("tenant_id", "department_id"),
		index.Fields("tenant_id", "employee_party_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
	}
}
