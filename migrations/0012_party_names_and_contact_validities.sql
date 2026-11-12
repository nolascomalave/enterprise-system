-- ============================================================================
-- 0012_party_names_and_contact_validities.sql
-- MDM de nombres y vigencias independientes.
--
--   * global.name_types: catálogo de tipos de nombre.
--   * tenants.party_names: valores sin fechas.
--   * tenants.party_name_validities: vigencias por nombre.
--   * tenants.party_contact_mechanism_validities: vigencias por contacto.
--   * Las fechas salen de party_contact_mechanisms y se migran a la nueva
--     tabla de vigencias.
--
-- Índices parciales garantizan un único registro primario vigente
-- (end_date IS NULL) por party + tipo.
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
  IF to_regclass('tenants.party_contact_mechanisms') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.party_contact_mechanisms');
  END IF;
  IF to_regclass('global.contact_mechanisms') IS NULL THEN
    v_missing := array_append(v_missing, 'global.contact_mechanisms');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('global.name_types') IS NOT NULL THEN
    RAISE EXCEPTION 'global.name_types ya existe; esta migración no es reejecutable';
  END IF;
  IF to_regclass('tenants.party_names') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_names ya existe; esta migración no es reejecutable';
  END IF;
  IF to_regclass('tenants.party_name_validities') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_name_validities ya existe; esta migración no es reejecutable';
  END IF;
  IF to_regclass('tenants.party_contact_mechanism_validities') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_contact_mechanism_validities ya existe; esta migración no es reejecutable';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'tenants' AND table_name = 'party_contact_mechanisms'
       AND column_name = 'start_date'
  ) THEN
    RAISE EXCEPTION 'tenants.party_contact_mechanisms.start_date no existe; migración ya aplicada o esquema inesperado';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el MDM de nombres; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica el esquema audit.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Catálogo global de tipos de nombre
-- ----------------------------------------------------------------------------
CREATE TABLE global.name_types (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  code        text        NOT NULL UNIQUE,
  name        text        NOT NULL,
  description text,
  is_system   boolean     NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT name_types_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT name_types_name_chk CHECK (btrim(name) <> '')
);

INSERT INTO global.name_types (code, name, description)
VALUES
  ('FIRST',  'First name',  'Given name'),
  ('LAST',   'Last name',   'Family name'),
  ('LEGAL',  'Legal name',  'Registered legal name'),
  ('TRADE',  'Trade name',  'Commercial or trade name'),
  ('ALIAS',  'Alias',       'Alternative name'),
  ('FORMER', 'Former name', 'Previous name kept for history')
ON CONFLICT (code) DO NOTHING;

COMMENT ON TABLE global.name_types IS
  'Catálogo global de tipos de nombre. Sin tenant_id ni RLS.';

-- ----------------------------------------------------------------------------
-- Nombres por Party (sin fechas)
-- ----------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'parties_id_tenant_uq'
       AND conrelid = 'tenants.parties'::regclass
  ) THEN
    ALTER TABLE tenants.parties
      ADD CONSTRAINT parties_id_tenant_uq UNIQUE (id, tenant_id);
  END IF;
END
$$;

CREATE TABLE tenants.party_names (
  id           uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id    uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id     uuid        NOT NULL,
  name_type_id uuid        NOT NULL REFERENCES global.name_types (id) ON DELETE RESTRICT,
  value        text        COLLATE "unaccent_case_insensitive" NOT NULL,
  is_primary   boolean     NOT NULL DEFAULT false,
  ordering     integer     NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,

  CONSTRAINT party_names_value_chk CHECK (btrim(value) <> ''),
  CONSTRAINT party_names_ordering_chk CHECK (ordering >= 0),
  -- Impide crear nombres para parties de otro tenant.
  CONSTRAINT party_names_party_fk
    FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  -- Soporta la FK compuesta de party_name_validities y evita que una vigencia
  -- apunte a un nombre de otro tenant/party/tipo.
  CONSTRAINT party_names_id_tenant_party_type_uq
    UNIQUE (id, tenant_id, party_id, name_type_id)
);

CREATE UNIQUE INDEX party_names_tenant_party_type_value_uq
  ON tenants.party_names (tenant_id, party_id, name_type_id, value)
  WHERE deleted_at IS NULL;
CREATE INDEX party_names_tenant_party_type_idx
  ON tenants.party_names (tenant_id, party_id, name_type_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.party_names ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_names FORCE ROW LEVEL SECURITY;
CREATE POLICY party_names_tenant_isolation
  ON tenants.party_names
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_names_audit_trg ON tenants.party_names;
CREATE TRIGGER party_names_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_names
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ----------------------------------------------------------------------------
-- Vigencias de nombres
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.party_name_validities (
  id            uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id     uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_name_id uuid        NOT NULL,
  party_id      uuid        NOT NULL,
  name_type_id  uuid        NOT NULL,
  is_primary    boolean     NOT NULL DEFAULT false,
  start_date    timestamptz NOT NULL,
  end_date      timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT party_name_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),

  CONSTRAINT party_name_validities_name_fk
    FOREIGN KEY (party_name_id, tenant_id, party_id, name_type_id)
    REFERENCES tenants.party_names (id, tenant_id, party_id, name_type_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

-- Solo un nombre primario vigente por party y tipo.
CREATE UNIQUE INDEX party_name_validities_primary_active_uq
  ON tenants.party_name_validities (tenant_id, party_id, name_type_id)
  WHERE is_primary AND end_date IS NULL;
-- Un nombre no puede tener dos vigencias abiertas simultáneamente.
CREATE UNIQUE INDEX party_name_validities_open_uq
  ON tenants.party_name_validities (tenant_id, party_name_id)
  WHERE end_date IS NULL;
CREATE INDEX party_name_validities_tenant_name_start_idx
  ON tenants.party_name_validities (tenant_id, party_name_id, start_date DESC);

ALTER TABLE tenants.party_name_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_name_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY party_name_validities_tenant_isolation
  ON tenants.party_name_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_name_validities_audit_trg ON tenants.party_name_validities;
CREATE TRIGGER party_name_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_name_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ----------------------------------------------------------------------------
-- Vigencias de contactos: se migran las fechas existentes y se eliminan de la
-- tabla principal.
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.party_contact_mechanisms
  ADD CONSTRAINT party_contact_mechanisms_id_tenant_party_uq
    UNIQUE (id, tenant_id, party_id);

CREATE TABLE tenants.party_contact_mechanism_validities (
  id                          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id                   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_contact_mechanism_id  uuid        NOT NULL,
  party_id                    uuid        NOT NULL,
  contact_mechanism_id        uuid        NOT NULL REFERENCES global.contact_mechanisms (id) ON DELETE RESTRICT,
  contact_type                text        NOT NULL,
  is_primary                  boolean     NOT NULL DEFAULT false,
  start_date                  timestamptz NOT NULL,
  end_date                    timestamptz,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  updated_at                  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT party_contact_mechanism_validities_type_chk
    CHECK (contact_type IN ('EMAIL', 'PHONE', 'MOBILE', 'FAX', 'WEBSITE')),
  CONSTRAINT party_contact_mechanism_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),

  CONSTRAINT party_contact_mechanism_validities_relation_fk
    FOREIGN KEY (party_contact_mechanism_id, tenant_id, party_id)
    REFERENCES tenants.party_contact_mechanisms (id, tenant_id, party_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

INSERT INTO tenants.party_contact_mechanism_validities (
  id, tenant_id, party_contact_mechanism_id, party_id, contact_mechanism_id,
  contact_type, is_primary, start_date, end_date, created_at, updated_at
)
SELECT pcm.id, pcm.tenant_id, pcm.id, pcm.party_id, pcm.contact_mechanism_id,
       cm.type, pcm.is_primary, pcm.start_date, pcm.end_date, pcm.created_at, pcm.updated_at
  FROM tenants.party_contact_mechanisms pcm
  JOIN global.contact_mechanisms cm ON cm.id = pcm.contact_mechanism_id;

-- Un contacto global solo puede estar vigente una vez por tenant, para que el
-- login por email siga siendo determinista.
CREATE UNIQUE INDEX party_contact_mechanism_validities_active_contact_uq
  ON tenants.party_contact_mechanism_validities (tenant_id, contact_mechanism_id)
  WHERE end_date IS NULL;

-- Solo un contacto primario vigente por party y tipo.
CREATE UNIQUE INDEX party_contact_mechanism_validities_primary_active_uq
  ON tenants.party_contact_mechanism_validities (tenant_id, party_id, contact_type)
  WHERE is_primary AND end_date IS NULL;
CREATE UNIQUE INDEX party_contact_mechanism_validities_open_uq
  ON tenants.party_contact_mechanism_validities (tenant_id, party_contact_mechanism_id)
  WHERE end_date IS NULL;
CREATE INDEX party_contact_mechanism_validities_tenant_relation_start_idx
  ON tenants.party_contact_mechanism_validities (tenant_id, party_contact_mechanism_id, start_date DESC);

ALTER TABLE tenants.party_contact_mechanism_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_contact_mechanism_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY party_contact_mechanism_validities_tenant_isolation
  ON tenants.party_contact_mechanism_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_contact_mechanism_validities_audit_trg
  ON tenants.party_contact_mechanism_validities;
CREATE TRIGGER party_contact_mechanism_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_contact_mechanism_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- Eliminar las fechas de la tabla principal.
DROP INDEX IF EXISTS tenants.party_contact_mechanisms_active_contact_uq;
ALTER TABLE tenants.party_contact_mechanisms
  DROP CONSTRAINT IF EXISTS party_contact_mechanisms_dates_chk;
ALTER TABLE tenants.party_contact_mechanisms
  DROP COLUMN start_date,
  DROP COLUMN end_date;

COMMENT ON TABLE tenants.party_names IS
  'Nombres MDM por party; los valores no llevan fechas.';
COMMENT ON TABLE tenants.party_name_validities IS
  'Vigencias de nombres; solo una primaria abierta por party/tipo.';
COMMENT ON TABLE tenants.party_contact_mechanism_validities IS
  'Vigencias de contactos; solo una primaria abierta por party/tipo.';
