-- ============================================================================
-- 0030_mfa_and_security_codes.sql
-- Preferencias de MFA y canal de recepción de códigos de seguridad.
--
--   * global.contact_mechanisms.can_receive_security_code
--   * tenants.user_credentials.totp_enabled -> mfa_enabled
--   * tenants.user_credentials.mfa_method: NONE | TOTP | EMAIL_OTP
-- ============================================================================

DO $$
BEGIN
  IF to_regclass('global.contact_mechanisms') IS NULL THEN
    RAISE EXCEPTION 'global.contact_mechanisms is required';
  END IF;
  IF to_regclass('tenants.user_credentials') IS NULL THEN
    RAISE EXCEPTION 'tenants.user_credentials is required';
  END IF;
END
$$;

ALTER TABLE global.contact_mechanisms
  ADD COLUMN IF NOT EXISTS can_receive_security_code boolean NOT NULL DEFAULT false;

UPDATE global.contact_mechanisms
   SET can_receive_security_code = CASE
     WHEN type IN ('EMAIL', 'PHONE', 'MOBILE') THEN true
     ELSE false
   END;

COMMENT ON COLUMN global.contact_mechanisms.can_receive_security_code IS
  'Indica si el mecanismo puede recibir códigos de seguridad (email, phone, mobile).';

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'tenants'
       AND table_name = 'user_credentials'
       AND column_name = 'totp_enabled'
  ) THEN
    ALTER TABLE tenants.user_credentials
      RENAME COLUMN totp_enabled TO mfa_enabled;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'tenants'
       AND table_name = 'user_credentials'
       AND column_name = 'mfa_method'
  ) THEN
    ALTER TABLE tenants.user_credentials
      ADD COLUMN mfa_method text NOT NULL DEFAULT 'NONE';
  END IF;

  UPDATE tenants.user_credentials
     SET mfa_method = CASE WHEN mfa_enabled THEN 'TOTP' ELSE 'NONE' END
   WHERE mfa_method = 'NONE' AND mfa_enabled;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'user_credentials_mfa_method_chk'
       AND conrelid = 'tenants.user_credentials'::regclass
  ) THEN
    ALTER TABLE tenants.user_credentials
      ADD CONSTRAINT user_credentials_mfa_method_chk
      CHECK (mfa_method IN ('NONE', 'TOTP', 'EMAIL_OTP'));
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'user_credentials_mfa_consistency_chk'
       AND conrelid = 'tenants.user_credentials'::regclass
  ) THEN
    ALTER TABLE tenants.user_credentials
      ADD CONSTRAINT user_credentials_mfa_consistency_chk
      CHECK (
        (mfa_enabled = false AND mfa_method = 'NONE')
        OR (mfa_enabled = true AND mfa_method IN ('TOTP', 'EMAIL_OTP'))
      );
  END IF;
END
$$;
