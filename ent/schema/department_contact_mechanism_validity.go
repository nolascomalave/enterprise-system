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

// DepartmentContactMechanismValidity is the independent validity window for a
// department contact.
type DepartmentContactMechanismValidity struct {
	ent.Schema
}

func (DepartmentContactMechanismValidity) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_contact_mechanism_validities", Schema: "tenants"},
	}
}

func (DepartmentContactMechanismValidity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_contact_mechanism_id", uuid.UUID{}),
		field.UUID("department_id", uuid.UUID{}),
		field.Enum("contact_type").
			Values("EMAIL", "PHONE", "MOBILE", "FAX", "WEBSITE"),
		field.Bool("is_primary").
			Default(false),
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

func (DepartmentContactMechanismValidity) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_contact_mechanism_validities").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("contact_mechanism_validities").
			Field("department_id").
			Unique().
			Required(),
		edge.From("department_contact_mechanism", DepartmentContactMechanism.Type).
			Ref("validities").
			Field("department_contact_mechanism_id").
			Unique().
			Required(),
	}
}

func (DepartmentContactMechanismValidity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_id", "start_date"),
		index.Fields("tenant_id", "department_contact_mechanism_id").
			Unique().
			Annotations(entsql.IndexWhere("end_date IS NULL")),
		index.Fields("tenant_id", "department_id", "contact_type").
			Unique().
			Annotations(entsql.IndexWhere("is_primary AND end_date IS NULL")),
	}
}
