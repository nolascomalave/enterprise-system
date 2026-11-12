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

// EmployeeMenuOptionOverride allows granting or denying a system function to an
// employee regardless of role.
type EmployeeMenuOptionOverride struct {
	ent.Schema
}

func (EmployeeMenuOptionOverride) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "employee_menu_option_overrides", Schema: "tenants"},
	}
}

func (EmployeeMenuOptionOverride) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("employee_party_id", uuid.UUID{}),
		field.UUID("system_function_id", uuid.UUID{}),
		field.Bool("is_granted").
			Default(true),
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

func (EmployeeMenuOptionOverride) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("employee_menu_option_overrides").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("employee", Employee.Type).
			Ref("employee_menu_option_overrides").
			Field("employee_party_id").
			Unique().
			Required(),
		edge.From("system_function", SystemFunction.Type).
			Ref("employee_menu_option_overrides").
			Field("system_function_id").
			Unique().
			Required(),
	}
}

func (EmployeeMenuOptionOverride) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "employee_party_id", "system_function_id").
			Unique(),
	}
}
