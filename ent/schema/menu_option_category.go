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

// MenuOptionCategory links a menu option to a menu category.
type MenuOptionCategory struct {
	ent.Schema
}

func (MenuOptionCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "menu_option_categories", Schema: "tenants"},
	}
}

func (MenuOptionCategory) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("menu_category_id", uuid.UUID{}),
		field.UUID("menu_option_id", uuid.UUID{}),
		field.Int("sort_order").
			NonNegative().
			Default(0),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
	}
}

func (MenuOptionCategory) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("menu_option_categories").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("menu_category", MenuCategory.Type).
			Ref("menu_option_categories").
			Field("menu_category_id").
			Unique().
			Required(),
		edge.From("menu_option", MenuOption.Type).
			Ref("menu_option_categories").
			Field("menu_option_id").
			Unique().
			Required(),
	}
}

func (MenuOptionCategory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "menu_category_id", "menu_option_id").
			Unique(),
	}
}
