-- ============================================================================
-- 0014_identity_documents_and_nationalities.sql
-- MDM de documentos de identidad y nacionalidades.
--
--   * global.identity_document_types: catálogo global + seed.
--   * tenants.party_nationalities: nacionalidades por party.
--   * tenants.party_identity_documents: documento maestro (sin fechas).
--   * tenants.party_identity_document_validities: vigencias independientes.
--
-- Los documentos se integran con global.countries y global.locations.
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF to_regclass('global.countries') IS NULL THEN
    v_missing := array_append(v_missing, 'global.countries');
  END IF;
  IF to_regclass('global.locations') IS NULL THEN
    v_missing := array_append(v_missing, 'global.locations');
  END IF;
  IF to_regclass('tenants.parties') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.parties');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('global.identity_document_types') IS NOT NULL THEN
    RAISE EXCEPTION 'global.identity_document_types ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.party_nationalities') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_nationalities ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.party_identity_documents') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_identity_documents ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.party_identity_document_validities') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.party_identity_document_validities ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el MDM de identidad; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Catálogo global de tipos de documento
-- ----------------------------------------------------------------------------
CREATE TABLE global.identity_document_types (
  id         uuid        PRIMARY KEY DEFAULT uuidv7(),
  code       text        NOT NULL UNIQUE,
  name       text        NOT NULL,
  scope      text        NOT NULL DEFAULT 'PERSON',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT identity_document_types_code_chk CHECK (code ~ '^[A-Z_]+$'),
  CONSTRAINT identity_document_types_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT identity_document_types_scope_chk CHECK (scope IN ('PERSON', 'ORGANIZATION'))
);

INSERT INTO global.identity_document_types (code, name, scope)
VALUES
  ('PASSPORT',          'Passport',          'PERSON'),
  ('NATIONAL_ID',       'National ID',       'PERSON'),
  ('DRIVER_LICENSE',    'Driver license',    'PERSON'),
  ('EU_CITIZEN_ID',     'EU citizen ID',     'PERSON'),
  ('BIRTH_CERTIFICATE', 'Birth certificate', 'PERSON')
ON CONFLICT (code) DO NOTHING;

COMMENT ON TABLE global.identity_document_types IS
  'Tipos globales de documento de identidad. Sin RLS.';

-- Necesaria para la FK compuesta que valida que la jurisdicción pertenece al
-- país emisor.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'locations_id_country_uq'
       AND conrelid = 'global.locations'::regclass
  ) THEN
    ALTER TABLE global.locations
      ADD CONSTRAINT locations_id_country_uq UNIQUE (id, country_code);
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Nacionalidades
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.party_nationalities (
  id           uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id    uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id     uuid        NOT NULL,
  country_code text        NOT NULL REFERENCES global.countries (code) ON DELETE RESTRICT,
  is_primary   boolean     NOT NULL DEFAULT false,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT party_nationalities_id_tenant_party_uq UNIQUE (id, tenant_id, party_id),
  CONSTRAINT party_nationalities_party_fk
    FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX party_nationalities_tenant_party_country_uq
  ON tenants.party_nationalities (tenant_id, party_id, country_code);
CREATE UNIQUE INDEX party_nationalities_primary_uq
  ON tenants.party_nationalities (tenant_id, party_id)
  WHERE is_primary;
CREATE INDEX party_nationalities_tenant_party_idx
  ON tenants.party_nationalities (tenant_id, party_id);

ALTER TABLE tenants.party_nationalities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_nationalities FORCE ROW LEVEL SECURITY;
CREATE POLICY party_nationalities_tenant_isolation
  ON tenants.party_nationalities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_nationalities_audit_trg ON tenants.party_nationalities;
CREATE TRIGGER party_nationalities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_nationalities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ----------------------------------------------------------------------------
-- Documentos de identidad (sin fechas)
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.party_identity_documents (
  id                      uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id               uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id                uuid        NOT NULL,
  document_type_id        uuid        NOT NULL REFERENCES global.identity_document_types (id) ON DELETE RESTRICT,
  issuing_country_code    text        NOT NULL REFERENCES global.countries (code) ON DELETE RESTRICT,
  issuing_jurisdiction_id uuid,
  document_number         text        NOT NULL,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  deleted_at              timestamptz,

  CONSTRAINT party_identity_documents_number_chk CHECK (btrim(document_number) <> ''),
  CONSTRAINT party_identity_documents_id_tenant_party_uq
    UNIQUE (id, tenant_id, party_id),
  CONSTRAINT party_identity_documents_party_fk
    FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  -- Si se informa jurisdicción, debe pertenecer al país emisor.
  CONSTRAINT party_identity_documents_jurisdiction_fk
    FOREIGN KEY (issuing_jurisdiction_id, issuing_country_code)
    REFERENCES global.locations (id, country_code)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

-- Un documento no puede repetirse en el tenant para el mismo tipo/país.
CREATE UNIQUE INDEX party_identity_documents_tenant_type_country_number_uq
  ON tenants.party_identity_documents
     (tenant_id, document_type_id, issuing_country_code, document_number)
  WHERE deleted_at IS NULL;

CREATE INDEX party_identity_documents_tenant_party_idx
  ON tenants.party_identity_documents (tenant_id, party_id)
  WHERE deleted_at IS NULL;
CREATE INDEX party_identity_documents_tenant_type_idx
  ON tenants.party_identity_documents (tenant_id, document_type_id)
  WHERE deleted_at IS NULL;
CREATE INDEX party_identity_documents_tenant_country_idx
  ON tenants.party_identity_documents (tenant_id, issuing_country_code)
  WHERE deleted_at IS NULL;
CREATE INDEX party_identity_documents_tenant_jurisdiction_idx
  ON tenants.party_identity_documents (tenant_id, issuing_jurisdiction_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.party_identity_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_identity_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY party_identity_documents_tenant_isolation
  ON tenants.party_identity_documents
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_identity_documents_audit_trg ON tenants.party_identity_documents;
CREATE TRIGGER party_identity_documents_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_identity_documents
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ----------------------------------------------------------------------------
-- Vigencias de documentos
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.party_identity_document_validities (
  id                           uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id                    uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_identity_document_id   uuid        NOT NULL,
  party_id                     uuid        NOT NULL,
  is_primary                   boolean     NOT NULL DEFAULT false,
  start_date                   timestamptz NOT NULL,
  end_date                     timestamptz,
  created_at                   timestamptz NOT NULL DEFAULT now(),
  updated_at                   timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT party_identity_document_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT party_identity_document_validities_document_fk
    FOREIGN KEY (party_identity_document_id, tenant_id, party_id)
    REFERENCES tenants.party_identity_documents (id, tenant_id, party_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX party_identity_document_validities_open_uq
  ON tenants.party_identity_document_validities (tenant_id, party_identity_document_id)
  WHERE end_date IS NULL;
CREATE UNIQUE INDEX party_identity_document_validities_primary_active_uq
  ON tenants.party_identity_document_validities (tenant_id, party_id)
  WHERE is_primary AND end_date IS NULL;
CREATE INDEX party_identity_document_validities_tenant_document_start_idx
  ON tenants.party_identity_document_validities
     (tenant_id, party_identity_document_id, start_date DESC);

ALTER TABLE tenants.party_identity_document_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_identity_document_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY party_identity_document_validities_tenant_isolation
  ON tenants.party_identity_document_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS party_identity_document_validities_audit_trg
  ON tenants.party_identity_document_validities;
CREATE TRIGGER party_identity_document_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.party_identity_document_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.party_nationalities IS
  'Nacionalidades por party; una primaria por party.';
COMMENT ON TABLE tenants.party_identity_documents IS
  'Documentos de identidad maestros; las fechas viven en validities.';
COMMENT ON TABLE tenants.party_identity_document_validities IS
  'Vigencias de documentos; una primaria abierta por party.';
