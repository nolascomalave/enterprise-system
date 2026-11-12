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

// Party is the tenant-scoped identity root (tenants.parties). Profile data is
// composed through 1:1 person_profiles / organization_profiles tables; there is
// no table inheritance.
type Party struct {
	ent.Schema
}

func (Party) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "parties", Schema: "tenants"},
	}
}

func (Party) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")).
			Comment("UUIDv7 primary key"),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.Enum("party_type").
			Values("INDIVIDUAL", "ORGANIZATION").
			Comment("Discriminates the 1:1 profile table"),
		field.String("display_name").
			NotEmpty(),
		field.UUID("canonical_party_id", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("Golden Record final dentro del mismo tenant"),
		field.Enum("record_status").
			Values("ACTIVE", "MERGED", "ARCHIVED").
			Default("ACTIVE").
			Comment("ACTIVE, MERGED o ARCHIVED"),
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

func (Party) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("parties").
			Field("tenant_id").
			Unique().
			Required(),
		edge.To("person_profile", PersonProfile.Type).
			Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("organization_profile", OrganizationProfile.Type).
			Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_roles", PartyRole.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_contact_mechanisms", PartyContactMechanism.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("names", PartyName.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("name_validities", PartyNameValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("nationalities", PartyNationality.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("identity_documents", PartyIdentityDocument.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("identity_document_validities", PartyIdentityDocumentValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("addresses", Address.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("sessions", Session.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("credentials", UserCredentials.Type).
			Unique().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("password_reset_tokens", PasswordResetToken.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_names", PartyName.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_name_validities", PartyNameValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_contact_mechanism_validities", PartyContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("merged_parties", Party.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("canonical_party", Party.Type).
			Ref("merged_parties").
			Field("canonical_party_id").
			Unique(),
		edge.To("merge_sources", PartyMergeLog.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("merge_targets", PartyMergeLog.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Party) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
		index.Fields("tenant_id", "party_type").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "canonical_party_id"),
	}
}
