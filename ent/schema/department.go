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

// Department is a tenant-scoped recursive organizational unit.
type Department struct {
	ent.Schema
}

func (Department) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "departments", Schema: "tenants"},
	}
}

func (Department) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("parent_department_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("code").
			NotEmpty(),
		field.Enum("unit_type").
			Values("DIVISION", "BRANCH", "OFFICE", "DEPARTMENT").
			Default("DEPARTMENT").
			Comment("Organizational unit type"),
		field.UUID("manager_party_id", uuid.UUID{}).
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
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

func (Department) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("departments").
			Field("tenant_id").
			Unique().
			Required(),
		edge.To("children", Department.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("parent", Department.Type).
			Ref("children").
			Field("parent_department_id").
			Unique(),
		edge.From("manager", Employee.Type).
			Ref("managed_departments").
			Field("manager_party_id").
			Unique(),
		edge.To("employees", Employee.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("assignments", EmployeeAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("group_assignments", DepartmentGroupAssignment.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("contact_mechanisms", DepartmentContactMechanism.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("contact_mechanism_validities", DepartmentContactMechanismValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("addresses", DepartmentAddress.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("address_validities", DepartmentAddressValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("translations", DepartmentTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("department_menu_options", DepartmentMenuOption.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Department) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "parent_department_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "name").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "manager_party_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
