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

// Subscription is a tenant. It lives in global.subscriptions and has no RLS.
type Subscription struct {
	ent.Schema
}

func (Subscription) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscriptions", Schema: "global"},
	}
}

func (Subscription) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")).
			Comment("UUIDv7 primary key"),
		field.String("name").
			NotEmpty().
			Comment("Commercial name of the tenant"),
		field.String("slug").
			NotEmpty().
			Comment("Stable, URL-safe tenant identifier"),
		field.Enum("status").
			Values("ACTIVE", "SUSPENDED", "CANCELLED").
			Default("ACTIVE").
			Comment("Subscription lifecycle status"),
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

func (Subscription) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("parties", Party.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("person_profiles", PersonProfile.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("organization_profiles", OrganizationProfile.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_roles", PartyRole.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_merge_logs", PartyMergeLog.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("job_families", JobFamily.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("jobs", Job.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("employees", Employee.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("departments", Department.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("employee_assignments", EmployeeAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_contact_mechanisms", PartyContactMechanism.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("addresses", Address.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("sessions", Session.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("user_credentials", UserCredentials.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("password_reset_tokens", PasswordResetToken.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("attached_documents", AttachedDocument.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("corporate_groups", CorporateGroup.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_group_assignments", DepartmentGroupAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_names", PartyName.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_name_validities", PartyNameValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_contact_mechanism_validities", PartyContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_nationalities", PartyNationality.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_identity_documents", PartyIdentityDocument.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("party_identity_document_validities", PartyIdentityDocumentValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("address_validities", AddressValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_contact_mechanisms", DepartmentContactMechanism.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_contact_mechanism_validities", DepartmentContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_addresses", DepartmentAddress.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_address_validities", DepartmentAddressValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Subscription) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
