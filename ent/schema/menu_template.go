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

// MenuTemplate is a global menu template shared by tenants.
type MenuTemplate struct {
	ent.Schema
}

func (MenuTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_templates", Schema: "global"},
	}
}

func (MenuTemplate) Fields() []ent.Field {
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
		field.Enum("client_type").
			Values("FIRST_PARTY", "THIRD_PARTY"),
		field.Enum("application_scope").
			Values("PLATFORM", "TENANT_SPECIFIC"),
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

func (MenuTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("menu_template_functions", MenuTemplateFunction.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("menu_options", MenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (MenuTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
