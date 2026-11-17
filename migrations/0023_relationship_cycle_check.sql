-- ============================================================================
-- 0023_relationship_cycle_check.sql
-- Full-graph cycle detection for global.tenant_relationships.
--
-- Why an edge projection:
--   global.tenant_relationships has FORCE ROW LEVEL SECURITY. A SECURITY
--   DEFINER function owned by the application role is still subject to FORCE
--   RLS unless the owner has BYPASSRLS. The edge projection is an internal,
--   minimal (relationship_id, holding, subsidiary, status) table without RLS,
--   synchronized by a trigger. The public API only reaches it through the
--   guarded SECURITY DEFINER function.
--
-- The function validates that the current tenant is a participant before it
-- is allowed to inspect the graph.
-- ============================================================================

DO $$
BEGIN
  IF to_regclass('global.tenant_relationships') IS NULL THEN
    RAISE EXCEPTION 'global.tenant_relationships is required';
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS global.tenant_relationship_edges (
  relationship_id uuid        PRIMARY KEY,
  holding_id      uuid        NOT NULL,
  subsidiary_id   uuid        NOT NULL,
  status          text        NOT NULL,
  updated_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_relationship_edges_status_chk
    CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CANCELED'))
);

CREATE INDEX IF NOT EXISTS tenant_relationship_edges_holding_status_idx
  ON global.tenant_relationship_edges (holding_id, status);
CREATE INDEX IF NOT EXISTS tenant_relationship_edges_subsidiary_status_idx
  ON global.tenant_relationship_edges (subsidiary_id, status);

COMMENT ON TABLE global.tenant_relationship_edges IS
  'Internal no-RLS projection of global.tenant_relationships used only by global.fn_check_relationship_cycle. Do not grant direct access.';

INSERT INTO global.tenant_relationship_edges (relationship_id, holding_id, subsidiary_id, status)
SELECT id, tenant_id_holding, tenant_id_subsidiary, status
  FROM global.tenant_relationships
ON CONFLICT (relationship_id) DO UPDATE
  SET holding_id = EXCLUDED.holding_id,
      subsidiary_id = EXCLUDED.subsidiary_id,
      status = EXCLUDED.status,
      updated_at = now();

CREATE OR REPLACE FUNCTION global.fn_sync_tenant_relationship_edge()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, global
AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    DELETE FROM global.tenant_relationship_edges WHERE relationship_id = OLD.id;
    RETURN OLD;
  END IF;

  INSERT INTO global.tenant_relationship_edges
    (relationship_id, holding_id, subsidiary_id, status, updated_at)
  VALUES
    (NEW.id, NEW.tenant_id_holding, NEW.tenant_id_subsidiary, NEW.status, now())
  ON CONFLICT (relationship_id) DO UPDATE
    SET holding_id = EXCLUDED.holding_id,
        subsidiary_id = EXCLUDED.subsidiary_id,
        status = EXCLUDED.status,
        updated_at = now();
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS tenant_relationships_edge_sync_trg ON global.tenant_relationships;
CREATE TRIGGER tenant_relationships_edge_sync_trg
  AFTER INSERT OR UPDATE OR DELETE ON global.tenant_relationships
  FOR EACH ROW EXECUTE FUNCTION global.fn_sync_tenant_relationship_edge();

CREATE OR REPLACE FUNCTION global.fn_check_relationship_cycle(
  p_holding_id uuid,
  p_subsidiary_id uuid
)
RETURNS boolean
LANGUAGE plpgsql
SECURITY DEFINER
STABLE
SET search_path = pg_catalog, global
AS $$
DECLARE
  v_current uuid;
  v_cycle   boolean;
BEGIN
  IF p_holding_id IS NULL OR p_subsidiary_id IS NULL THEN
    RAISE EXCEPTION 'holding_id and subsidiary_id are required' USING ERRCODE = '22023';
  END IF;

  v_current := NULLIF(current_setting('app.current_tenant_id', true), '')::uuid;
  IF v_current IS NULL THEN
    RAISE EXCEPTION 'missing tenant context' USING ERRCODE = '42501';
  END IF;
  IF v_current <> p_holding_id AND v_current <> p_subsidiary_id THEN
    RAISE EXCEPTION 'caller is not a relationship participant' USING ERRCODE = '42501';
  END IF;

  WITH RECURSIVE descendants AS (
    SELECT e.subsidiary_id AS tenant_id, 1 AS depth
      FROM global.tenant_relationship_edges e
     WHERE e.holding_id = p_subsidiary_id
       AND e.status = 'APPROVED'
    UNION ALL
    SELECT e.subsidiary_id, d.depth + 1
      FROM global.tenant_relationship_edges e
      JOIN descendants d ON e.holding_id = d.tenant_id
     WHERE e.status = 'APPROVED'
       AND d.depth < 20
  )
  SELECT EXISTS (
    SELECT 1 FROM descendants WHERE tenant_id = p_holding_id
  ) INTO v_cycle;

  RETURN v_cycle;
END;
$$;

REVOKE ALL ON FUNCTION global.fn_check_relationship_cycle(uuid, uuid) FROM PUBLIC;
DO $$
BEGIN
  EXECUTE format(
    'GRANT EXECUTE ON FUNCTION global.fn_check_relationship_cycle(uuid, uuid) TO %I',
    current_user
  );
END
$$;

COMMENT ON FUNCTION global.fn_check_relationship_cycle(uuid, uuid) IS
  'Returns true if adding holding -> subsidiary would close an APPROVED cycle. Caller must be the holding or subsidiary.';
