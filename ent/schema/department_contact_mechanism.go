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

// DepartmentContactMechanism links a department to a global contact.
type DepartmentContactMechanism struct {
	ent.Schema
}

func (DepartmentContactMechanism) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_contact_mechanisms", Schema: "tenants"},
	}
}

func (DepartmentContactMechanism) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("contact_mechanism_id", uuid.UUID{}),
		field.Int("contact_mechanism_order").
			NonNegative().
			Default(0),
		field.Bool("is_primary").
			Default(false),
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

func (DepartmentContactMechanism) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_contact_mechanisms").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("contact_mechanisms").
			Field("department_id").
			Unique().
			Required(),
		edge.From("contact_mechanism", ContactMechanism.Type).
			Ref("department_contact_mechanisms").
			Field("contact_mechanism_id").
			Unique().
			Required(),
		edge.To("validities", DepartmentContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (DepartmentContactMechanism) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "contact_mechanism_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
