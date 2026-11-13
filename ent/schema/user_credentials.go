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

// UserCredentials stores authentication material for a Party. Passwords and
// backup codes are only stored as bcrypt hashes.
type UserCredentials struct {
	ent.Schema
}

func (UserCredentials) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_credentials", Schema: "tenants"},
	}
}

func (UserCredentials) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.UUID("party_contact_mechanism_id", uuid.UUID{}),
		field.String("password_hash").
			NotEmpty().
			Sensitive(),
		field.String("totp_secret").
			Optional().
			Sensitive(),
		field.Bool("totp_enabled").
			Default(false),
		field.JSON("backup_codes_hash", []string{}).
			Default([]string{}),
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

func (UserCredentials) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("user_credentials").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("credentials").
			Field("party_id").
			Unique().
			Required(),
		edge.From("party_contact_mechanism", PartyContactMechanism.Type).
			Ref("credentials").
			Field("party_contact_mechanism_id").
			Unique().
			Required(),
		edge.To("user_role_assignments", UserRoleAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (UserCredentials) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("party_contact_mechanism_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
