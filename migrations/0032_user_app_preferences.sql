-- ============================================================================
-- 0032_user_app_preferences.sql
-- Preferencias de UI por usuario y aplicación cliente.
--
-- El JWT expone el party_id como app.current_user_id; la política RLS lo
-- resuelve contra user_credentials.party_id para aislar por credencial. No se
-- compara directamente user_credentials_id con app.current_user_id porque el
-- token no transporta el id de credenciales.
-- ============================================================================

CREATE TABLE IF NOT EXISTS tenants.user_app_preferences (
  id                    uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id             uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  user_credentials_id   uuid        NOT NULL,
  client_application_id uuid        NOT NULL,
  preferences           jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  deleted_at            timestamptz,

  CONSTRAINT user_app_preferences_preferences_chk
    CHECK (jsonb_typeof(preferences) = 'object'),

  -- FKs compuestas: impiden asociar preferencias a credenciales o apps de otro tenant.
  CONSTRAINT user_app_preferences_user_fk
    FOREIGN KEY (user_credentials_id, tenant_id)
    REFERENCES tenants.user_credentials (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,

  CONSTRAINT user_app_preferences_app_fk
    FOREIGN KEY (client_application_id, tenant_id)
    REFERENCES global.client_applications (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS user_app_pref_uq
  ON tenants.user_app_preferences (tenant_id, user_credentials_id, client_application_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_app_prefs_tenant_user
  ON tenants.user_app_preferences (tenant_id, user_credentials_id);

CREATE INDEX IF NOT EXISTS idx_user_app_prefs_gin
  ON tenants.user_app_preferences USING GIN (preferences);

ALTER TABLE tenants.user_app_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.user_app_preferences FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS user_app_preferences_isolation ON tenants.user_app_preferences;
CREATE POLICY user_app_preferences_isolation
  ON tenants.user_app_preferences
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    AND EXISTS (
      SELECT 1
        FROM tenants.user_credentials uc
       WHERE uc.id = user_app_preferences.user_credentials_id
         AND uc.tenant_id = user_app_preferences.tenant_id
         AND uc.party_id = NULLIF(current_setting('app.current_user_id', true), '')::uuid
         AND uc.deleted_at IS NULL
    )
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
    AND EXISTS (
      SELECT 1
        FROM tenants.user_credentials uc
       WHERE uc.id = user_app_preferences.user_credentials_id
         AND uc.tenant_id = user_app_preferences.tenant_id
         AND uc.party_id = NULLIF(current_setting('app.current_user_id', true), '')::uuid
         AND uc.deleted_at IS NULL
    )
  );

COMMENT ON TABLE tenants.user_app_preferences IS
  'Preferencias de UI por usuario (credencial) y aplicación cliente; aisladas por tenant y usuario.';
COMMENT ON POLICY user_app_preferences_isolation ON tenants.user_app_preferences IS
  'Aísla por tenant y por el party_id del JWT, resuelto a user_credentials.';
COMMENT ON COLUMN tenants.user_app_preferences.preferences IS
  'Objeto JSON con theme, locale, sidebar_collapsed y opciones específicas de cada frontend.';

DROP TRIGGER IF EXISTS user_app_preferences_audit_trg ON tenants.user_app_preferences;
CREATE TRIGGER user_app_preferences_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.user_app_preferences
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();
