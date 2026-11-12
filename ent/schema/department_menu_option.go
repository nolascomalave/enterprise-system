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

// DepartmentMenuOption links a department to a menu option.
type DepartmentMenuOption struct {
	ent.Schema
}

func (DepartmentMenuOption) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_menu_options", Schema: "tenants"},
	}
}

func (DepartmentMenuOption) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("menu_option_id", uuid.UUID{}),
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

func (DepartmentMenuOption) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_menu_options").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("department_menu_options").
			Field("department_id").
			Unique().
			Required(),
		edge.From("menu_option", MenuOption.Type).
			Ref("department_menu_options").
			Field("menu_option_id").
			Unique().
			Required(),
		edge.To("validities", DepartmentMenuOptionValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (DepartmentMenuOption) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_id", "menu_option_id").
			Unique(),
	}
}
