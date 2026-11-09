-- ============================================================================
-- 0001_app_sessions.sql
-- Tabla de sesiones multitenencia: app.sessions
--
-- Requisitos previos (deben existir antes de ejecutar):
--   * app.tenants(tenant_id uuid PRIMARY KEY)
--   * app.users(id uuid PRIMARY KEY)
--   * PostgreSQL 18+ (uuidv7())
--
-- Ejecución atómica recomendada:
--   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 --single-transaction \
--        -f migrations/0001_app_sessions.sql
--
-- Auditoría: este migration NO crea el esquema audit.*. Ver la nota final.
--
-- Contexto obligatorio del backend al inicio de CADA transacción (SET LOCAL
-- evita fugas entre peticiones en el pool de conexiones):
--   SET LOCAL app.current_tenant_id = 'uuid-del-tenant';
--   SET LOCAL app.current_session_id = 'uuid-de-la-sesion';
--   SET LOCAL app.current_user_id = 'uuid-del-usuario';
-- ============================================================================

CREATE SCHEMA IF NOT EXISTS app;

-- ----------------------------------------------------------------------------
-- Preconditions: fail loudly and atomically instead of leaving a partial table.
-- ----------------------------------------------------------------------------
DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;

  IF to_regclass('app.tenants') IS NULL THEN
    v_missing := array_append(v_missing, 'app.tenants');
  ELSIF NOT EXISTS (
    SELECT 1
      FROM pg_attribute
     WHERE attrelid = 'app.tenants'::regclass
       AND attname  = 'tenant_id'
       AND NOT attisdropped
       AND atttypid = 'uuid'::regtype
  ) THEN
    v_missing := array_append(v_missing, 'app.tenants.tenant_id uuid');
  END IF;

  IF to_regclass('app.users') IS NULL THEN
    v_missing := array_append(v_missing, 'app.users');
  ELSIF NOT EXISTS (
    SELECT 1
      FROM pg_attribute
     WHERE attrelid = 'app.users'::regclass
       AND attname  = 'id'
       AND NOT attisdropped
       AND atttypid = 'uuid'::regtype
  ) THEN
    v_missing := array_append(v_missing, 'app.users.id uuid');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear app.sessions; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Crea app.tenants y app.users, o apunta DATABASE_URL a una base que ya los tenga.';
  END IF;

  IF to_regclass('app.sessions') IS NOT NULL THEN
    RAISE EXCEPTION 'app.sessions ya existe; esta migración no es reejecutable';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Tabla
-- ----------------------------------------------------------------------------
-- Guardrails actualizados (2026-11-09):
-- 6. Collation "unaccent_case_insensitive" (ICU, no determinista) verificada en
--    la base. No aplica a app.sessions: no hay columnas text comparadas con
--    búsqueda de cliente; metadata es jsonb. Si se añade device_name/ip_text,
--    aplicar COLLATE "unaccent_case_insensitive" en esas comparaciones.
-- 7. EXCLUDE USING GIST: btree_gist está instalado. No aplica a sesiones:
--    se permiten varias sesiones concurrentes por usuario y no existe una
--    vigencia exclusiva que pueda solaparse. El único trigger es updated_at,
--    no un trigger de solapamiento.
CREATE TABLE app.sessions (
  id           uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id    uuid        NOT NULL,
  user_id      uuid        NOT NULL,
  token_hash   bytea       NOT NULL,
  expires_at   timestamptz NOT NULL,
  revoked_at   timestamptz,
  last_used_at timestamptz,
  metadata     jsonb       NOT NULL DEFAULT '{}'::jsonb,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,

  -- Unicidad global deliberada: el token se resuelve antes de conocer el
  -- tenant. El hash SHA-256 es aleatorio, por lo que no filtra información
  -- práctica entre tenants. Es la única excepción a "índices empiezan por
  -- tenant_id", junto con la PK.
  CONSTRAINT sessions_token_hash_key UNIQUE (token_hash),

  CONSTRAINT sessions_tenant_id_fkey
    FOREIGN KEY (tenant_id) REFERENCES app.tenants (tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,

  -- CASCADE justificado: la sesión es una composición fuerte del usuario.
  -- Al eliminar el usuario no deben quedar sesiones huérfanas ni credenciales
  -- válidas. El historial de auditoría es independiente (sin FK a sessions).
  CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES app.users (id)
    ON UPDATE RESTRICT ON DELETE CASCADE,

  CONSTRAINT sessions_token_hash_sha256_chk
    CHECK (octet_length(token_hash) = 32),

  CONSTRAINT sessions_expires_after_created_chk
    CHECK (expires_at > created_at),

  CONSTRAINT sessions_revoked_after_created_chk
    CHECK (revoked_at IS NULL OR revoked_at >= created_at),

  CONSTRAINT sessions_last_used_after_created_chk
    CHECK (last_used_at IS NULL OR last_used_at >= created_at),

  CONSTRAINT sessions_deleted_after_created_chk
    CHECK (deleted_at IS NULL OR deleted_at >= created_at),

  CONSTRAINT sessions_metadata_object_chk
    CHECK (jsonb_typeof(metadata) = 'object')
);

COMMENT ON TABLE app.sessions IS
  'Sesiones autenticadas con aislamiento por tenant. RLS ENABLE + FORCE; token solo como hash SHA-256.';

COMMENT ON COLUMN app.sessions.id IS
  'Identificador de sesión (UUIDv7, ordenable temporalmente).';

COMMENT ON COLUMN app.sessions.tenant_id IS
  'Discriminador de tenant; FK RESTRICT a app.tenants.';

COMMENT ON COLUMN app.sessions.user_id IS
  'Usuario propietario; FK CASCADE justificada en el comentario de la constraint.';

COMMENT ON COLUMN app.sessions.token_hash IS
  'SHA-256 crudo (32 bytes) del token de sesión. Nunca se almacena el token en claro.';

COMMENT ON COLUMN app.sessions.expires_at IS
  'Expiración absoluta; debe ser posterior a created_at.';

COMMENT ON COLUMN app.sessions.revoked_at IS
  'Revocación explícita (logout, rotación o incidente). Distinto de deleted_at.';

COMMENT ON COLUMN app.sessions.last_used_at IS
  'Último uso válido; permite idle timeout sin tocar created_at.';

COMMENT ON COLUMN app.sessions.metadata IS
  'Contexto no estructurado (IP, user-agent, etc.). Debe ser un objeto JSON.';

COMMENT ON COLUMN app.sessions.deleted_at IS
  'Soft delete exclusivamente con timestamptz; prohibido boolean is_deleted.';

COMMENT ON CONSTRAINT sessions_user_id_fkey ON app.sessions IS
  'ON DELETE CASCADE: la sesión es composición fuerte del usuario; se evita dejar credenciales huérfanas.';

COMMENT ON CONSTRAINT sessions_token_hash_key ON app.sessions IS
  'Unicidad global necesaria para validar el token antes de resolver el tenant.';

-- ----------------------------------------------------------------------------
-- Índices
-- Todos los índices secundarios empiezan por tenant_id para que la política
-- RLS y las consultas multi-tenant sean eficientes.
-- ----------------------------------------------------------------------------
CREATE INDEX sessions_tenant_idx
  ON app.sessions (tenant_id);

COMMENT ON INDEX app.sessions_tenant_idx IS
  'Soporte de la política RLS y de listados/limpiezas por tenant.';

CREATE INDEX sessions_active_user_idx
  ON app.sessions (tenant_id, user_id)
  WHERE revoked_at IS NULL AND deleted_at IS NULL;

COMMENT ON INDEX app.sessions_active_user_idx IS
  'Listar sesiones activas de un usuario dentro del tenant; índice parcial de vigencia.';

CREATE INDEX sessions_expiry_idx
  ON app.sessions (tenant_id, expires_at)
  WHERE revoked_at IS NULL AND deleted_at IS NULL;

COMMENT ON INDEX app.sessions_expiry_idx IS
  'Barrido de sesiones expiradas por tenant. Índice parcial que cubre deleted_at IS NULL.';

-- ----------------------------------------------------------------------------
-- Row Level Security
-- Fail-closed: sin app.current_tenant_id (o con cadena vacía), la comparación
-- es NULL y no se ve ni se permite ninguna fila.
-- ----------------------------------------------------------------------------
ALTER TABLE app.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE app.sessions FORCE ROW LEVEL SECURITY;

CREATE POLICY sessions_tenant_isolation
  ON app.sessions
  FOR ALL
  TO PUBLIC
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY sessions_tenant_isolation ON app.sessions IS
  'Aislamiento por tenant fail-closed usando NULLIF(current_setting(..., true), '''')::uuid.';

-- ----------------------------------------------------------------------------
-- updated_at mantenido por trigger (no se confía en el cliente)
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION app.fn_sessions_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

COMMENT ON FUNCTION app.fn_sessions_set_updated_at() IS
  'Fuerza updated_at = now() en cada UPDATE de app.sessions.';

CREATE TRIGGER sessions_set_updated_at_trg
  BEFORE UPDATE ON app.sessions
  FOR EACH ROW
  EXECUTE FUNCTION app.fn_sessions_set_updated_at();

-- ----------------------------------------------------------------------------
-- Privilegios
-- No se otorgan permisos porque el rol de aplicación no está definido en esta
-- base. Al crearlo, otorgar solo SELECT/INSERT/UPDATE/DELETE sobre app.sessions
-- (nunca superusuario ni BYPASSRLS).
-- ----------------------------------------------------------------------------

-- ----------------------------------------------------------------------------
-- AUDITORÍA (pendiente, no incluida activamente)
-- El estándar audit-logging exige audit.audit_log particionado por changed_at,
-- append-only, y audit.fn_log_change(). Esa infraestructura no existe todavía.
-- Cuando se cree, añadir:
--
--   CREATE TRIGGER sessions_audit_trg
--     AFTER INSERT OR UPDATE OR DELETE ON app.sessions
--     FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();
--
-- No se incluye un trigger activo para no crear una dependencia inexistente.
-- ----------------------------------------------------------------------------
