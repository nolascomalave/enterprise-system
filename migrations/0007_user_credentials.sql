-- ============================================================================
-- 0007_user_credentials.sql
-- Credenciales de autenticación por Party.
--
--   * password_hash: bcrypt (nunca contraseña en claro).
--   * totp_secret / totp_enabled: segundo factor TOTP.
--   * backup_codes_hash: hashes bcrypt de códigos de respaldo (jsonb array).
--   * RLS + auditoría.
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
  IF to_regclass('tenants.parties') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.parties');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_collation
     WHERE collname = 'unaccent_case_insensitive'
       AND collprovider = 'i'
       AND NOT collisdeterministic
  ) THEN
    v_missing := array_append(v_missing, 'collation unaccent_case_insensitive');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear credenciales; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica la collation ICU.';
  END IF;
END
$$;

CREATE TABLE tenants.user_credentials (
  id                uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id         uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id          uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  email             text        COLLATE "unaccent_case_insensitive" NOT NULL,
  password_hash     text        NOT NULL,
  totp_secret       text,
  totp_enabled      boolean     NOT NULL DEFAULT false,
  backup_codes_hash jsonb       NOT NULL DEFAULT '[]'::jsonb,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,

  CONSTRAINT user_credentials_email_chk CHECK (btrim(email) <> ''),
  CONSTRAINT user_credentials_password_chk CHECK (btrim(password_hash) <> ''),
  CONSTRAINT user_credentials_backup_codes_chk
    CHECK (jsonb_typeof(backup_codes_hash) = 'array')
);

CREATE UNIQUE INDEX user_credentials_tenant_email_uq
  ON tenants.user_credentials (tenant_id, email)
  WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX user_credentials_tenant_party_uq
  ON tenants.user_credentials (tenant_id, party_id)
  WHERE deleted_at IS NULL;
CREATE INDEX user_credentials_tenant_party_idx
  ON tenants.user_credentials (tenant_id, party_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.user_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.user_credentials FORCE ROW LEVEL SECURITY;
CREATE POLICY user_credentials_tenant_isolation ON tenants.user_credentials
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS user_credentials_audit_trg ON tenants.user_credentials;
CREATE TRIGGER user_credentials_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.user_credentials
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change(
    'password_hash', 'totp_secret', 'backup_codes_hash'
  );
