-- ============================================================================
-- 0019_client_application_types.sql
-- First-party clients y aplicaciones exclusivas por tenant.
--
--   * client_type: FIRST_PARTY | THIRD_PARTY
--   * application_scope: PLATFORM | TENANT_SPECIFIC
--   * tenant_id nullable para apps PLATFORM.
--   * Una app PLATFORM debe ser FIRST_PARTY y no tener tenant.
--   * Una app TENANT_SPECIFIC debe tener tenant.
-- ============================================================================

ALTER TABLE global.client_applications
  ADD COLUMN client_type text NOT NULL DEFAULT 'THIRD_PARTY',
  ADD COLUMN application_scope text NOT NULL DEFAULT 'TENANT_SPECIFIC';

ALTER TABLE global.client_applications
  ALTER COLUMN tenant_id DROP NOT NULL;

ALTER TABLE global.client_applications
  ADD CONSTRAINT client_applications_client_type_chk
    CHECK (client_type IN ('FIRST_PARTY', 'THIRD_PARTY')),
  ADD CONSTRAINT client_applications_application_scope_chk
    CHECK (application_scope IN ('PLATFORM', 'TENANT_SPECIFIC')),
  ADD CONSTRAINT client_applications_scope_tenant_chk
    CHECK (
      (application_scope = 'TENANT_SPECIFIC' AND tenant_id IS NOT NULL)
      OR (application_scope = 'PLATFORM' AND tenant_id IS NULL AND client_type = 'FIRST_PARTY')
    );

CREATE INDEX client_applications_platform_idx
  ON global.client_applications (application_scope, client_type)
  WHERE is_active AND deleted_at IS NULL;

COMMENT ON COLUMN global.client_applications.client_type IS
  'FIRST_PARTY para apps oficiales; THIRD_PARTY para integraciones de clientes.';
COMMENT ON COLUMN global.client_applications.application_scope IS
  'PLATFORM para apps globales sin tenant; TENANT_SPECIFIC para apps de un tenant.';
