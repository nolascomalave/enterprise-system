package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

var locationCountryPattern = regexp.MustCompile(`^[A-Z]{2}$`)

// Location is a global geographic node. The hierarchy is represented both by
// parent_id and by the materialized ltree path.
type Location struct {
	ent.Schema
}

func (Location) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "locations", Schema: "global"},
	}
}

func (Location) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")),
		field.Enum("location_type").
			Values("REGION", "SUBREGION", "STATE", "PROVINCE", "CITY", "MUNICIPALITY", "COUNTY"),
		field.UUID("parent_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.String("country_code").
			NotEmpty().
			Match(locationCountryPattern),
		field.String("iso_code").
			Optional().
			Nillable(),
		field.String("external_id").
			Optional().
			Nillable(),
		field.String("default_name").
			NotEmpty().
			Annotations(entsql.Annotation{Collation: "unaccent_case_insensitive"}),
		field.String("path").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "ltree"}),
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

func (Location) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Location.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("parent", Location.Type).
			Ref("children").
			Field("parent_id").
			Unique(),
		edge.To("translations", LocationTranslation.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("addresses", Address.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Location) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("path").Unique(),
		index.Fields("country_code", "parent_id"),
		index.Fields("parent_id"),
		index.Fields("country_code", "location_type"),
		index.Fields("country_code", "iso_code").
			Unique().
			Annotations(entsql.IndexWhere("iso_code IS NOT NULL")),
		index.Fields("country_code", "external_id").
			Unique().
			Annotations(entsql.IndexWhere("external_id IS NOT NULL")),
	}
}
