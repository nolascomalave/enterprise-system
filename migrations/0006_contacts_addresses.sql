-- ============================================================================
-- 0006_contacts_addresses.sql
-- Contactos y direcciones por Party.
--
--   * tenants.contact_mechanisms
--   * tenants.addresses
--
-- Ambas con tenant_id, RLS ENABLE/FORCE, triggers de auditoría, timestamps,
-- soft delete y un índice único parcial de is_primary por (party, type).
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
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change() (migración 0003)');
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_collation
     WHERE collname = 'unaccent_case_insensitive'
       AND collprovider = 'i'
       AND NOT collisdeterministic
  ) THEN
    v_missing := array_append(v_missing, 'collation unaccent_case_insensitive');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear contactos/direcciones; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica la collation ICU.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Contactos
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.contact_mechanisms (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id    uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  type        text        NOT NULL,
  value       text        COLLATE "unaccent_case_insensitive" NOT NULL,
  is_primary  boolean     NOT NULL DEFAULT false,
  is_verified boolean     NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT contact_mechanisms_type_chk
    CHECK (type IN ('EMAIL', 'PHONE', 'MOBILE', 'FAX', 'WEBSITE')),
  CONSTRAINT contact_mechanisms_value_chk CHECK (btrim(value) <> '')
);

CREATE INDEX contact_mechanisms_tenant_party_idx
  ON tenants.contact_mechanisms (tenant_id, party_id)
  WHERE deleted_at IS NULL;
CREATE INDEX contact_mechanisms_tenant_party_type_idx
  ON tenants.contact_mechanisms (tenant_id, party_id, type)
  WHERE deleted_at IS NULL;

-- Un solo primario por Party y tipo de contacto.
CREATE UNIQUE INDEX contact_mechanisms_one_primary_uq
  ON tenants.contact_mechanisms (tenant_id, party_id, type)
  WHERE is_primary AND deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- Direcciones
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.addresses (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id    uuid        NOT NULL REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  type        text        NOT NULL,
  line1       text        COLLATE "unaccent_case_insensitive" NOT NULL,
  line2       text        COLLATE "unaccent_case_insensitive",
  city        text        COLLATE "unaccent_case_insensitive" NOT NULL,
  state       text        COLLATE "unaccent_case_insensitive",
  postal_code text        COLLATE "unaccent_case_insensitive",
  country     text        NOT NULL,
  is_primary  boolean     NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT addresses_type_chk
    CHECK (type IN ('LEGAL', 'FISCAL', 'POSTAL', 'DELIVERY')),
  CONSTRAINT addresses_line1_chk CHECK (btrim(line1) <> ''),
  CONSTRAINT addresses_city_chk CHECK (btrim(city) <> ''),
  CONSTRAINT addresses_country_chk CHECK (country ~ '^[A-Z]{2}$')
);

CREATE INDEX addresses_tenant_party_idx
  ON tenants.addresses (tenant_id, party_id)
  WHERE deleted_at IS NULL;
CREATE INDEX addresses_tenant_party_type_idx
  ON tenants.addresses (tenant_id, party_id, type)
  WHERE deleted_at IS NULL;
CREATE INDEX addresses_tenant_city_idx
  ON tenants.addresses (tenant_id, city)
  WHERE deleted_at IS NULL;

-- Un solo primario por Party y tipo de dirección.
CREATE UNIQUE INDEX addresses_one_primary_uq
  ON tenants.addresses (tenant_id, party_id, type)
  WHERE is_primary AND deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- RLS
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.contact_mechanisms ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.contact_mechanisms FORCE ROW LEVEL SECURITY;
CREATE POLICY contact_mechanisms_tenant_isolation ON tenants.contact_mechanisms
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE tenants.addresses ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.addresses FORCE ROW LEVEL SECURITY;
CREATE POLICY addresses_tenant_isolation ON tenants.addresses
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

-- ----------------------------------------------------------------------------
-- Auditoría
-- ----------------------------------------------------------------------------
DROP TRIGGER IF EXISTS contact_mechanisms_audit_trg ON tenants.contact_mechanisms;
CREATE TRIGGER contact_mechanisms_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.contact_mechanisms
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS addresses_audit_trg ON tenants.addresses;
CREATE TRIGGER addresses_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.addresses
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();
