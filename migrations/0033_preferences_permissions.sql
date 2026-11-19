-- ============================================================================
-- 0033_preferences_permissions.sql
-- Permisos RBAC para las preferencias de usuario por aplicación cliente.
-- ============================================================================

INSERT INTO global.system_functions (code, name, description, function_scope)
VALUES
  ('preferences:read',  'Leer preferencias de usuario',   'Permite consultar las preferencias de UI del propio usuario', 'TENANT'),
  ('preferences:write', 'Gestionar preferencias de usuario', 'Permite actualizar las preferencias de UI del propio usuario', 'TENANT')
ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  function_scope = EXCLUDED.function_scope,
  updated_at = now()
WHERE global.system_functions.deleted_at IS NULL;
