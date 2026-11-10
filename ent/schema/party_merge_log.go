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

// PartyMergeLog is the append-only record of a source -> target identity merge.
type PartyMergeLog struct {
	ent.Schema
}

func (PartyMergeLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "party_merge_log", Schema: "tenants"},
	}
}

func (PartyMergeLog) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuidV7).
			Immutable().
			Annotations(entsql.DefaultExpr("uuidv7()")).
			Comment("UUIDv7 primary key"),
		field.UUID("tenant_id", uuid.UUID{}).
			Comment("Tenant discriminator; RLS key"),
		field.UUID("source_party_id", uuid.UUID{}).
			Comment("Party absorbed by the merge"),
		field.UUID("target_party_id", uuid.UUID{}).
			Comment("Golden Record that receives the merge"),
		field.Time("merged_at").
			Default(time.Now).
			Immutable().
			Annotations(entsql.DefaultExpr("now()")),
		field.UUID("merged_by", uuid.UUID{}).
			Optional().
			Nillable().
			Comment("Actor id from app.current_user_id; no FK"),
	}
}

func (PartyMergeLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Subscription.Type).
			Ref("party_merge_logs").
			Field("tenant_id").
			Unique().
			Required(),
		edge.From("source_party", Party.Type).
			Ref("merge_sources").
			Field("source_party_id").
			Unique().
			Required(),
		edge.From("target_party", Party.Type).
			Ref("merge_targets").
			Field("target_party_id").
			Unique().
			Required(),
	}
}

func (PartyMergeLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "merged_at"),
		index.Fields("tenant_id", "source_party_id"),
		index.Fields("tenant_id", "target_party_id"),
	}
}
