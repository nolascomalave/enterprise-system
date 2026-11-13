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

// DepartmentMenuOptionValidity is the independent validity window for a
// department menu option.
type DepartmentMenuOptionValidity struct {
	ent.Schema
}

func (DepartmentMenuOptionValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_menu_option_validities", Schema: "tenants"},
	}
}

func (DepartmentMenuOptionValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_menu_option_id", uuid.UUID{}),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("menu_option_id", uuid.UUID{}),
		field.Time("start_date"),
		field.Time("end_date").
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
	}
}

func (DepartmentMenuOptionValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_menu_option_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department_menu_option", DepartmentMenuOption.Type).
			Ref("validities").
			Field("department_menu_option_id").
			Unique().
			Required(),
	}
}

func (DepartmentMenuOptionValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_menu_option_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
		index.Fields("tenant_id", "department_id", "menu_option_id", "start_date"),
	}
}
