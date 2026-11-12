package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

var countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)

// Address is a tenant-scoped legal, fiscal, postal or delivery address.
type Address struct {
	ent.Schema
}

func (Address) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "addresses", Schema: "tenants"},
	}
}

func (Address) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("party_id", uuid.UUID{}),
		field.Enum("type").
			Values("LEGAL", "FISCAL", "POSTAL", "DELIVERY"),
		field.String("line1").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("line2").
			Optional().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.UUID("city_location_id", uuid.UUID{}),
		field.String("postal_code").
			Optional().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("country").
			NotEmpty().
			Match(countryCodePattern),
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

func (Address) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("addresses").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("party", Party.Type).
			Ref("addresses").
			Field("party_id").
			Unique().
			Required(),
		edge.From("city_location", Location.Type).
			Ref("addresses").
			Field("city_location_id").
			Unique().
			Required(),
		edge.To("validities", AddressValidity.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Address) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "party_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "party_id", "type").
			StorageKey("address_tenant_party_type_idx").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("tenant_id", "city_location_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
