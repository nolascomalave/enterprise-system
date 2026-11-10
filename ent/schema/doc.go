// Package schema defines the Ent schemas for the Party Model.
//
// The global/ and tenants/ PostgreSQL schema annotations are honored by Atlas.
// Ent's default migrator is not schema-aware; use Atlas (or the SQL migrations)
// to create the tables in the correct PostgreSQL schemas. RLS policies are
// applied afterwards by migrations/0002_party_model_rls.sql.
package schema
