-- ============================================================================
-- 0021_tenant_relationships_and_transfers.sql
-- Relaciones holding/subsidiary, invitaciones y transferencias entre tenants.
--
-- Incluye:
--   * global.tenant_relationships con RLS (holding o subsidiary).
--   * Unicidad parcial de una sola relación PENDING por
--     (holding, subsidiary, relationship_type).
--   * global.tenant_relationship_invites sin tenant_id; RLS y auditoría
--     heredan el tenant desde tenant_relationships.
--   * global.tenant_transfers con RLS explícito.
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
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;

  IF to_regclass('global.tenant_relationships') IS NOT NULL THEN
    RAISE EXCEPTION 'global.tenant_relationships ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('global.tenant_relationship_invites') IS NOT NULL THEN
    RAISE EXCEPTION 'global.tenant_relationship_invites ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('global.tenant_transfers') IS NOT NULL THEN
    RAISE EXCEPTION 'global.tenant_transfers ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el módulo de relaciones entre tenants; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

-- ============================================================================
-- 1) RELACIONES HOLDING / SUBSIDIARY
-- ============================================================================

CREATE TABLE global.tenant_relationships (
  id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id_holding    uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  tenant_id_subsidiary uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  relationship_type    text        NOT NULL,
  status               text        NOT NULL DEFAULT 'PENDING',
  requested_at         timestamptz NOT NULL DEFAULT now(),
  approved_at          timestamptz,
  rejected_at          timestamptz,
  canceled_at          timestamptz,
  ends_at              timestamptz,
  rejection_reason     text,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_relationships_type_chk
    CHECK (btrim(relationship_type) <> ''),
  CONSTRAINT tenant_relationships_distinct_tenants_chk
    CHECK (tenant_id_holding <> tenant_id_subsidiary),
  CONSTRAINT tenant_relationships_status_chk
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELED'))
);

-- Exactamente una relación PENDING por (holding, subsidiary, type).
CREATE UNIQUE INDEX tenant_relationships_pending_uq
  ON global.tenant_relationships (tenant_id_holding, tenant_id_subsidiary, relationship_type)
  WHERE status = 'PENDING';

CREATE INDEX tenant_relationships_holding_idx
  ON global.tenant_relationships (tenant_id_holding, status);
CREATE INDEX tenant_relationships_subsidiary_idx
  ON global.tenant_relationships (tenant_id_subsidiary, status);
CREATE INDEX tenant_relationships_status_idx
  ON global.tenant_relationships (status);

COMMENT ON TABLE global.tenant_relationships IS
  'Relaciones holding/subsidiary. Una sola PENDING por holding, subsidiary y tipo.';
COMMENT ON COLUMN global.tenant_relationships.relationship_type IS
  'Tipo de relación (p. ej. OWNERSHIP, MANAGEMENT).';

ALTER TABLE global.tenant_relationships ENABLE ROW LEVEL SECURITY;
ALTER TABLE global.tenant_relationships FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_relationships_select
  ON global.tenant_relationships
  FOR SELECT
  USING (
    tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    OR tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

CREATE POLICY tenant_relationships_insert
  ON global.tenant_relationships
  FOR INSERT
  WITH CHECK (
    tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

CREATE POLICY tenant_relationships_update
  ON global.tenant_relationships
  FOR UPDATE
  USING (
    tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    OR tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    OR tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

DROP TRIGGER IF EXISTS tenant_relationships_audit_trg ON global.tenant_relationships;
CREATE TRIGGER tenant_relationships_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON global.tenant_relationships
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ============================================================================
-- 2) INVITACIONES
-- ============================================================================

CREATE TABLE global.tenant_relationship_invites (
  id                       uuid        PRIMARY KEY DEFAULT uuidv7(),
  relationship_id          uuid        NOT NULL REFERENCES global.tenant_relationships (id) ON DELETE RESTRICT,
  token_hash               bytea       NOT NULL,
  expires_at               timestamptz NOT NULL,
  consumed_at              timestamptz,
  canceled_at              timestamptz,
  canceled_by_tenant_id    uuid        REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_relationship_invites_token_uq UNIQUE (token_hash),
  CONSTRAINT tenant_relationship_invites_expiry_chk
    CHECK (expires_at > created_at),
  CONSTRAINT tenant_relationship_invites_cancel_pair_chk
    CHECK (
      (canceled_at IS NULL AND canceled_by_tenant_id IS NULL)
      OR (canceled_at IS NOT NULL AND canceled_by_tenant_id IS NOT NULL)
    )
);

CREATE INDEX tenant_relationship_invites_relationship_active_idx
  ON global.tenant_relationship_invites (relationship_id)
  WHERE canceled_at IS NULL AND consumed_at IS NULL;
CREATE INDEX tenant_relationship_invites_relationship_cancelled_idx
  ON global.tenant_relationship_invites (relationship_id)
  WHERE canceled_at IS NOT NULL;

COMMENT ON TABLE global.tenant_relationship_invites IS
  'Invitaciones por relación. No lleva tenant_id: el tenant se hereda de tenant_relationships.';
COMMENT ON COLUMN global.tenant_relationship_invites.token_hash IS
  'SHA-256 del código de invitación; nunca se guarda el código en claro.';

ALTER TABLE global.tenant_relationship_invites ENABLE ROW LEVEL SECURITY;
ALTER TABLE global.tenant_relationship_invites FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_relationship_invites_select
  ON global.tenant_relationship_invites
  FOR SELECT
  USING (
    EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_relationship_invites.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  );

CREATE POLICY tenant_relationship_invites_insert
  ON global.tenant_relationship_invites
  FOR INSERT
  WITH CHECK (
    EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_relationship_invites.relationship_id
         AND r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    )
  );

CREATE POLICY tenant_relationship_invites_update
  ON global.tenant_relationship_invites
  FOR UPDATE
  USING (
    EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_relationship_invites.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  )
  WITH CHECK (
    EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_relationship_invites.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  );

-- Auditoría específica: hereda tenant_id_holding desde tenant_relationships.
CREATE OR REPLACE FUNCTION audit.fn_log_tenant_relationship_invite()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, audit, global
AS $$
DECLARE
  v_old         jsonb;
  v_new         jsonb;
  v_changed     jsonb := '{}'::jsonb;
  v_operation   text;
  v_changed_by  uuid;
  v_tenant_id   uuid;
  v_session_id  uuid;
  v_record_id   text;
  v_key         text;
  v_relationship_id uuid;
BEGIN
  v_changed_by := NULLIF(current_setting('app.current_user_id', true), '')::uuid;
  v_session_id := NULLIF(current_setting('app.current_session_id', true), '')::uuid;

  IF TG_OP = 'INSERT' THEN
    v_new := to_jsonb(NEW);
    v_operation := 'INSERT';
    v_changed := v_new;
  ELSIF TG_OP = 'DELETE' THEN
    v_old := to_jsonb(OLD);
    v_operation := 'DELETE';
    v_changed := v_old;
  ELSE
    v_old := to_jsonb(OLD);
    v_new := to_jsonb(NEW);
    v_operation := 'UPDATE';
    SELECT COALESCE(jsonb_object_agg(COALESCE(n.key, o.key), n.value), '{}'::jsonb)
      INTO v_changed
      FROM jsonb_each(v_new) AS n
      FULL OUTER JOIN jsonb_each(v_old) AS o ON o.key = n.key
     WHERE n.value IS DISTINCT FROM o.value;
  END IF;

  v_record_id := COALESCE(v_new, v_old)->>'id';
  v_relationship_id := (COALESCE(v_new, v_old)->>'relationship_id')::uuid;

  -- El histórico hereda el tenant holding de la relación.
  IF v_relationship_id IS NOT NULL THEN
    SELECT r.tenant_id_holding INTO v_tenant_id
      FROM global.tenant_relationships r
     WHERE r.id = v_relationship_id;
  END IF;

  IF v_tenant_id IS NULL THEN
    v_tenant_id := NULLIF(current_setting('app.current_tenant_id', true), '')::uuid;
  END IF;

  -- Redacción de token_hash: nunca se audita el hash completo.
  IF TG_NARGS > 0 THEN
    FOREACH v_key IN ARRAY TG_ARGV LOOP
      IF v_old IS NOT NULL THEN
        v_old := v_old - v_key;
      END IF;
      IF v_new IS NOT NULL THEN
        v_new := v_new - v_key;
      END IF;
      v_changed := v_changed - v_key;
    END LOOP;
  END IF;

  INSERT INTO audit.change_log (
    table_name, record_id, operation, old_data, new_data,
    changed_fields, changed_by, tenant_id, session_id
  ) VALUES (
    TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME,
    v_record_id,
    v_operation,
    v_old,
    v_new,
    v_changed,
    v_changed_by,
    v_tenant_id,
    v_session_id
  );

  RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS tenant_relationship_invites_audit_trg
  ON global.tenant_relationship_invites;
CREATE TRIGGER tenant_relationship_invites_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON global.tenant_relationship_invites
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_tenant_relationship_invite('token_hash');

-- ============================================================================
-- 3) TRANSFERENCIAS ENTRE TENANTS
-- ============================================================================

CREATE TABLE global.tenant_transfers (
  id                       uuid        PRIMARY KEY DEFAULT uuidv7(),
  relationship_id          uuid        NOT NULL REFERENCES global.tenant_relationships (id) ON DELETE RESTRICT,
  requested_by_tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  status                   text        NOT NULL DEFAULT 'TRANSFER_REQUESTED',
  requested_at             timestamptz NOT NULL DEFAULT now(),
  started_at               timestamptz,
  completed_at             timestamptz,
  canceled_at              timestamptz,
  canceled_by_tenant_id    uuid        REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  transfer_reason          text,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_transfers_status_chk
    CHECK (status IN (
      'TRANSFER_REQUESTED',
      'TRANSFER_IN_PROGRESS',
      'TRANSFER_COMPLETED',
      'TRANSFER_FAILED',
      'TRANSFER_CANCELED'
    )),
  CONSTRAINT tenant_transfers_cancel_pair_chk
    CHECK (
      (canceled_at IS NULL AND canceled_by_tenant_id IS NULL)
      OR (canceled_at IS NOT NULL AND canceled_by_tenant_id IS NOT NULL)
    )
);

CREATE INDEX tenant_transfers_relationship_id_idx
  ON global.tenant_transfers (relationship_id);
CREATE INDEX tenant_transfers_requested_by_tenant_id_idx
  ON global.tenant_transfers (requested_by_tenant_id);
CREATE INDEX tenant_transfers_status_idx
  ON global.tenant_transfers (status);

COMMENT ON TABLE global.tenant_transfers IS
  'Transferencias de subsidiarias entre holdings; RLS explícito por relación y requested_by_tenant_id.';

ALTER TABLE global.tenant_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE global.tenant_transfers FORCE ROW LEVEL SECURITY;

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

CREATE POLICY tenant_transfers_insert
  ON global.tenant_transfers
  FOR INSERT
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

CREATE POLICY tenant_transfers_update
  ON global.tenant_transfers
  FOR UPDATE
  USING (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

DROP TRIGGER IF EXISTS tenant_transfers_audit_trg ON global.tenant_transfers;
CREATE TRIGGER tenant_transfers_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON global.tenant_transfers
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();
