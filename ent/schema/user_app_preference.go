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

// UserAppPreference stores per-user, per-client-application UI preferences.
// The JSON object is intentionally flexible so each frontend can evolve its
// own settings without a migration.
type UserAppPreference struct {
	ent.Schema
}

func (UserAppPreference) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_app_preferences", Schema: "tenants"},
	}
}

func (UserAppPreference) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("user_credentials_id", uuid.UUID{}),
		field.UUID("client_application_id", uuid.UUID{}),
		field.JSON("preferences", map[string]any{}).
			Default(map[string]any{}),
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
			Nillable().
			Comment("Soft delete; null means active row"),
	}
}

func (UserAppPreference) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("user_app_preferences").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("user_credentials", UserCredentials.Type).
			Ref("app_preferences").
			Field("user_credentials_id").
			Unique().
			Required(),
		edge.From("client_application", ClientApplication.Type).
			Ref("user_preferences").
			Field("client_application_id").
			Unique().
			Required(),
	}
}

func (UserAppPreference) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "user_credentials_id", "client_application_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "user_credentials_id"),
		index.Fields("preferences").
			Annotations(entsql.IndexType("GIN")),
	}
}
