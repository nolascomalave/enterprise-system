package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// MenuOptionFunction links a menu option to a system function.
type MenuOptionFunction struct {
	ent.Schema
}

func (MenuOptionFunction) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_option_functions", Schema: "tenants"},
	}
}

func (MenuOptionFunction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("menu_option_id", uuid.UUID{}),
		field.UUID("system_function_id", uuid.UUID{}),
		field.Bool("is_required").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (MenuOptionFunction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("menu_option_functions").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("menu_option", MenuOption.Type).
			Ref("menu_option_functions").
			Field("menu_option_id").
			Unique().
			Required(),
		edge.From("system_function", SystemFunction.Type).
			Ref("menu_option_functions").
			Field("system_function_id").
			Unique().
			Required(),
	}
}
