-- ============================================================================
-- seeds/system_functions.sql
-- Funciones de sistema base para el módulo RBAC.
-- Idempotente: puede reejecutarse sin duplicar filas.
-- ============================================================================

INSERT INTO global.system_functions (code, name, description, function_scope)
VALUES
  ('party:read',        'Consultar parties',        'Permite listar y ver parties',                  'TENANT'),
  ('party:write',       'Gestionar parties',        'Permite crear y modificar parties',             'TENANT'),
  ('employee:read',     'Consultar empleados',      'Permite listar y ver empleados',                'TENANT'),
  ('org:read',          'Consultar organización',   'Permite ver estructura organizacional',         'TENANT'),
  ('org:write',         'Gestionar organización',   'Permite modificar estructura organizacional',   'TENANT'),
  ('identity:write',    'Gestionar identidad',      'Permite crear documentos y nacionalidades',     'TENANT'),
  ('documents:write',   'Adjuntar documentos',      'Permite subir documentos adjuntos',             'TENANT'),
  ('locations:read',    'Consultar ubicaciones',    'Permite consultar catálogo geográfico',         'TENANT'),
  ('admin:write',       'Administración tenant',    'Permite operaciones administrativas del tenant','TENANT'),
  ('platform:admin',    'Administración plataforma','Permite operaciones globales de plataforma',    'PLATFORM'),
  ('reports:global',    'Reportes globales',        'Permite acceder a reportes de plataforma',      'PLATFORM'),
  ('rbac:admin',        'Administrar RBAC',         'Permite gestionar roles, menús y permisos',     'TENANT'),
  ('rbac:read',         'Leer RBAC',                'Permite consultar roles y permisos propios',    'TENANT'),
  ('account:write',     'Gestionar cuenta',         'Permite modificar la propia cuenta de usuario', 'TENANT')
ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET
  name = EXCLUDED.name,
  description = EXCLUDED.description,
  function_scope = EXCLUDED.function_scope,
  updated_at = now()
WHERE global.system_functions.deleted_at IS NULL;
