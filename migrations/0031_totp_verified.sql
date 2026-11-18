-- ============================================================================
-- 0031_totp_verified.sql
-- Distingue un secreto TOTP pendiente de enrolamiento de uno ya verificado.
-- Sin este flag, PUT /auth/mfa-preferences podría habilitar TOTP saltándose
-- POST /auth/totp/verify.
-- ============================================================================

DO $$
BEGIN
  IF to_regclass('tenants.user_credentials') IS NULL THEN
    RAISE EXCEPTION 'tenants.user_credentials is required';
  END IF;
END
$$;

ALTER TABLE tenants.user_credentials
  ADD COLUMN IF NOT EXISTS totp_verified boolean NOT NULL DEFAULT false;

-- Rows that were already using TOTP but have no usable secret cannot complete
-- login; disable them instead of declaring them verified.
UPDATE tenants.user_credentials
   SET mfa_enabled = false,
       mfa_method = 'NONE',
       totp_verified = false
 WHERE mfa_enabled
   AND mfa_method = 'TOTP'
   AND (totp_secret IS NULL OR btrim(totp_secret) = '');

-- Preserve already-working legacy TOTP enrolments.
UPDATE tenants.user_credentials
   SET totp_verified = true
 WHERE mfa_enabled
   AND mfa_method = 'TOTP';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'user_credentials_totp_verified_chk'
       AND conrelid = 'tenants.user_credentials'::regclass
  ) THEN
    ALTER TABLE tenants.user_credentials
      ADD CONSTRAINT user_credentials_totp_verified_chk
      CHECK (NOT (mfa_enabled AND mfa_method = 'TOTP' AND NOT totp_verified));
  END IF;
END
$$;

COMMENT ON COLUMN tenants.user_credentials.totp_verified IS
  'Indica que el usuario validó un código contra totp_secret antes de activar TOTP.';
