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

// MenuCategory is a tenant-scoped hierarchical menu category.
type MenuCategory struct {
	ent.Schema
}

func (MenuCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_categories", Schema: "tenants"},
	}
}

func (MenuCategory) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("code").
			NotEmpty(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.Int("sort_order").
			NonNegative().
			Default(0),
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

func (MenuCategory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("menu_categories").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("parent", MenuCategory.Type).
			Ref("children").
			Field("parent_id").
			Unique(),
		edge.To("children", MenuCategory.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("menu_option_categories", MenuOptionCategory.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (MenuCategory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "parent_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "sort_order").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
