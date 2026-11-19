-- ============================================================================
-- 0035_settings_permissions.sql
-- Permiso RBAC para consultar la configuración del tenant.
-- ============================================================================

INSERT INTO global.system_functions (code, name, description, function_scope)
VALUES
  ('settings:read', 'Leer configuración', 'Permite consultar la configuración y las aplicaciones cliente del tenant', 'TENANT')
ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  function_scope = EXCLUDED.function_scope,
  updated_at = now()
WHERE global.system_functions.deleted_at IS NULL;
