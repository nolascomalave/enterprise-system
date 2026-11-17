-- ============================================================================
-- 0027_relationship_type_check.sql
-- Restrict new relationship rows to the SUBSIDIARY_OF catalog value.
--
-- NOT VALID keeps existing historical rows insertable while enforcing the
-- catalog for every new row. Run VALIDATE CONSTRAINT after cleaning history.
-- ============================================================================

DO $$
BEGIN
  IF to_regclass('global.tenant_relationships') IS NULL THEN
    RAISE EXCEPTION 'global.tenant_relationships is required';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'tenant_relationships_type_catalog_chk'
       AND conrelid = 'global.tenant_relationships'::regclass
  ) THEN
    ALTER TABLE global.tenant_relationships
      ADD CONSTRAINT tenant_relationships_type_catalog_chk
      CHECK (relationship_type = 'SUBSIDIARY_OF') NOT VALID;
  END IF;
END
$$;
