-- ============================================================================
-- 0005_org_structure_employment.sql
-- Estructura organizacional y empleo:
--   tenants.job_families, tenants.jobs, tenants.departments,
--   tenants.employees y tenants.employee_assignments.
--
-- Incluye:
--   * tenant_id + RLS ENABLE/FORCE en todas las tablas.
--   * COLLATE unaccent_case_insensitive en name/title.
--   * EXCLUDE USING GIST para evitar solapamiento de asignaciones.
--   * Unicidad parcial de la asignación abierta (end_date IS NULL).
--   * Triggers de auditoría audit.fn_log_change().
--
-- Prerrequisitos:
--   * global.subscriptions, tenants.parties
--   * audit.fn_log_change() (migración 0003)
--   * extensiones btree_gist y collation unaccent_case_insensitive
--   * PostgreSQL 18+ (uuidv7())
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
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'btree_gist') THEN
    v_missing := array_append(v_missing, 'extensión btree_gist');
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
      'No se puede crear la estructura organizacional; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica btree_gist/unaccent_case_insensitive.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Catálogo: familias de puestos
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.job_families (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  name        text        COLLATE "unaccent_case_insensitive" NOT NULL,
  code        text        NOT NULL,
  description text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT job_families_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT job_families_code_chk CHECK (btrim(code) <> '')
);

CREATE UNIQUE INDEX job_families_tenant_code_uq
  ON tenants.job_families (tenant_id, code)
  WHERE deleted_at IS NULL;
CREATE INDEX job_families_tenant_name_idx
  ON tenants.job_families (tenant_id, name)
  WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- Catálogo: puestos
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.jobs (
  id            uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id     uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  job_family_id uuid        NOT NULL REFERENCES tenants.job_families (id) ON DELETE RESTRICT,
  title         text        COLLATE "unaccent_case_insensitive" NOT NULL,
  code          text        NOT NULL,
  description   text,
  min_salary    numeric(14,2),
  max_salary    numeric(14,2),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT jobs_title_chk CHECK (btrim(title) <> ''),
  CONSTRAINT jobs_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT jobs_min_salary_chk CHECK (min_salary IS NULL OR min_salary >= 0),
  CONSTRAINT jobs_max_salary_chk CHECK (max_salary IS NULL OR max_salary >= 0),
  CONSTRAINT jobs_salary_range_chk
    CHECK (min_salary IS NULL OR max_salary IS NULL OR min_salary <= max_salary)
);

CREATE UNIQUE INDEX jobs_tenant_code_uq
  ON tenants.jobs (tenant_id, code)
  WHERE deleted_at IS NULL;
CREATE INDEX jobs_tenant_family_idx
  ON tenants.jobs (tenant_id, job_family_id)
  WHERE deleted_at IS NULL;
CREATE INDEX jobs_tenant_title_idx
  ON tenants.jobs (tenant_id, title)
  WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- Empleados
-- PK = party_id (1:1 con tenants.parties)
-- department_id FK se añade después de crear departments (dependencia circular).
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.employees (
  party_id         uuid        PRIMARY KEY REFERENCES tenants.parties (id) ON DELETE RESTRICT,
  tenant_id        uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  employee_code    text        NOT NULL,
  hire_date        timestamptz NOT NULL,
  status           text        NOT NULL DEFAULT 'ACTIVE',
  job_id           uuid        NOT NULL REFERENCES tenants.jobs (id) ON DELETE RESTRICT,
  department_id    uuid        NOT NULL,
  manager_party_id uuid        REFERENCES tenants.employees (party_id) ON DELETE RESTRICT,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT employees_code_chk CHECK (btrim(employee_code) <> ''),
  CONSTRAINT employees_status_chk
    CHECK (status IN ('ACTIVE', 'ON_LEAVE', 'TERMINATED')),
  CONSTRAINT employees_not_own_manager_chk
    CHECK (manager_party_id IS NULL OR manager_party_id <> party_id)
);

CREATE UNIQUE INDEX employees_tenant_code_uq
  ON tenants.employees (tenant_id, employee_code)
  WHERE deleted_at IS NULL;
CREATE INDEX employees_tenant_department_idx
  ON tenants.employees (tenant_id, department_id)
  WHERE deleted_at IS NULL;
CREATE INDEX employees_tenant_job_idx
  ON tenants.employees (tenant_id, job_id)
  WHERE deleted_at IS NULL;
CREATE INDEX employees_tenant_manager_idx
  ON tenants.employees (tenant_id, manager_party_id)
  WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- Departamentos: jerarquía recursiva + manager empleado
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.departments (
  id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id            uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  parent_department_id uuid        REFERENCES tenants.departments (id) ON DELETE RESTRICT,
  name                 text        COLLATE "unaccent_case_insensitive" NOT NULL,
  code                 text        NOT NULL,
  manager_party_id     uuid        REFERENCES tenants.employees (party_id) ON DELETE RESTRICT,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,

  CONSTRAINT departments_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT departments_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT departments_parent_not_self_chk
    CHECK (parent_department_id IS NULL OR parent_department_id <> id)
);

CREATE UNIQUE INDEX departments_tenant_code_uq
  ON tenants.departments (tenant_id, code)
  WHERE deleted_at IS NULL;
CREATE INDEX departments_tenant_parent_idx
  ON tenants.departments (tenant_id, parent_department_id)
  WHERE deleted_at IS NULL;
CREATE INDEX departments_tenant_name_idx
  ON tenants.departments (tenant_id, name)
  WHERE deleted_at IS NULL;
CREATE INDEX departments_tenant_manager_idx
  ON tenants.departments (tenant_id, manager_party_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.employees
  ADD CONSTRAINT employees_department_id_fkey
    FOREIGN KEY (department_id) REFERENCES tenants.departments (id)
    ON UPDATE RESTRICT ON DELETE RESTRICT;

-- ----------------------------------------------------------------------------
-- Historial de asignaciones
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.employee_assignments (
  id                uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id         uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  employee_party_id uuid        NOT NULL REFERENCES tenants.employees (party_id) ON DELETE RESTRICT,
  job_id            uuid        NOT NULL REFERENCES tenants.jobs (id) ON DELETE RESTRICT,
  department_id     uuid        NOT NULL REFERENCES tenants.departments (id) ON DELETE RESTRICT,
  start_date        timestamptz NOT NULL,
  end_date          timestamptz,
  salary            numeric(14,2) NOT NULL,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT employee_assignments_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT employee_assignments_salary_chk CHECK (salary >= 0)
);

CREATE INDEX employee_assignments_tenant_employee_idx
  ON tenants.employee_assignments (tenant_id, employee_party_id, start_date);
CREATE INDEX employee_assignments_tenant_job_idx
  ON tenants.employee_assignments (tenant_id, job_id);
CREATE INDEX employee_assignments_tenant_department_idx
  ON tenants.employee_assignments (tenant_id, department_id);

-- Una sola asignación abierta por empleado.
CREATE UNIQUE INDEX employee_assignments_one_open_uq
  ON tenants.employee_assignments (tenant_id, employee_party_id)
  WHERE end_date IS NULL;

-- Sin solapamiento de vigencias para el mismo empleado.
ALTER TABLE tenants.employee_assignments
  ADD CONSTRAINT employee_assignments_no_overlap
  EXCLUDE USING GIST (
    tenant_id WITH =,
    employee_party_id WITH =,
    tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamptz), '[)') WITH &&
  );

-- ----------------------------------------------------------------------------
-- RLS
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.job_families ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.job_families FORCE ROW LEVEL SECURITY;
CREATE POLICY job_families_tenant_isolation ON tenants.job_families
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE tenants.jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.jobs FORCE ROW LEVEL SECURITY;
CREATE POLICY jobs_tenant_isolation ON tenants.jobs
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE tenants.employees ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.employees FORCE ROW LEVEL SECURITY;
CREATE POLICY employees_tenant_isolation ON tenants.employees
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE tenants.departments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.departments FORCE ROW LEVEL SECURITY;
CREATE POLICY departments_tenant_isolation ON tenants.departments
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

ALTER TABLE tenants.employee_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.employee_assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY employee_assignments_tenant_isolation ON tenants.employee_assignments
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

-- ----------------------------------------------------------------------------
-- Auditoría
-- ----------------------------------------------------------------------------
DROP TRIGGER IF EXISTS job_families_audit_trg ON tenants.job_families;
CREATE TRIGGER job_families_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.job_families
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS jobs_audit_trg ON tenants.jobs;
CREATE TRIGGER jobs_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.jobs
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS employees_audit_trg ON tenants.employees;
CREATE TRIGGER employees_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.employees
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS departments_audit_trg ON tenants.departments;
CREATE TRIGGER departments_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.departments
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

DROP TRIGGER IF EXISTS employee_assignments_audit_trg ON tenants.employee_assignments;
CREATE TRIGGER employee_assignments_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.employee_assignments
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();
