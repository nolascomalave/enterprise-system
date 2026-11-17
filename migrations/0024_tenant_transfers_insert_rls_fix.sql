-- ============================================================================
-- 0024_tenant_transfers_insert_rls_fix.sql
-- The tenant_transfers INSERT/UPDATE policies checked only
-- requested_by_tenant_id = current_tenant_id. An outsider could create or
-- update a transfer row for a relationship it does not participate in.
--
-- Require the current tenant to be the requested_by tenant AND the holding or
-- subsidiary of the referenced relationship.
-- ============================================================================

DROP POLICY IF EXISTS tenant_transfers_insert ON global.tenant_transfers;
CREATE POLICY tenant_transfers_insert
  ON global.tenant_transfers
  FOR INSERT
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    AND EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_transfers.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  );

DROP POLICY IF EXISTS tenant_transfers_update ON global.tenant_transfers;
CREATE POLICY tenant_transfers_update
  ON global.tenant_transfers
  FOR UPDATE
  USING (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    AND EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_transfers.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  )
  WITH CHECK (
    requested_by_tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    AND EXISTS (
      SELECT 1
        FROM global.tenant_relationships r
       WHERE r.id = tenant_transfers.relationship_id
         AND (
           r.tenant_id_holding = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
           OR r.tenant_id_subsidiary = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
         )
    )
  );
