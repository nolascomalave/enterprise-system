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

// PasswordResetToken is a single-use password recovery token. Only its hash is
// stored; the plaintext token is delivered out of band.
type PasswordResetToken struct {
	ent.Schema
}

func (PasswordResetToken) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "password_reset_tokens", Schema: "tenants"},
	}
}

func (PasswordResetToken) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.Bytes("token_hash").
			Unique().
			Sensitive(),
		field.Time("expires_at"),
		field.Time("used_at").
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

func (PasswordResetToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("password_reset_tokens").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("password_reset_tokens").
			Field("party_id").
			Unique().
			Required(),
	}
}

func (PasswordResetToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id", "expires_at"),
		index.Fields("tenant_id", "expires_at"),
	}
}
