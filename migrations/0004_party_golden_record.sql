-- ============================================================================
-- 0004_party_golden_record.sql
-- Golden Record y Path Compression para tenants.parties.
--
-- Añade:
--   * canonical_party_id (self-FK) y record_status.
--   * índice (tenant_id, canonical_party_id).
--   * tenants.party_merge_log con RLS.
--   * tenants.resolve_golden_party(uuid) con WITH RECURSIVE.
--
-- Prerrequisitos:
--   * tenants.parties y global.subscriptions
--   * PostgreSQL 18+ (uuidv7())
--
-- Ejecución atómica recomendada:
--   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 --single-transaction \
--        -f migrations/0004_party_golden_record.sql
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF to_regclass('global.subscriptions') IS NULL THEN
    v_missing := array_append(v_missing, 'global.subscriptions');
  END IF;
  IF to_regclass('tenants.parties') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.parties');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede aplicar Golden Record; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica primero el Módulo 1 (tenants.parties) o apunta DATABASE_URL a la base correcta.';
  END IF;

  IF EXISTS (
    SELECT 1 FROM pg_attribute
     WHERE attrelid = 'tenants.parties'::regclass
       AND attname = 'canonical_party_id'
       AND NOT attisdropped
  ) THEN
    RAISE EXCEPTION 'tenants.parties.canonical_party_id ya existe; esta migración no es reejecutable';
  END IF;

  IF to_regclass('tenants.party_merge_log') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_merge_log ya existe; esta migración no es reejecutable';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Golden Record en tenants.parties
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.parties
  ADD COLUMN canonical_party_id uuid,
  ADD COLUMN record_status text NOT NULL DEFAULT 'ACTIVE';

ALTER TABLE tenants.parties
  ADD CONSTRAINT parties_canonical_party_id_fkey
    FOREIGN KEY (canonical_party_id) REFERENCES tenants.parties (id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  ADD CONSTRAINT parties_record_status_chk
    CHECK (record_status IN ('ACTIVE', 'MERGED', 'ARCHIVED')),
  ADD CONSTRAINT parties_canonical_not_self_chk
    CHECK (canonical_party_id IS NULL OR canonical_party_id <> id);

CREATE INDEX parties_tenant_canonical_idx
  ON tenants.parties (tenant_id, canonical_party_id);

COMMENT ON COLUMN tenants.parties.canonical_party_id IS
  'Golden Record final dentro del mismo tenant; NULL en el registro canónico.';
COMMENT ON COLUMN tenants.parties.record_status IS
  'ACTIVE, MERGED o ARCHIVED. Un MERGED tiene canonical_party_id y deleted_at.';
COMMENT ON INDEX tenants.parties_tenant_canonical_idx IS
  'Resolución y compresión de cadenas de fusión por tenant.';

-- ----------------------------------------------------------------------------
-- Log append-only de fusiones
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.party_merge_log (
  id              uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id       uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  source_party_id uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  target_party_id uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  merged_at       timestamptz NOT NULL DEFAULT now(),
  merged_by       uuid,

  CONSTRAINT party_merge_log_distinct_chk
    CHECK (source_party_id <> target_party_id)
);

COMMENT ON TABLE tenants.party_merge_log IS
  'Log append-only de fusiones de identidad; conserva source, target, fecha y actor.';
COMMENT ON COLUMN tenants.party_merge_log.merged_by IS
  'Valor de app.current_user_id; sin FK para preservar el historial.';

CREATE INDEX party_merge_log_tenant_merged_at_idx
  ON tenants.party_merge_log (tenant_id, merged_at);
CREATE INDEX party_merge_log_source_idx
  ON tenants.party_merge_log (tenant_id, source_party_id);
CREATE INDEX party_merge_log_target_idx
  ON tenants.party_merge_log (tenant_id, target_party_id);

ALTER TABLE tenants.party_merge_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_merge_log FORCE ROW LEVEL SECURITY;

CREATE POLICY party_merge_log_tenant_isolation
  ON tenants.party_merge_log
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY party_merge_log_tenant_isolation ON tenants.party_merge_log IS
  'Aislamiento fail-closed por app.current_tenant_id.';

-- ----------------------------------------------------------------------------
-- Resolución del Golden Record
-- La recursión es una red de seguridad para cadenas previas a la compresión.
-- Con path compression, la profundidad normal es 1.
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION tenants.resolve_golden_party(p_party_id uuid)
RETURNS uuid
LANGUAGE sql
STABLE
SET search_path = pg_catalog, tenants
AS $$
  WITH RECURSIVE chain AS (
    SELECT p.id, p.canonical_party_id, 0 AS depth
      FROM tenants.parties p
     WHERE p.id = p_party_id

    UNION ALL

    SELECT p.id, p.canonical_party_id, c.depth + 1
      FROM tenants.parties p
      JOIN chain c ON p.id = c.canonical_party_id
     WHERE c.depth < 100
  )
  SELECT id
    FROM chain
   WHERE canonical_party_id IS NULL
   ORDER BY depth DESC
   LIMIT 1;
$$;

COMMENT ON FUNCTION tenants.resolve_golden_party(uuid) IS
  'Devuelve el Golden Record final siguiendo canonical_party_id; NULL si no existe o hay un ciclo.';
