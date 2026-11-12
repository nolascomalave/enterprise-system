-- ============================================================================
-- 0017_client_applications.sql
-- Aplicaciones cliente OAuth 2.0 (Client Credentials / M2M).
--
--   * global.client_applications: registro por tenant.
--   * Solo se persiste client_secret_hash; el secreto plano se devuelve una vez.
--   * scopes text[] con los permisos concedidos.
--   * El endpoint de token necesita buscar por client_id sin contexto de tenant,
--     por eso la tabla vive en global y no lleva RLS.
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF to_regclass('global.subscriptions') IS NULL THEN
    v_missing := array_append(v_missing, 'global.subscriptions');
  END IF;
  IF to_regclass('global.client_applications') IS NOT NULL THEN
    RAISE EXCEPTION 'global.client_applications ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear client_applications; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

CREATE TABLE global.client_applications (
  id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id          uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  name               text        NOT NULL,
  client_id          text        NOT NULL UNIQUE,
  client_secret_hash text        NOT NULL,
  scopes             text[]      NOT NULL DEFAULT '{}',
  is_active          boolean     NOT NULL DEFAULT true,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,

  CONSTRAINT client_applications_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT client_applications_client_id_chk CHECK (btrim(client_id) <> ''),
  CONSTRAINT client_applications_secret_chk CHECK (btrim(client_secret_hash) <> ''),
  CONSTRAINT client_applications_scopes_chk CHECK (cardinality(scopes) > 0)
);

CREATE INDEX client_applications_tenant_idx
  ON global.client_applications (tenant_id)
  WHERE deleted_at IS NULL;
CREATE INDEX client_applications_active_idx
  ON global.client_applications (client_id)
  WHERE is_active AND deleted_at IS NULL;

COMMENT ON TABLE global.client_applications IS
  'Aplicaciones cliente OAuth2 M2M. Sin RLS: el token endpoint busca por client_id.';
COMMENT ON COLUMN global.client_applications.client_secret_hash IS
  'Hash bcrypt del secreto; el valor plano nunca se persiste.';
COMMENT ON COLUMN global.client_applications.scopes IS
  'Scopes concedidos, p. ej. parties:read, org:write, documents:write.';
