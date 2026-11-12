-- ============================================================================
-- 0011_corporate_groups.sql
-- Estructura organizacional: tipo de unidad, grupos corporativos y asignación
-- de departamentos a grupos.
--
--   * departments.unit_type: DIVISION (rama), BRANCH (sucursal),
--     OFFICE (oficina), DEPARTMENT (departamento).
--   * corporate_groups: agrupación lógica de departamentos del tenant.
--   * department_group_assignments: puente N:M con PK compuesta.
--   * FKs compuestas (id, tenant_id) para impedir vínculos entre tenants.
--   * RLS + auditoría en ambas tablas nuevas.
--
-- Prerrequisitos: PostgreSQL 18+ (uuidv7), tenants.departments,
-- global.subscriptions y audit.fn_log_change().
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regprocedure('pg_catalog.uuidv7()') IS NULL THEN
    v_missing := array_append(v_missing, 'PostgreSQL 18+ (pg_catalog.uuidv7())');
  END IF;
  IF to_regclass('tenants.departments') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.departments');
  END IF;
  IF to_regclass('global.subscriptions') IS NULL THEN
    v_missing := array_append(v_missing, 'global.subscriptions');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('tenants.corporate_groups') IS NOT NULL THEN
    RAISE EXCEPTION
      'tenants.corporate_groups ya existe; esta migración no es reejecutable';
  END IF;
  IF to_regclass('tenants.department_group_assignments') IS NOT NULL THEN
    RAISE EXCEPTION
      'tenants.department_group_assignments ya existe; esta migración no es reejecutable';
  END IF;
  IF EXISTS (
    SELECT 1
      FROM information_schema.columns
     WHERE table_schema = 'tenants'
       AND table_name = 'departments'
       AND column_name = 'unit_type'
  ) THEN
    RAISE EXCEPTION
      'tenants.departments.unit_type ya existe; esta migración no es reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear la estructura organizacional; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica el esquema audit.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Tipo de unidad organizacional
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.departments
  ADD COLUMN unit_type text NOT NULL DEFAULT 'DEPARTMENT';

ALTER TABLE tenants.departments
  ADD CONSTRAINT departments_unit_type_chk
    CHECK (unit_type IN ('DIVISION', 'BRANCH', 'OFFICE', 'DEPARTMENT'));

-- Necesaria para las FKs compuestas de department_group_assignments.
ALTER TABLE tenants.departments
  ADD CONSTRAINT departments_id_tenant_uq UNIQUE (id, tenant_id);

COMMENT ON COLUMN tenants.departments.unit_type IS
  'Tipo de unidad: DIVISION, BRANCH, OFFICE o DEPARTMENT.';

-- ----------------------------------------------------------------------------
-- Grupos corporativos
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.corporate_groups (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  name        text        COLLATE "unaccent_case_insensitive" NOT NULL,
  description text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT corporate_groups_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT corporate_groups_id_tenant_uq UNIQUE (id, tenant_id)
);

CREATE UNIQUE INDEX corporate_groups_tenant_name_uq
  ON tenants.corporate_groups (tenant_id, name)
  WHERE deleted_at IS NULL;
CREATE INDEX corporate_groups_tenant_active_idx
  ON tenants.corporate_groups (tenant_id)
  WHERE deleted_at IS NULL;

ALTER TABLE tenants.corporate_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.corporate_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY corporate_groups_tenant_isolation
  ON tenants.corporate_groups
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS corporate_groups_audit_trg ON tenants.corporate_groups;
CREATE TRIGGER corporate_groups_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.corporate_groups
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

-- ----------------------------------------------------------------------------
-- Asignación Departamento <-> Grupo Corporativo
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.department_group_assignments (
  id                 uuid        NOT NULL DEFAULT uuidv7(),
  tenant_id          uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_id      uuid        NOT NULL,
  corporate_group_id uuid        NOT NULL,
  created_at         timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_group_assignments_pkey
    PRIMARY KEY (department_id, corporate_group_id),
  CONSTRAINT department_group_assignments_id_uq UNIQUE (id),

  -- Las FKs compuestas garantizan que departamento y grupo pertenecen al
  -- mismo tenant, incluso si una ruta de escritura omitiera la comprobación
  -- de la capa de aplicación.
  CONSTRAINT department_group_assignments_department_fk
    FOREIGN KEY (department_id, tenant_id)
    REFERENCES tenants.departments (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,

  CONSTRAINT department_group_assignments_group_fk
    FOREIGN KEY (corporate_group_id, tenant_id)
    REFERENCES tenants.corporate_groups (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_group_assignments_tenant_department_idx
  ON tenants.department_group_assignments (tenant_id, department_id);
CREATE INDEX department_group_assignments_tenant_group_idx
  ON tenants.department_group_assignments (tenant_id, corporate_group_id);

ALTER TABLE tenants.department_group_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_group_assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY department_group_assignments_tenant_isolation
  ON tenants.department_group_assignments
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_group_assignments_audit_trg
  ON tenants.department_group_assignments;
CREATE TRIGGER department_group_assignments_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_group_assignments
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.corporate_groups IS
  'Agrupación lógica de departamentos dentro de un tenant.';
COMMENT ON TABLE tenants.department_group_assignments IS
  'Asignación N:M entre departamentos y grupos corporativos del mismo tenant.';
