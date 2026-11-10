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

// RoleType is a global catalog entry (global.role_types). It has no tenant_id
// and therefore no RLS.
type RoleType struct {
	ent.Schema
}

func (RoleType) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "role_types", Schema: "global"},
	}
}

func (RoleType) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")).
			Comment("UUIDv7 primary key"),
		field.String("code").
			NotEmpty().
			Comment("Stable code, e.g. OWNER or TENANT_ADMIN"),
		field.String("name").
			NotEmpty(),
		field.String("description").
			Optional(),
		field.Bool("is_system").
			Default(false).
			Comment("System roles cannot be deleted by tenants"),
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

func (RoleType) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("party_roles", PartyRole.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (RoleType) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
