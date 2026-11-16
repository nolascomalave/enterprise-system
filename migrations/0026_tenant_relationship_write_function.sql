-- ============================================================================
-- 0026_tenant_relationship_write_function.sql
-- Dedicated system function for tenant relationship management.
-- ============================================================================

INSERT INTO global.system_functions (code, name, description, function_scope)
SELECT
  'tenant_relationship:write',
  'Gestionar relaciones entre tenants',
  'Permite iniciar, emitir, consumir, rechazar, cancelar y revocar relaciones holding/subsidiary',
  'TENANT'
WHERE NOT EXISTS (
  SELECT 1 FROM global.system_functions
   WHERE code = 'tenant_relationship:write'
     AND deleted_at IS NULL
);
