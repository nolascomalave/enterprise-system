-- ============================================================================
-- 0016_address_validities_and_department_mdm.sql
-- 1) Vigencias independientes para direcciones de Parties.
-- 2) MDM de contactos y direcciones para Departments.
--
-- Todas las tablas tenant llevan RLS, auditoría y FKs compuestas
-- (department_id/party_id, tenant_id) para bloquear cruces entre tenants.
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regclass('tenants.addresses') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.addresses');
  END IF;
  IF to_regclass('tenants.departments') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.departments');
  END IF;
  IF to_regclass('global.contact_mechanisms') IS NULL THEN
    v_missing := array_append(v_missing, 'global.contact_mechanisms');
  END IF;
  IF to_regclass('global.locations') IS NULL THEN
    v_missing := array_append(v_missing, 'global.locations');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('tenants.address_validities') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.address_validities ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.department_contact_mechanisms') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.department_contact_mechanisms ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.department_addresses') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.department_addresses ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el MDM de contactos/direcciones; faltan prerequisitos: %',
      array_to_string(v_missing, ', ');
  END IF;
END
$$;

-- ============================================================================
-- 1) DIRECCIONES DE PARTIES: is_primary -> address_validities
-- ============================================================================
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'addresses_id_tenant_party_uq'
       AND conrelid = 'tenants.addresses'::regclass
  ) THEN
    ALTER TABLE tenants.addresses
      ADD CONSTRAINT addresses_id_tenant_party_uq UNIQUE (id, tenant_id, party_id);
  END IF;
END
$$;

CREATE TABLE tenants.address_validities (
  id         uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id  uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  party_id   uuid        NOT NULL,
  address_id uuid        NOT NULL,
  type       text        NOT NULL,
  is_primary boolean     NOT NULL DEFAULT false,
  start_date timestamptz NOT NULL,
  end_date   timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT address_validities_type_chk
    CHECK (type IN ('LEGAL', 'FISCAL', 'POSTAL', 'DELIVERY')),
  CONSTRAINT address_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT address_validities_address_fk
    FOREIGN KEY (address_id, tenant_id, party_id)
    REFERENCES tenants.addresses (id, tenant_id, party_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX address_validities_open_uq
  ON tenants.address_validities (tenant_id, address_id)
  WHERE end_date IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX address_validities_primary_active_uq
  ON tenants.address_validities (tenant_id, party_id, type)
  WHERE is_primary AND end_date IS NULL AND deleted_at IS NULL;
CREATE INDEX address_validities_tenant_party_type_start_idx
  ON tenants.address_validities (tenant_id, party_id, type, start_date DESC);

ALTER TABLE tenants.address_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.address_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY address_validities_tenant_isolation
  ON tenants.address_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS address_validities_audit_trg ON tenants.address_validities;
CREATE TRIGGER address_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.address_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- Backfill de las direcciones existentes antes de eliminar is_primary.
INSERT INTO tenants.address_validities (
  tenant_id, party_id, address_id, type, is_primary,
  start_date, end_date, created_at, deleted_at
)
SELECT tenant_id, party_id, id, type, is_primary,
       created_at,
       CASE WHEN deleted_at IS NOT NULL THEN deleted_at ELSE NULL END,
       created_at, deleted_at
  FROM tenants.addresses;

DROP INDEX IF EXISTS tenants.addresses_one_primary_uq;
ALTER TABLE tenants.addresses DROP COLUMN is_primary;

COMMENT ON TABLE tenants.address_validities IS
  'Vigencias de direcciones; una primaria abierta por party/tipo.';

-- ============================================================================
-- 2) CONTACTOS DE DEPARTMENTS
-- ============================================================================
CREATE TABLE tenants.department_contact_mechanisms (
  id                     uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id              uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_id          uuid        NOT NULL,
  contact_mechanism_id   uuid        NOT NULL REFERENCES global.contact_mechanisms (id) ON DELETE RESTRICT,
  contact_mechanism_order integer    NOT NULL DEFAULT 0,
  is_primary             boolean     NOT NULL DEFAULT false,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  deleted_at             timestamptz,

  CONSTRAINT department_contact_mechanisms_order_chk
    CHECK (contact_mechanism_order >= 0),
  CONSTRAINT department_contact_mechanisms_id_tenant_department_uq
    UNIQUE (id, tenant_id, department_id),
  CONSTRAINT department_contact_mechanisms_department_fk
    FOREIGN KEY (department_id, tenant_id)
    REFERENCES tenants.departments (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_contact_mechanisms_tenant_department_idx
  ON tenants.department_contact_mechanisms (tenant_id, department_id)
  WHERE deleted_at IS NULL;
CREATE INDEX department_contact_mechanisms_tenant_contact_idx
  ON tenants.department_contact_mechanisms (tenant_id, contact_mechanism_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.department_contact_mechanisms ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_contact_mechanisms FORCE ROW LEVEL SECURITY;
CREATE POLICY department_contact_mechanisms_tenant_isolation
  ON tenants.department_contact_mechanisms
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_contact_mechanisms_audit_trg
  ON tenants.department_contact_mechanisms;
CREATE TRIGGER department_contact_mechanisms_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_contact_mechanisms
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.department_contact_mechanism_validities (
  id                            uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id                     uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_contact_mechanism_id uuid      NOT NULL,
  department_id                 uuid        NOT NULL,
  contact_type                  text        NOT NULL,
  is_primary                    boolean     NOT NULL DEFAULT false,
  start_date                    timestamptz NOT NULL,
  end_date                      timestamptz,
  created_at                    timestamptz NOT NULL DEFAULT now(),
  updated_at                    timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_contact_mechanism_validities_type_chk
    CHECK (contact_type IN ('EMAIL', 'PHONE', 'MOBILE', 'FAX', 'WEBSITE')),
  CONSTRAINT department_contact_mechanism_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT department_contact_mechanism_validities_contact_fk
    FOREIGN KEY (department_contact_mechanism_id, tenant_id, department_id)
    REFERENCES tenants.department_contact_mechanisms (id, tenant_id, department_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX department_contact_mechanism_validities_open_uq
  ON tenants.department_contact_mechanism_validities (tenant_id, department_contact_mechanism_id)
  WHERE end_date IS NULL;
CREATE UNIQUE INDEX department_contact_mechanism_validities_primary_active_uq
  ON tenants.department_contact_mechanism_validities (tenant_id, department_id, contact_type)
  WHERE is_primary AND end_date IS NULL;
CREATE INDEX department_contact_mechanism_validities_tenant_department_start_idx
  ON tenants.department_contact_mechanism_validities
     (tenant_id, department_id, start_date DESC);

ALTER TABLE tenants.department_contact_mechanism_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_contact_mechanism_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY department_contact_mechanism_validities_tenant_isolation
  ON tenants.department_contact_mechanism_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_contact_mechanism_validities_audit_trg
  ON tenants.department_contact_mechanism_validities;
CREATE TRIGGER department_contact_mechanism_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_contact_mechanism_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ============================================================================
-- 3) DIRECCIONES DE DEPARTMENTS
-- ============================================================================
CREATE TABLE tenants.department_addresses (
  id               uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id        uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_id    uuid        NOT NULL,
  city_location_id uuid        NOT NULL REFERENCES global.locations (id) ON DELETE RESTRICT,
  type             text        NOT NULL,
  line1            text        COLLATE "unaccent_case_insensitive" NOT NULL,
  line2            text        COLLATE "unaccent_case_insensitive",
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT department_addresses_type_chk
    CHECK (type IN ('LEGAL', 'FISCAL', 'POSTAL', 'DELIVERY')),
  CONSTRAINT department_addresses_line1_chk CHECK (btrim(line1) <> ''),
  CONSTRAINT department_addresses_id_tenant_department_uq
    UNIQUE (id, tenant_id, department_id),
  CONSTRAINT department_addresses_department_fk
    FOREIGN KEY (department_id, tenant_id)
    REFERENCES tenants.departments (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_addresses_tenant_department_idx
  ON tenants.department_addresses (tenant_id, department_id)
  WHERE deleted_at IS NULL;
CREATE INDEX department_addresses_tenant_city_idx
  ON tenants.department_addresses (tenant_id, city_location_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.department_addresses ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_addresses FORCE ROW LEVEL SECURITY;
CREATE POLICY department_addresses_tenant_isolation
  ON tenants.department_addresses
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_addresses_audit_trg ON tenants.department_addresses;
CREATE TRIGGER department_addresses_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_addresses
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.department_address_validities (
  id                    uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id             uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_address_id uuid        NOT NULL,
  department_id         uuid        NOT NULL,
  type                  text        NOT NULL,
  is_primary            boolean     NOT NULL DEFAULT false,
  start_date            timestamptz NOT NULL,
  end_date              timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_address_validities_type_chk
    CHECK (type IN ('LEGAL', 'FISCAL', 'POSTAL', 'DELIVERY')),
  CONSTRAINT department_address_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT department_address_validities_address_fk
    FOREIGN KEY (department_address_id, tenant_id, department_id)
    REFERENCES tenants.department_addresses (id, tenant_id, department_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE UNIQUE INDEX department_address_validities_open_uq
  ON tenants.department_address_validities (tenant_id, department_address_id)
  WHERE end_date IS NULL;
CREATE UNIQUE INDEX department_address_validities_primary_active_uq
  ON tenants.department_address_validities (tenant_id, department_id, type)
  WHERE is_primary AND end_date IS NULL;
CREATE INDEX department_address_validities_tenant_department_start_idx
  ON tenants.department_address_validities (tenant_id, department_id, start_date DESC);

ALTER TABLE tenants.department_address_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_address_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY department_address_validities_tenant_isolation
  ON tenants.department_address_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_address_validities_audit_trg
  ON tenants.department_address_validities;
CREATE TRIGGER department_address_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_address_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.department_contact_mechanisms IS
  'Contactos MDM por department con FK compuesta de tenant.';
COMMENT ON TABLE tenants.department_addresses IS
  'Direcciones MDM por department con FK compuesta de tenant.';
COMMENT ON TABLE tenants.department_contact_mechanism_validities IS
  'Vigencias de contactos de department; una primaria por tipo.';
COMMENT ON TABLE tenants.department_address_validities IS
  'Vigencias de direcciones de department; una primaria por tipo.';
