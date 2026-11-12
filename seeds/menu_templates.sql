-- ============================================================================
-- seeds/menu_templates.sql
-- Plantillas de menú base para aplicaciones cliente.
-- Idempotente: puede reejecutarse sin duplicar filas.
-- ============================================================================

INSERT INTO global.menu_templates (code, name, client_type, application_scope)
VALUES
  ('ADMIN_PERSONNEL', 'Administración de Personal', 'FIRST_PARTY', 'TENANT_SPECIFIC'),
  ('PLATFORM_ADMIN',  'Administración de Plataforma', 'FIRST_PARTY', 'PLATFORM'),
  ('THIRD_PARTY_INTEGRATION', 'Integración de Terceros', 'THIRD_PARTY', 'TENANT_SPECIFIC')
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  client_type = EXCLUDED.client_type,
  application_scope = EXCLUDED.application_scope,
  updated_at = now()
WHERE global.menu_templates.deleted_at IS NULL;
