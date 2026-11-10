-- ============================================================================
-- 0008_password_reset_tokens.sql
-- Tokens de un solo uso para recuperación de contraseña.
--
-- Solo se persiste token_hash (SHA-256). El token en claro viaja únicamente
-- al usuario por correo.
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

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear password_reset_tokens; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

CREATE TABLE tenants.password_reset_tokens (
  id         uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id  uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id   uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  token_hash bytea       NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT password_reset_tokens_hash_chk CHECK (octet_length(token_hash) = 32),
  CONSTRAINT password_reset_tokens_expiry_chk CHECK (expires_at > created_at),
  CONSTRAINT password_reset_tokens_used_chk
    CHECK (used_at IS NULL OR used_at >= created_at)
);

CREATE INDEX password_reset_tokens_tenant_party_idx
  ON tenants.password_reset_tokens (tenant_id, party_id, expires_at);
CREATE INDEX password_reset_tokens_tenant_expiry_idx
  ON tenants.password_reset_tokens (tenant_id, expires_at);

ALTER TABLE tenants.password_reset_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.password_reset_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY password_reset_tokens_tenant_isolation
  ON tenants.password_reset_tokens
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

-- El hash del token no se escribe en la auditoría.
DROP TRIGGER IF EXISTS password_reset_tokens_audit_trg ON tenants.password_reset_tokens;
CREATE TRIGGER password_reset_tokens_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.password_reset_tokens
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change('token_hash');
