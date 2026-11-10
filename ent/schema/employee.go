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

// Employee is a tenant-scoped employment record. Its PK is party_id: the party
// is the identity, and employment is a role over it. The SQL migration owns the
// FK from the party_id column to tenants.parties(id).
type Employee struct {
	ent.Schema
}

func (Employee) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "employees", Schema: "tenants"},
	}
}

func (Employee) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("party_id").
			Immutable().
			Comment("PK = party_id (1:1 with tenants.parties)"),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.String("employee_code").
			NotEmpty(),
		field.Time("hire_date"),
		field.Enum("status").
			Values("ACTIVE", "ON_LEAVE", "TERMINATED").
			Default("ACTIVE"),
		field.UUID("job_id", uuid.UUID{}),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("manager_party_id", uuid.UUID{}).
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

func (Employee) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("employees").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("job", Job.Type).
			Ref("employees").
			Field("job_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("employees").
			Field("department_id").
			Unique().
			Required(),
		edge.To("direct_reports", Employee.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("manager", Employee.Type).
			Ref("direct_reports").
			Field("manager_party_id").
			Unique(),
		edge.To("managed_departments", Department.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("assignments", EmployeeAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Employee) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "employee_code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "department_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "job_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "manager_party_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
