-- ============================================================================
-- 0034_dashboard_permissions.sql
-- Permiso RBAC para el resumen del dashboard.
-- ============================================================================

INSERT INTO global.system_functions (code, name, description, function_scope)
VALUES
  ('dashboard:read', 'Leer dashboard', 'Permite consultar el resumen y los indicadores del dashboard', 'TENANT')
ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  function_scope = EXCLUDED.function_scope,
  updated_at = now()
WHERE global.system_functions.deleted_at IS NULL;
