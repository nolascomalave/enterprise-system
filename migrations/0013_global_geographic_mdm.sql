-- ============================================================================
-- 0013_global_geographic_mdm.sql
-- MDM geográfico global con jerarquía flexible (ltree).
--
--   * global.countries: catálogo ISO alpha-2 (+ phone_code/currency_code).
--   * global.locations: jerarquía auto-referenciada y path ltree.
--   * global.location_translations: nombres por idioma.
--   * tenants.addresses: city_location_id reemplaza city/state de texto.
--
-- Estas tablas globales NO llevan RLS: son catálogos compartidos.
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS ltree;

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF to_regclass('tenants.addresses') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.addresses');
  END IF;
  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el MDM geográfico; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;

  -- Sin datos no hay una forma determinista de mapear city/state de texto a
  -- una fila de global.locations.
  IF EXISTS (SELECT 1 FROM tenants.addresses) THEN
    RAISE EXCEPTION
      'tenants.addresses contiene filas; migra city/state a global.locations y city_location_id manualmente'
      USING HINT = 'Haz un backfill de city_location_id antes de aplicar esta migración.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Países
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS global.countries (
  code          text        PRIMARY KEY,
  name          text        COLLATE "unaccent_case_insensitive" NOT NULL,
  phone_code    text,
  currency_code text,
  is_active     boolean     NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT countries_code_chk CHECK (code ~ '^[A-Z]{2}$'),
  CONSTRAINT countries_name_chk CHECK (btrim(name) <> '')
);

ALTER TABLE global.countries
  ADD COLUMN IF NOT EXISTS phone_code text;
ALTER TABLE global.countries
  ADD COLUMN IF NOT EXISTS currency_code text;

COMMENT ON TABLE global.countries IS
  'Catálogo global de países; code es ISO 3166-1 alpha-2. Sin RLS.';

-- ----------------------------------------------------------------------------
-- Ubicaciones jerárquicas con ltree
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS global.locations (
  id            uuid        PRIMARY KEY DEFAULT uuidv7(),
  location_type text        NOT NULL,
  parent_id     uuid        REFERENCES global.locations (id) ON DELETE RESTRICT,
  country_code  text        NOT NULL REFERENCES global.countries (code) ON DELETE RESTRICT,
  iso_code      text,
  external_id   text,
  default_name  text        COLLATE "unaccent_case_insensitive" NOT NULL,
  path          ltree       NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT locations_type_chk
    CHECK (location_type IN ('REGION', 'SUBREGION', 'STATE', 'PROVINCE', 'CITY', 'MUNICIPALITY', 'COUNTY')),
  CONSTRAINT locations_name_chk CHECK (btrim(default_name) <> ''),
  CONSTRAINT locations_parent_not_self_chk CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS locations_path_uq
  ON global.locations (path);
CREATE INDEX IF NOT EXISTS locations_path_gist_idx
  ON global.locations USING GIST (path);
CREATE INDEX IF NOT EXISTS locations_country_parent_idx
  ON global.locations (country_code, parent_id);
CREATE INDEX IF NOT EXISTS locations_parent_idx
  ON global.locations (parent_id);
CREATE INDEX IF NOT EXISTS locations_country_type_idx
  ON global.locations (country_code, location_type);
CREATE UNIQUE INDEX IF NOT EXISTS locations_country_iso_uq
  ON global.locations (country_code, iso_code)
  WHERE iso_code IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS locations_country_external_uq
  ON global.locations (country_code, external_id)
  WHERE external_id IS NOT NULL;

COMMENT ON TABLE global.locations IS
  'Jerarquía geográfica global; path ltree (world.<país>.<...>). Sin RLS.';
COMMENT ON COLUMN global.locations.path IS
  'Ruta ltree normalizada; permite ancestros con @> y descendientes con <@.';

-- ----------------------------------------------------------------------------
-- Traducciones de ubicaciones
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS global.location_translations (
  id            uuid        NOT NULL DEFAULT uuidv7(),
  location_id   uuid        NOT NULL REFERENCES global.locations (id) ON DELETE RESTRICT,
  language_code text        NOT NULL,
  name          text        COLLATE "unaccent_case_insensitive" NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT location_translations_pkey PRIMARY KEY (location_id, language_code),
  CONSTRAINT location_translations_id_uq UNIQUE (id),
  CONSTRAINT location_translations_language_chk
    CHECK (btrim(language_code) <> ''),
  CONSTRAINT location_translations_name_chk CHECK (btrim(name) <> '')
);

COMMENT ON TABLE global.location_translations IS
  'Traducciones por idioma con PK compuesta (location_id, language_code); id surrogate para Ent.';

CREATE INDEX IF NOT EXISTS location_translations_language_idx
  ON global.location_translations (language_code);
CREATE INDEX IF NOT EXISTS location_translations_name_idx
  ON global.location_translations (name);

-- ----------------------------------------------------------------------------
-- Addresses: city/state de texto -> city_location_id
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.addresses
  ADD COLUMN city_location_id uuid NOT NULL REFERENCES global.locations (id) ON DELETE RESTRICT;

ALTER TABLE tenants.addresses
  DROP CONSTRAINT IF EXISTS addresses_city_chk;
DROP INDEX IF EXISTS tenants.addresses_tenant_city_idx;
ALTER TABLE tenants.addresses
  DROP COLUMN city,
  DROP COLUMN state;

CREATE INDEX addresses_tenant_city_location_idx
  ON tenants.addresses (tenant_id, city_location_id)
  WHERE deleted_at IS NULL;

COMMENT ON COLUMN tenants.addresses.city_location_id IS
  'Ciudad/municipio en global.locations; reemplaza city/state de texto.';
