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

// DepartmentAddress is a department-scoped address referencing a global
// location for its city.
type DepartmentAddress struct {
	ent.Schema
}

func (DepartmentAddress) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "department_addresses", Schema: "tenants"},
	}
}

func (DepartmentAddress) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("department_id", uuid.UUID{}),
		field.UUID("city_location_id", uuid.UUID{}),
		field.Enum("type").
			Values("LEGAL", "FISCAL", "POSTAL", "DELIVERY"),
		field.String("line1").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("line2").
			Optional().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
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

func (DepartmentAddress) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("department_addresses").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("department", Department.Type).
			Ref("addresses").
			Field("department_id").
			Unique().
			Required(),
		edge.From("city_location", Location.Type).
			Ref("department_addresses").
			Field("city_location_id").
			Unique().
			Required(),
		edge.To("validities", DepartmentAddressValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (DepartmentAddress) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "department_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "city_location_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
