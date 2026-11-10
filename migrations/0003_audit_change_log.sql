-- ============================================================================
-- 0003_audit_change_log.sql
-- Infraestructura de auditoría: esquema audit, tabla particionada
-- audit.change_log, funciones de logging/inmutabilidad y triggers sobre las
-- tablas del Party Model.
--
-- Prerrequisitos:
--   * tenants.parties, tenants.person_profiles, tenants.organization_profiles
--     y tenants.party_roles (Módulo 1)
--   * PostgreSQL 18+ (uuidv7())
--
-- Ejecución atómica recomendada:
--   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 --single-transaction \
--        -f migrations/0003_audit_change_log.sql
--
-- La tabla es append-only. No tiene FKs hacia usuarios ni sesiones: changed_by,
-- tenant_id y session_id se guardan como valores para preservar el historial.
-- ============================================================================

CREATE SCHEMA IF NOT EXISTS audit;

-- ----------------------------------------------------------------------------
-- Preconditions
-- ----------------------------------------------------------------------------
DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;

  IF to_regclass('tenants.parties') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.parties');
  END IF;
  IF to_regclass('tenants.person_profiles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.person_profiles');
  END IF;
  IF to_regclass('tenants.organization_profiles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.organization_profiles');
  END IF;
  IF to_regclass('tenants.party_roles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.party_roles');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear audit.change_log; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica primero el Módulo 1 (tablas tenants.*) o apunta DATABASE_URL a la base correcta.';
  END IF;

  IF to_regclass('audit.change_log') IS NOT NULL THEN
    RAISE EXCEPTION 'audit.change_log ya existe; esta migración no es reejecutable';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Tabla particionada
-- ----------------------------------------------------------------------------
CREATE TABLE audit.change_log (
  id             uuid        NOT NULL DEFAULT uuidv7(),
  table_name     text        NOT NULL,
  record_id      text        NOT NULL,
  operation      text        NOT NULL,
  old_data       jsonb,
  new_data       jsonb,
  changed_fields jsonb       NOT NULL DEFAULT '{}'::jsonb,
  changed_by     uuid,
  tenant_id      uuid,
  session_id     uuid,
  changed_at     timestamptz NOT NULL DEFAULT clock_timestamp(),

  CONSTRAINT change_log_pkey PRIMARY KEY (id, changed_at),

  CONSTRAINT change_log_operation_chk
    CHECK (operation IN ('INSERT', 'UPDATE', 'SOFT_DELETE', 'DELETE')),

  CONSTRAINT change_log_table_name_chk
    CHECK (btrim(table_name) <> ''),

  CONSTRAINT change_log_record_id_chk
    CHECK (btrim(record_id) <> ''),

  CONSTRAINT change_log_changed_fields_object_chk
    CHECK (jsonb_typeof(changed_fields) = 'object'),

  CONSTRAINT change_log_old_data_object_chk
    CHECK (old_data IS NULL OR jsonb_typeof(old_data) = 'object'),

  CONSTRAINT change_log_new_data_object_chk
    CHECK (new_data IS NULL OR jsonb_typeof(new_data) = 'object')
) PARTITION BY RANGE (changed_at);

COMMENT ON TABLE audit.change_log IS
  'Auditoría central append-only. Particionada mensualmente por changed_at; sin FKs a usuarios ni sesiones.';

COMMENT ON COLUMN audit.change_log.id IS
  'UUIDv7 del evento de auditoría; la PK es compuesta porque changed_at es la clave de partición.';

COMMENT ON COLUMN audit.change_log.table_name IS
  'Tabla auditada en formato schema.table.';

COMMENT ON COLUMN audit.change_log.record_id IS
  'Valor textual del id de la fila auditada; no es FK.';

COMMENT ON COLUMN audit.change_log.operation IS
  'INSERT, UPDATE, SOFT_DELETE o DELETE.';

COMMENT ON COLUMN audit.change_log.old_data IS
  'Fila completa antes del cambio (NULL en INSERT).';

COMMENT ON COLUMN audit.change_log.new_data IS
  'Fila completa después del cambio (NULL en DELETE).';

COMMENT ON COLUMN audit.change_log.changed_fields IS
  'Solo los campos que cambiaron (o la fila completa en INSERT/DELETE).';

COMMENT ON COLUMN audit.change_log.changed_by IS
  'Valor de app.current_user_id; sin FK para preservar el historial.';

COMMENT ON COLUMN audit.change_log.tenant_id IS
  'Valor de app.current_tenant_id (fallback: tenant_id de la fila); sin FK.';

COMMENT ON COLUMN audit.change_log.session_id IS
  'Valor de app.current_session_id; sin FK para preservar el historial.';

COMMENT ON COLUMN audit.change_log.changed_at IS
  'Momento exacto del cambio; clock_timestamp() para no depender del inicio de la transacción.';

-- ----------------------------------------------------------------------------
-- Particiones mensuales: mes actual + 11 meses. El DEFAULT cubre fechas
-- fuera de ese rango; un job de mantenimiento debe crear la partición de cada
-- mes ANTES de que el DEFAULT reciba filas de ese mes (si el DEFAULT ya tiene
-- filas del rango, PostgreSQL no permite adjuntar la partición sin moverlas).
-- ----------------------------------------------------------------------------
DO $$
DECLARE
  v_month date := (date_trunc('month', now() AT TIME ZONE 'UTC'))::date;
  v_start date;
  v_end   date;
  v_name  text;
  i       int;
BEGIN
  FOR i IN 0..11 LOOP
    v_start := (v_month + make_interval(months => i))::date;
    v_end   := (v_start + interval '1 month')::date;
    v_name  := 'change_log_' || to_char(v_start, 'YYYY_MM');

    IF to_regclass(format('audit.%I', v_name)) IS NULL THEN
      EXECUTE format(
        'CREATE TABLE audit.%I PARTITION OF audit.change_log FOR VALUES FROM (%L) TO (%L)',
        v_name,
        v_start::text || ' 00:00:00+00',
        v_end::text   || ' 00:00:00+00'
      );
    END IF;
  END LOOP;

  IF to_regclass('audit.change_log_default') IS NULL THEN
    EXECUTE 'CREATE TABLE audit.change_log_default PARTITION OF audit.change_log DEFAULT';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Índices optimizados
-- ----------------------------------------------------------------------------
CREATE INDEX change_log_changed_at_idx
  ON audit.change_log (changed_at);

CREATE INDEX change_log_table_record_idx
  ON audit.change_log (table_name, record_id, changed_at DESC);

CREATE INDEX change_log_tenant_changed_idx
  ON audit.change_log (tenant_id, changed_at DESC);

CREATE INDEX change_log_changed_by_idx
  ON audit.change_log (changed_by, changed_at DESC);

CREATE INDEX change_log_changed_fields_gin
  ON audit.change_log USING GIN (changed_fields jsonb_path_ops);

COMMENT ON INDEX audit.change_log_changed_at_idx IS
  'Consultas por rango temporal (retención y auditoría por fecha).';

COMMENT ON INDEX audit.change_log_table_record_idx IS
  'Historial de una fila concreta de una tabla.';

COMMENT ON INDEX audit.change_log_tenant_changed_idx IS
  'Actividad de auditoría por tenant.';

COMMENT ON INDEX audit.change_log_changed_by_idx IS
  'Actividad de auditoría por usuario.';

COMMENT ON INDEX audit.change_log_changed_fields_gin IS
  'Búsqueda por campo cambiado dentro de changed_fields.';

-- ----------------------------------------------------------------------------
-- Función genérica de logging
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION audit.fn_log_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, audit
AS $$
DECLARE
  v_old        jsonb;
  v_new        jsonb;
  v_changed    jsonb := '{}'::jsonb;
  v_operation  text;
  v_changed_by uuid;
  v_tenant_id  uuid;
  v_session_id uuid;
  v_record_id  text;
  v_key        text;
BEGIN
  v_changed_by := NULLIF(current_setting('app.current_user_id', true), '')::uuid;
  v_tenant_id  := NULLIF(current_setting('app.current_tenant_id', true), '')::uuid;
  v_session_id := NULLIF(current_setting('app.current_session_id', true), '')::uuid;

  IF TG_OP = 'INSERT' THEN
    v_new        := to_jsonb(NEW);
    v_operation  := 'INSERT';
    v_changed    := v_new;
  ELSIF TG_OP = 'DELETE' THEN
    v_old        := to_jsonb(OLD);
    v_operation  := 'DELETE';
    v_changed    := v_old;
  ELSE
    v_old := to_jsonb(OLD);
    v_new := to_jsonb(NEW);

    IF (v_old->>'deleted_at') IS NULL AND (v_new->>'deleted_at') IS NOT NULL THEN
      v_operation := 'SOFT_DELETE';
    ELSE
      v_operation := 'UPDATE';
    END IF;

    SELECT COALESCE(jsonb_object_agg(COALESCE(n.key, o.key), n.value), '{}'::jsonb)
      INTO v_changed
      FROM jsonb_each(v_new) AS n
      FULL OUTER JOIN jsonb_each(v_old) AS o ON o.key = n.key
     WHERE n.value IS DISTINCT FROM o.value;

  END IF;

  -- La mayoría de las tablas usan `id`; employees usa `party_id` como PK.
  v_record_id := COALESCE(
    (COALESCE(v_new, v_old))->>'id',
    (COALESCE(v_new, v_old))->>'party_id'
  );

  -- Sensitive columns can be redacted per trigger via TG_ARGV, e.g.
  -- audit.fn_log_change('password_hash', 'totp_secret').
  IF TG_NARGS > 0 THEN
    FOREACH v_key IN ARRAY TG_ARGV LOOP
      IF v_old IS NOT NULL THEN
        v_old := v_old - v_key;
      END IF;
      IF v_new IS NOT NULL THEN
        v_new := v_new - v_key;
      END IF;
      v_changed := v_changed - v_key;
    END LOOP;
  END IF;

  IF v_tenant_id IS NULL THEN
    v_tenant_id := (COALESCE(v_new, v_old)->>'tenant_id')::uuid;
  END IF;

  INSERT INTO audit.change_log (
    table_name,
    record_id,
    operation,
    old_data,
    new_data,
    changed_fields,
    changed_by,
    tenant_id,
    session_id
  ) VALUES (
    TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME,
    v_record_id,
    v_operation,
    v_old,
    v_new,
    v_changed,
    v_changed_by,
    v_tenant_id,
    v_session_id
  );

  RETURN NULL;
END;
$$;

COMMENT ON FUNCTION audit.fn_log_change() IS
  'Registra INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log y calcula changed_fields.';

-- ----------------------------------------------------------------------------
-- Inmutabilidad append-only
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION audit.fn_deny_modification()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $$
BEGIN
  RAISE EXCEPTION 'audit.change_log es append-only: % no permitido', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;

COMMENT ON FUNCTION audit.fn_deny_modification() IS
  'Bloquea UPDATE/DELETE/TRUNCATE sobre audit.change_log.';

CREATE TRIGGER change_log_deny_update_delete
  BEFORE UPDATE OR DELETE ON audit.change_log
  FOR EACH ROW
  EXECUTE FUNCTION audit.fn_deny_modification();

CREATE TRIGGER change_log_deny_truncate
  BEFORE TRUNCATE ON audit.change_log
  FOR EACH STATEMENT
  EXECUTE FUNCTION audit.fn_deny_modification();

-- El trigger de TRUNCATE del padre no se clona en las particiones. Se replica
-- en cada partición para que un TRUNCATE directo sobre una partición tampoco
-- pueda evadir la inmutabilidad. Las particiones futuras deben crearse con
-- este mismo trigger.
DO $$
DECLARE
  v_partition regclass;
BEGIN
  FOR v_partition IN
    SELECT inhrelid::regclass
      FROM pg_inherits
     WHERE inhparent = 'audit.change_log'::regclass
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS change_log_deny_truncate ON %s', v_partition);
    EXECUTE format(
      'CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON %s FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification()',
      v_partition
    );
  END LOOP;
END
$$;

REVOKE UPDATE, DELETE, TRUNCATE ON audit.change_log FROM PUBLIC;

-- ----------------------------------------------------------------------------
-- Triggers sobre las tablas del Módulo 1
-- ----------------------------------------------------------------------------
DROP TRIGGER IF EXISTS parties_audit_trg ON tenants.parties;
CREATE TRIGGER parties_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.parties
  FOR EACH ROW
  EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS person_profiles_audit_trg ON tenants.person_profiles;
CREATE TRIGGER person_profiles_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.person_profiles
  FOR EACH ROW
  EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS organization_profiles_audit_trg ON tenants.organization_profiles;
CREATE TRIGGER organization_profiles_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.organization_profiles
  FOR EACH ROW
  EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS party_roles_audit_trg ON tenants.party_roles;
CREATE TRIGGER party_roles_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_roles
  FOR EACH ROW
  EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TRIGGER parties_audit_trg ON tenants.parties IS
  'Audita INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log.';
COMMENT ON TRIGGER person_profiles_audit_trg ON tenants.person_profiles IS
  'Audita INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log.';
COMMENT ON TRIGGER organization_profiles_audit_trg ON tenants.organization_profiles IS
  'Audita INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log.';
COMMENT ON TRIGGER party_roles_audit_trg ON tenants.party_roles IS
  'Audita INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log.';

-- La tabla no usa RLS: es un registro central de cumplimiento. El acceso debe
-- restringirse por GRANT al rol de auditoría; la aplicación inserta a través
-- de audit.fn_log_change() (SECURITY DEFINER).
