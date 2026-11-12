-- ============================================================================
-- 0009_role_types_seed.sql
-- Catálogo global mínimo para el onboarding.
-- ============================================================================

DO $$
BEGIN
  IF to_regclass('global.role_types') IS NULL THEN
    RAISE EXCEPTION 'Falta global.role_types; aplica primero el esquema Ent/Atlas';
  END IF;
END
$$;

INSERT INTO global.role_types (id, code, name, description, is_system, created_at, updated_at)
VALUES
  (uuidv7(), 'TENANT', 'Tenant', 'Organization that owns the tenant', true, now(), now()),
  (uuidv7(), 'ADMIN',  'Tenant Administrator', 'First administrator of the tenant', true, now(), now())
ON CONFLICT DO NOTHING;
