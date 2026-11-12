-- Verificación de la base persistente creada con Atlas + atlas_post_schema.sql
\echo '=== Base tables (audit partition children excluded) ==='
SELECT schemaname, count(*) AS tables
FROM pg_tables t
WHERE schemaname IN ('global','tenants','audit')
  AND NOT EXISTS (
    SELECT 1 FROM pg_inherits i
    WHERE i.inhrelid = (quote_ident(t.schemaname)||'.'||quote_ident(t.tablename))::regclass
  )
GROUP BY schemaname
ORDER BY schemaname;

SELECT count(*) AS total_base_tables
FROM pg_tables t
WHERE schemaname IN ('global','tenants','audit')
  AND NOT EXISTS (
    SELECT 1 FROM pg_inherits i
    WHERE i.inhrelid = (quote_ident(t.schemaname)||'.'||quote_ident(t.tablename))::regclass
  );

\echo '=== Required extension / collation ==='
SELECT extname, extversion FROM pg_extension WHERE extname='btree_gist';
SELECT collname, collprovider::text, collisdeterministic, colllocale
FROM pg_collation WHERE collname='unaccent_case_insensitive';

\echo '=== Collated table columns ==='
SELECT n.nspname||'.'||c.relname||'.'||a.attname AS column_name, co.collname
FROM pg_attribute a
JOIN pg_class c ON c.oid=a.attrelid
JOIN pg_namespace n ON n.oid=c.relnamespace
JOIN pg_collation co ON co.oid=a.attcollation
WHERE a.attnum>0 AND NOT a.attisdropped AND c.relkind='r'
  AND n.nspname IN ('global','tenants')
  AND co.collname='unaccent_case_insensitive'
ORDER BY 1;

\echo '=== RLS coverage ==='
SELECT count(*) AS tenant_tables,
       count(*) FILTER (WHERE c.relrowsecurity) AS rls_enabled,
       count(*) FILTER (WHERE c.relforcerowsecurity) AS rls_forced
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='tenants' AND c.relkind='r';

\echo '=== Audit ==='
SELECT count(*) AS audit_relations FROM pg_tables WHERE schemaname='audit';
SELECT count(DISTINCT trigger_name) AS distinct_audit_triggers
FROM information_schema.triggers
WHERE trigger_schema='tenants' AND trigger_name LIKE '%audit_trg';

\echo '=== Roles ==='
SELECT code, name FROM global.role_types ORDER BY code;
