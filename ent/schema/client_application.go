package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ClientApplication is a tenant-owned OAuth 2.0 machine-to-machine client.
type ClientApplication struct {
	ent.Schema
}

func (ClientApplication) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "client_applications", Schema: "global"},
	}
}

func (ClientApplication) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("Null for PLATFORM applications"),
		field.String("name").
			NotEmpty(),
		field.String("client_id").
			NotEmpty().
			Unique(),
		field.String("client_secret_hash").
			NotEmpty().
			Sensitive(),
		field.Strings("scopes").
			SchemaType(map[string]string{dialect.Postgres: "text[]"}),
		field.Enum("client_type").
			Values("FIRST_PARTY", "THIRD_PARTY").
			Default("THIRD_PARTY"),
		field.Enum("application_scope").
			Values("PLATFORM", "TENANT_SPECIFIC").
			Default("TENANT_SPECIFIC"),
		field.Bool("is_active").
			Default(true),
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

func (ClientApplication) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("client_applications").
			Field("tenant_id").
			Unique(),
	}
}

func (ClientApplication) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
