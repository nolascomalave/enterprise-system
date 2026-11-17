-- ============================================================================
-- 0025_rbac_tenant_app_fk.sql
-- Prevent tenant_roles from referencing a client application owned by another
-- tenant. The original FK only referenced global.client_applications(id).
-- ============================================================================

DO $$
DECLARE
  v_cross_tenant bigint;
BEGIN
  IF to_regclass('tenants.tenant_roles') IS NULL THEN
    RAISE EXCEPTION 'tenants.tenant_roles is required';
  END IF;
  IF to_regclass('global.client_applications') IS NULL THEN
    RAISE EXCEPTION 'global.client_applications is required';
  END IF;

  SELECT count(*) INTO v_cross_tenant
    FROM tenants.tenant_roles tr
    JOIN global.client_applications ca ON ca.id = tr.client_application_id
   WHERE tr.client_application_id IS NOT NULL
     AND ca.tenant_id IS DISTINCT FROM tr.tenant_id;

  IF v_cross_tenant > 0 THEN
    RAISE EXCEPTION
      'tenant_roles has % cross-tenant client_application_id reference(s); fix them before applying this migration',
      v_cross_tenant;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'client_applications_id_tenant_uq'
       AND conrelid = 'global.client_applications'::regclass
  ) THEN
    ALTER TABLE global.client_applications
      ADD CONSTRAINT client_applications_id_tenant_uq UNIQUE (id, tenant_id);
  END IF;

  IF EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'tenant_roles_client_application_id_fkey'
       AND conrelid = 'tenants.tenant_roles'::regclass
  ) THEN
    ALTER TABLE tenants.tenant_roles
      DROP CONSTRAINT tenant_roles_client_application_id_fkey;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'tenant_roles_client_application_tenant_fk'
       AND conrelid = 'tenants.tenant_roles'::regclass
  ) THEN
    ALTER TABLE tenants.tenant_roles
      ADD CONSTRAINT tenant_roles_client_application_tenant_fk
      FOREIGN KEY (client_application_id, tenant_id)
      REFERENCES global.client_applications (id, tenant_id)
      ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
END
$$;
