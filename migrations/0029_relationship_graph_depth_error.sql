-- ============================================================================
-- 0029_relationship_graph_depth_error.sql
-- Preserve the depth-20 safety bound but fail with a controlled error instead
-- of silently returning a false negative when the graph exceeds it.
-- SQLSTATE 54000 -> Go maps it to domain.ErrRelationshipGraphTooDeep.
-- ============================================================================

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
  v_current    uuid;
  v_cycle      boolean;
  v_max_depth  integer;
  v_has_deeper boolean;
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
  SELECT
    EXISTS (SELECT 1 FROM descendants WHERE tenant_id = p_holding_id),
    COALESCE(MAX(d.depth), 0),
    EXISTS (
      SELECT 1
        FROM global.tenant_relationship_edges e
        JOIN descendants d2 ON e.holding_id = d2.tenant_id
       WHERE e.status = 'APPROVED'
         AND d2.depth = 20
    )
  INTO v_cycle, v_max_depth, v_has_deeper
  FROM descendants d;

  IF v_has_deeper THEN
    RAISE EXCEPTION
      'relationship graph exceeds the supported depth of 20'
      USING ERRCODE = '54000';
  END IF;

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
