-- ============================================================================
-- 0015_localization.sql
-- Locales BCP 47 y vínculo con global.location_translations.
--
--   * global.locales: catálogo global (sin RLS).
--   * language_code de location_translations pasa a ser FK a locales.code.
--   * El fallback de idioma se resuelve en la capa de aplicación.
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regclass('global.location_translations') IS NULL THEN
    v_missing := array_append(v_missing, 'global.location_translations');
  END IF;
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el catálogo de locales; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS global.locales (
  code       varchar(10) PRIMARY KEY,
  name       text        NOT NULL,
  is_default boolean     NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT locales_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT locales_name_chk CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS locales_one_default_uq
  ON global.locales ((is_default))
  WHERE is_default;

INSERT INTO global.locales (code, name, is_default)
VALUES
  ('es',    'Español',            true),
  ('en',    'English',            false),
  ('pt-BR', 'Português (Brasil)', false)
ON CONFLICT (code) DO NOTHING;

-- Preserva traducciones existentes con locales que no estén en el seed.
INSERT INTO global.locales (code, name, is_default)
SELECT DISTINCT language_code, language_code, false
  FROM global.location_translations
 WHERE NOT EXISTS (
   SELECT 1 FROM global.locales l WHERE l.code = global.location_translations.language_code
 )
ON CONFLICT (code) DO NOTHING;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'location_translations_locale_fk'
       AND conrelid = 'global.location_translations'::regclass
  ) THEN
    ALTER TABLE global.location_translations
      ADD CONSTRAINT location_translations_locale_fk
      FOREIGN KEY (language_code) REFERENCES global.locales (code)
      ON UPDATE RESTRICT ON DELETE RESTRICT;
  END IF;
END
$$;

COMMENT ON TABLE global.locales IS
  'Locales BCP 47 soportados. Exactamente uno marcado is_default.';
COMMENT ON COLUMN global.location_translations.language_code IS
  'Locale BCP 47; FK a global.locales(code).';
