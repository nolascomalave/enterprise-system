-- ============================================================================
-- 0022_tenant_relationships_governance_fixes.sql
-- Ajustes de gobernanza sobre entornos que ya aplicaron la versión inicial
-- de 0021.
--
--   * INSERT de tenant_relationships: solo la subsidiary.
--   * Elimina tenant_transfers.acting_tenant_id.
--   * Renombra tenant_relationship_invites.used_at -> consumed_at.
--   * Recrea índices y políticas afectadas.
--
-- Migración idempotente: puede ejecutarse tanto si 0021 era la versión
-- antigua como si ya estaba corregida.
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regclass('global.tenant_relationships') IS NULL THEN
    v_missing := array_append(v_missing, 'global.tenant_relationships');
  END IF;
  IF to_regclass('global.tenant_relationship_invites') IS NULL THEN
    v_missing := array_append(v_missing, 'global.tenant_relationship_invites');
  END IF;
  IF to_regclass('global.tenant_transfers') IS NULL THEN
    v_missing := array_append(v_missing, 'global.tenant_transfers');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden aplicar los ajustes de gobernanza; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- 1) used_at -> consumed_at
-- ----------------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'global'
       AND table_name = 'tenant_relationship_invites'
       AND column_name = 'used_at'
  ) THEN
    ALTER TABLE global.tenant_relationship_invites
      RENAME COLUMN used_at TO consumed_at;
  END IF;
END
$$;

DROP INDEX IF EXISTS global.tenant_relationship_invites_relationship_active_idx;
CREATE INDEX tenant_relationship_invites_relationship_active_idx
  ON global.tenant_relationship_invites (relationship_id)
  WHERE canceled_at IS NULL AND consumed_at IS NULL;

DROP INDEX IF EXISTS global.tenant_relationship_invites_relationship_cancelled_idx;
CREATE INDEX tenant_relationship_invites_relationship_cancelled_idx
  ON global.tenant_relationship_invites (relationship_id)
  WHERE canceled_at IS NOT NULL;

-- ----------------------------------------------------------------------------
-- 2) Eliminar acting_tenant_id
-- ----------------------------------------------------------------------------
ALTER TABLE global.tenant_transfers
  DROP COLUMN IF EXISTS acting_tenant_id;

-- ----------------------------------------------------------------------------
-- 3) Políticas RLS
-- ----------------------------------------------------------------------------
DROP POLICY IF EXISTS tenant_relationships_insert ON global.tenant_relationships;
CREATE POLICY tenant_relationships_insert
  ON global.tenant_relationships
  FOR INSERT
  WITH CHECK (
    tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

DROP POLICY IF EXISTS tenant_transfers_select ON global.tenant_transfers;
CREATE POLICY tenant_transfers_select
  ON global.tenant_transfers
  FOR SELECT
  USING (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    OR EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_transfers.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  );

DROP POLICY IF EXISTS tenant_transfers_insert ON global.tenant_transfers;
CREATE POLICY tenant_transfers_insert
  ON global.tenant_transfers
  FOR INSERT
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

DROP POLICY IF EXISTS tenant_transfers_update ON global.tenant_transfers;
CREATE POLICY tenant_transfers_update
  ON global.tenant_transfers
  FOR UPDATE
  USING (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );
