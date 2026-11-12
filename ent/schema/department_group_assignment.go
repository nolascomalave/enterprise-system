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

// DepartmentGroupAssignment links a department to a corporate group. The
// composite primary key is (department_id, corporate_group_id); the id column
// exists for auditing.
type DepartmentGroupAssignment struct {
	ent.Schema
}

func (DepartmentGroupAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_group_assignments", Schema: "tenants"},
	}
}

func (DepartmentGroupAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("corporate_group_id", uuid.UUID{}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (DepartmentGroupAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_group_assignments").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("group_assignments").
			Field("department_id").
			Unique().
			Required(),
		edge.From("corporate_group", CorporateGroup.Type).
			Ref("department_assignments").
			Field("corporate_group_id").
			Unique().
			Required(),
	}
}

func (DepartmentGroupAssignment) Indexes() []ent.Index {
	return []ent.Index{
		// Mirrors the composite primary key of the migration.
		index.Fields("department_id", "corporate_group_id").
			Unique(),
		index.Fields("tenant_id", "corporate_group_id"),
		index.Fields("tenant_id", "department_id"),
	}
}
