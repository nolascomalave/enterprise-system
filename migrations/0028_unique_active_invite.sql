-- ============================================================================
-- 0028_unique_active_invite.sql
-- Enforce at most one active invite token per relationship.
-- ============================================================================

DO $$
DECLARE
  v_duplicates bigint;
BEGIN
  IF to_regclass('global.tenant_relationship_invites') IS NULL THEN
    RAISE EXCEPTION 'global.tenant_relationship_invites is required';
  END IF;

  SELECT count(*) INTO v_duplicates
    FROM (
      SELECT relationship_id
        FROM global.tenant_relationship_invites
       WHERE canceled_at IS NULL
         AND consumed_at IS NULL
       GROUP BY relationship_id
      HAVING count(*) > 1
    ) duplicates;

  IF v_duplicates > 0 THEN
    RAISE EXCEPTION
      'there are % relationship(s) with multiple active invites; cancel duplicates before applying this migration',
      v_duplicates
      USING HINT = 'UPDATE global.tenant_relationship_invites SET canceled_at = now(), canceled_by_tenant_id = (SELECT tenant_id_holding FROM global.tenant_relationships WHERE id = relationship_id) WHERE relationship_id IN (...) AND canceled_at IS NULL AND consumed_at IS NULL;';
  END IF;

  DROP INDEX IF EXISTS global.tenant_relationship_invites_relationship_active_idx;
  CREATE UNIQUE INDEX IF NOT EXISTS tenant_relationship_invites_relationship_active_uq
    ON global.tenant_relationship_invites (relationship_id)
    WHERE canceled_at IS NULL AND consumed_at IS NULL;
END
$$;
