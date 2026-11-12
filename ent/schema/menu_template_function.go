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

// MenuTemplateFunction links a menu template to its required system functions.
type MenuTemplateFunction struct {
	ent.Schema
}

func (MenuTemplateFunction) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_template_functions", Schema: "global"},
	}
}

func (MenuTemplateFunction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("template_id", uuid.UUID{}),
		field.UUID("system_function_id", uuid.UUID{}),
		field.Bool("is_required").
			Default(false),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (MenuTemplateFunction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("template", MenuTemplate.Type).
			Ref("menu_template_functions").
			Field("template_id").
			Unique().
			Required(),
		edge.From("system_function", SystemFunction.Type).
			Ref("menu_template_functions").
			Field("system_function_id").
			Unique().
			Required(),
	}
}
