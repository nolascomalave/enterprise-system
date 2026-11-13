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

// PositionMenuOptionValidity is the independent validity window for a position
// menu option.
type PositionMenuOptionValidity struct {
	ent.Schema
}

func (PositionMenuOptionValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "position_menu_option_validities", Schema: "tenants"},
	}
}

func (PositionMenuOptionValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("position_menu_option_id", uuid.UUID{}),
		field.UUID("job_id", uuid.UUID{}),
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

func (PositionMenuOptionValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("position_menu_option_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("position_menu_option", PositionMenuOption.Type).
			Ref("validities").
			Field("position_menu_option_id").
			Unique().
			Required(),
	}
}

func (PositionMenuOptionValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "position_menu_option_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
		index.Fields("tenant_id", "job_id", "menu_option_id", "start_date"),
	}
}
