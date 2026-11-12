-- ============================================================================
-- 0018_catalog_translations.sql
-- Traducciones normalizadas para catálogos globales y departments.
--
--   * global.identity_document_type_translations
--   * global.role_type_translations
--   * tenants.department_translations
--
-- El fallback se resuelve en Go: locale solicitado -> locale default ->
-- valor base (code o departments.name).
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regclass('global.locales') IS NULL THEN
    v_missing := array_append(v_missing, 'global.locales');
  END IF;
  IF to_regclass('global.identity_document_types') IS NULL THEN
    v_missing := array_append(v_missing, 'global.identity_document_types');
  END IF;
  IF to_regclass('global.role_types') IS NULL THEN
    v_missing := array_append(v_missing, 'global.role_types');
  END IF;
  IF to_regclass('tenants.departments') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.departments');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('global.identity_document_type_translations') IS NOT NULL THEN
    RAISE EXCEPTION 'identity_document_type_translations ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('global.role_type_translations') IS NOT NULL THEN
    RAISE EXCEPTION 'role_type_translations ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.department_translations') IS NOT NULL THEN
    RAISE EXCEPTION 'department_translations ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear traducciones; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

-- ============================================================================
-- Tipos de documento de identidad
-- ============================================================================
CREATE TABLE global.identity_document_type_translations (
  id                         uuid         PRIMARY KEY DEFAULT uuidv7(),
  identity_document_type_id  uuid         NOT NULL REFERENCES global.identity_document_types (id) ON DELETE RESTRICT,
  locale_code                varchar(10)  NOT NULL REFERENCES global.locales (code) ON DELETE RESTRICT,
  name                       text         COLLATE "unaccent_case_insensitive" NOT NULL,
  description                text         COLLATE "unaccent_case_insensitive",
  created_at                 timestamptz  NOT NULL DEFAULT now(),
  updated_at                 timestamptz  NOT NULL DEFAULT now(),

  CONSTRAINT identity_document_type_translations_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT identity_document_type_translations_uq UNIQUE (identity_document_type_id, locale_code)
);

CREATE INDEX identity_document_type_translations_locale_idx
  ON global.identity_document_type_translations (locale_code);

-- Los nombres existentes eran ingleses.
INSERT INTO global.identity_document_type_translations
  (identity_document_type_id, locale_code, name)
SELECT id, 'en', name
  FROM global.identity_document_types;

INSERT INTO global.identity_document_type_translations
  (identity_document_type_id, locale_code, name)
SELECT id, 'es',
       CASE code
         WHEN 'PASSPORT' THEN 'Pasaporte'
         WHEN 'NATIONAL_ID' THEN 'Documento nacional de identidad'
         WHEN 'DRIVER_LICENSE' THEN 'Licencia de conducir'
         WHEN 'EU_CITIZEN_ID' THEN 'Documento de ciudadano de la Unión Europea'
         WHEN 'BIRTH_CERTIFICATE' THEN 'Acta de nacimiento'
         ELSE name
       END
  FROM global.identity_document_types;

ALTER TABLE global.identity_document_types DROP COLUMN name;

COMMENT ON TABLE global.identity_document_type_translations IS
  'Traducciones de tipos de documento; el fallback final es el code.';

-- ============================================================================
-- Tipos de rol
-- ============================================================================
CREATE TABLE global.role_type_translations (
  id            uuid        PRIMARY KEY DEFAULT uuidv7(),
  role_type_id  uuid        NOT NULL REFERENCES global.role_types (id) ON DELETE RESTRICT,
  locale_code   varchar(10) NOT NULL REFERENCES global.locales (code) ON DELETE RESTRICT,
  name          text        COLLATE "unaccent_case_insensitive" NOT NULL,
  description   text        COLLATE "unaccent_case_insensitive",
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT role_type_translations_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT role_type_translations_uq UNIQUE (role_type_id, locale_code)
);

CREATE INDEX role_type_translations_locale_idx
  ON global.role_type_translations (locale_code);

INSERT INTO global.role_type_translations
  (role_type_id, locale_code, name, description)
SELECT id, 'en', name, description
  FROM global.role_types;

INSERT INTO global.role_type_translations
  (role_type_id, locale_code, name, description)
SELECT id, 'es',
       CASE code
         WHEN 'TENANT' THEN 'Organización'
         WHEN 'ADMIN' THEN 'Administrador'
         ELSE name
       END,
       description
  FROM global.role_types;

ALTER TABLE global.role_types
  DROP COLUMN name,
  DROP COLUMN description;

COMMENT ON TABLE global.role_type_translations IS
  'Traducciones de tipos de rol; el fallback final es el code.';

-- ============================================================================
-- Departments
-- ============================================================================
CREATE TABLE tenants.department_translations (
  id            uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id     uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_id uuid        NOT NULL,
  locale_code   varchar(10) NOT NULL REFERENCES global.locales (code) ON DELETE RESTRICT,
  name          text        COLLATE "unaccent_case_insensitive" NOT NULL,
  description   text        COLLATE "unaccent_case_insensitive",
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_translations_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT department_translations_uq UNIQUE (tenant_id, department_id, locale_code),
  CONSTRAINT department_translations_department_fk
    FOREIGN KEY (department_id, tenant_id)
    REFERENCES tenants.departments (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_translations_tenant_department_idx
  ON tenants.department_translations (tenant_id, department_id);
CREATE INDEX department_translations_locale_idx
  ON tenants.department_translations (locale_code);

ALTER TABLE tenants.department_translations ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_translations FORCE ROW LEVEL SECURITY;
CREATE POLICY department_translations_tenant_isolation
  ON tenants.department_translations
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_translations_audit_trg ON tenants.department_translations;
CREATE TRIGGER department_translations_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_translations
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.department_translations IS
  'Traducciones de departments; fallback final a departments.name.';
