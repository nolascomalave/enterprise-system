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

// DepartmentTranslation is a tenant-scoped translation for a department.
type DepartmentTranslation struct {
	ent.Schema
}

func (DepartmentTranslation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_translations", Schema: "tenants"},
	}
}

func (DepartmentTranslation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_id", uuid.UUID{}),
		field.String("locale_code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("description").
			Optional().
			Nillable().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
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

func (DepartmentTranslation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_translations").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("translations").
			Field("department_id").
			Unique().
			Required(),
		edge.From("locale", Locale.Type).
			Ref("department_translations").
			Field("locale_code").
			Unique().
			Required(),
	}
}

func (DepartmentTranslation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_id", "locale_code").Unique(),
		index.Fields("tenant_id", "department_id"),
		index.Fields("locale_code"),
	}
}
