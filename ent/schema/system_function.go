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

// SystemFunction is a global catalog of platform and tenant functions.
type SystemFunction struct {
	ent.Schema
}

func (SystemFunction) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "system_functions", Schema: "global"},
	}
}

func (SystemFunction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.String("code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("description").
			Optional(),
		field.Enum("function_scope").
			Values("PLATFORM", "TENANT"),
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

func (SystemFunction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("menu_template_functions", MenuTemplateFunction.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("menu_option_functions", MenuOptionFunction.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("employee_menu_option_overrides", EmployeeMenuOptionOverride.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (SystemFunction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("function_scope").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
