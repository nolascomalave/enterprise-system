-- ============================================================================
-- 0020_rbac_catalogs_and_roles.sql
-- Fase 1 del módulo RBAC: catálogos globales, menús y roles tenant-scoped.
--
-- Catálogos globales (sin RLS):
--   * global.system_functions
--   * global.menu_templates
--   * global.menu_template_functions
--
-- Tablas tenant-scoped (RLS + auditoría):
--   * tenants.menu_categories
--   * tenants.menu_options
--   * tenants.menu_option_functions
--   * tenants.menu_option_categories
--   * tenants.tenant_roles
--   * tenants.role_menu_options
--   * tenants.user_role_assignments
--   * tenants.department_menu_options + _validities
--   * tenants.position_menu_options + _validities
--   * tenants.employee_menu_option_overrides
--
-- Las tablas de vigencia usan EXCLUDE USING GIST para impedir solapamientos.
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
  IF to_regclass('tenants.user_credentials') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.user_credentials');
  END IF;
  IF to_regclass('tenants.employees') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.employees');
  END IF;
  IF to_regclass('tenants.jobs') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.jobs');
  END IF;
  IF to_regclass('tenants.departments') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.departments');
  END IF;
  IF to_regclass('global.client_applications') IS NULL THEN
    v_missing := array_append(v_missing, 'global.client_applications');
  END IF;
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'btree_gist') THEN
    v_missing := array_append(v_missing, 'extensión btree_gist');
  END IF;

  -- Guardias de reejecución
  IF to_regclass('global.system_functions') IS NOT NULL THEN
    RAISE EXCEPTION 'global.system_functions ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('global.menu_templates') IS NOT NULL THEN
    RAISE EXCEPTION 'global.menu_templates ya existe; migración no reejecutable';
  END IF;
  IF to_regclass('tenants.tenant_roles') IS NOT NULL THEN
    RAISE EXCEPTION 'tenants.tenant_roles ya existe; migración no reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede crear el módulo RBAC; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica btree_gist.';
  END IF;
END
$$;

-- ============================================================================
-- 1) CATÁLOGOS GLOBALES
-- ============================================================================

CREATE TABLE global.system_functions (
  id              uuid        PRIMARY KEY DEFAULT uuidv7(),
  code            text        NOT NULL,
  name            text        NOT NULL,
  description     text,
  function_scope  text        NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,

  CONSTRAINT system_functions_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT system_functions_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT system_functions_scope_chk
    CHECK (function_scope IN ('PLATFORM', 'TENANT'))
);

CREATE UNIQUE INDEX system_functions_code_uq
  ON global.system_functions (code) WHERE deleted_at IS NULL;
CREATE INDEX system_functions_scope_idx
  ON global.system_functions (function_scope) WHERE deleted_at IS NULL;

COMMENT ON TABLE global.system_functions IS
  'Catálogo de funciones de negocio que el RBAC puede conceder o exigir.';

CREATE TABLE global.menu_templates (
  id                 uuid        PRIMARY KEY DEFAULT uuidv7(),
  code               text        NOT NULL,
  name               text        NOT NULL,
  client_type        text        NOT NULL,
  application_scope  text        NOT NULL,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,

  CONSTRAINT menu_templates_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT menu_templates_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT menu_templates_client_type_chk
    CHECK (client_type IN ('FIRST_PARTY', 'THIRD_PARTY')),
  CONSTRAINT menu_templates_application_scope_chk
    CHECK (application_scope IN ('PLATFORM', 'TENANT_SPECIFIC'))
);

CREATE UNIQUE INDEX menu_templates_code_uq
  ON global.menu_templates (code) WHERE deleted_at IS NULL;

COMMENT ON TABLE global.menu_templates IS
  'Plantillas de menú compartidas para diferentes tipos de aplicación cliente.';

CREATE TABLE global.menu_template_functions (
  id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
  template_id         uuid        NOT NULL REFERENCES global.menu_templates (id) ON DELETE RESTRICT,
  system_function_id  uuid        NOT NULL REFERENCES global.system_functions (id) ON DELETE RESTRICT,
  is_required         boolean     NOT NULL DEFAULT false,
  created_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT menu_template_functions_uq
    UNIQUE (template_id, system_function_id)
);

CREATE INDEX menu_template_functions_template_idx
  ON global.menu_template_functions (template_id);
CREATE INDEX menu_template_functions_function_idx
  ON global.menu_template_functions (system_function_id);

COMMENT ON TABLE global.menu_template_functions IS
  'Funciones requeridas u opcionales incluidas en una plantilla de menú.';

-- ============================================================================
-- 2) MENÚS Y CATEGORÍAS TENANT-SCOPED
-- ============================================================================

CREATE TABLE tenants.menu_categories (
  id          uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id   uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  parent_id   uuid        REFERENCES tenants.menu_categories (id) ON DELETE RESTRICT,
  code        text        NOT NULL,
  name        text        NOT NULL COLLATE "unaccent_case_insensitive",
  sort_order  integer     NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT menu_categories_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT menu_categories_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT menu_categories_sort_order_chk CHECK (sort_order >= 0),
  CONSTRAINT menu_categories_parent_not_self_chk
    CHECK (parent_id IS NULL OR parent_id <> id),
  CONSTRAINT menu_categories_id_tenant_uq UNIQUE (id, tenant_id)
);

CREATE UNIQUE INDEX menu_categories_tenant_code_uq
  ON tenants.menu_categories (tenant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX menu_categories_tenant_parent_idx
  ON tenants.menu_categories (tenant_id, parent_id) WHERE deleted_at IS NULL;
CREATE INDEX menu_categories_tenant_sort_idx
  ON tenants.menu_categories (tenant_id, sort_order) WHERE deleted_at IS NULL;

ALTER TABLE tenants.menu_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.menu_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY menu_categories_tenant_isolation
  ON tenants.menu_categories
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS menu_categories_audit_trg ON tenants.menu_categories;
CREATE TRIGGER menu_categories_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.menu_categories
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.menu_categories IS
  'Categorías jerárquicas de menú propias de cada tenant.';

CREATE TABLE tenants.menu_options (
  id           uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id    uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  template_id  uuid        REFERENCES global.menu_templates (id) ON DELETE RESTRICT,
  code         text        NOT NULL,
  name         text        NOT NULL COLLATE "unaccent_case_insensitive",
  icon         text,
  route        text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,

  CONSTRAINT menu_options_code_chk CHECK (btrim(code) <> ''),
  CONSTRAINT menu_options_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT menu_options_id_tenant_uq UNIQUE (id, tenant_id)
);

CREATE UNIQUE INDEX menu_options_tenant_code_uq
  ON tenants.menu_options (tenant_id, code) WHERE deleted_at IS NULL;
CREATE INDEX menu_options_tenant_template_idx
  ON tenants.menu_options (tenant_id, template_id) WHERE deleted_at IS NULL;
CREATE INDEX menu_options_tenant_name_idx
  ON tenants.menu_options (tenant_id, name) WHERE deleted_at IS NULL;

ALTER TABLE tenants.menu_options ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.menu_options FORCE ROW LEVEL SECURITY;
CREATE POLICY menu_options_tenant_isolation
  ON tenants.menu_options
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS menu_options_audit_trg ON tenants.menu_options;
CREATE TRIGGER menu_options_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.menu_options
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.menu_options IS
  'Opciones de menú configurables por tenant, opcionalmente basadas en plantillas.';

CREATE TABLE tenants.menu_option_functions (
  id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id           uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  menu_option_id      uuid        NOT NULL,
  system_function_id  uuid        NOT NULL REFERENCES global.system_functions (id) ON DELETE RESTRICT,
  is_required         boolean     NOT NULL DEFAULT false,
  created_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT menu_option_functions_uq
    UNIQUE (tenant_id, menu_option_id, system_function_id),
  CONSTRAINT menu_option_functions_option_fk
    FOREIGN KEY (menu_option_id, tenant_id)
    REFERENCES tenants.menu_options (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX menu_option_functions_tenant_option_idx
  ON tenants.menu_option_functions (tenant_id, menu_option_id);
CREATE INDEX menu_option_functions_tenant_function_idx
  ON tenants.menu_option_functions (tenant_id, system_function_id);

ALTER TABLE tenants.menu_option_functions ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.menu_option_functions FORCE ROW LEVEL SECURITY;
CREATE POLICY menu_option_functions_tenant_isolation
  ON tenants.menu_option_functions
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS menu_option_functions_audit_trg ON tenants.menu_option_functions;
CREATE TRIGGER menu_option_functions_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.menu_option_functions
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.menu_option_functions IS
  'Funciones de sistema requeridas u opcionales para visualizar una opción de menú.';

CREATE TABLE tenants.menu_option_categories (
  id                uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id         uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  menu_category_id  uuid        NOT NULL,
  menu_option_id    uuid        NOT NULL,
  sort_order        integer     NOT NULL DEFAULT 0,
  created_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT menu_option_categories_sort_order_chk CHECK (sort_order >= 0),
  CONSTRAINT menu_option_categories_uq
    UNIQUE (tenant_id, menu_category_id, menu_option_id),
  CONSTRAINT menu_option_categories_category_fk
    FOREIGN KEY (menu_category_id, tenant_id)
    REFERENCES tenants.menu_categories (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT menu_option_categories_option_fk
    FOREIGN KEY (menu_option_id, tenant_id)
    REFERENCES tenants.menu_options (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX menu_option_categories_tenant_category_idx
  ON tenants.menu_option_categories (tenant_id, menu_category_id);
CREATE INDEX menu_option_categories_tenant_option_idx
  ON tenants.menu_option_categories (tenant_id, menu_option_id);

ALTER TABLE tenants.menu_option_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.menu_option_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY menu_option_categories_tenant_isolation
  ON tenants.menu_option_categories
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS menu_option_categories_audit_trg ON tenants.menu_option_categories;
CREATE TRIGGER menu_option_categories_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.menu_option_categories
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.menu_option_categories IS
  'Relación N:M entre categorías y opciones de menú con ordenamiento.';

-- ============================================================================
-- 3) ROLES Y ASIGNACIONES TENANT-SCOPED
-- ============================================================================

CREATE TABLE tenants.tenant_roles (
  id                    uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id             uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  client_application_id uuid        REFERENCES global.client_applications (id) ON DELETE RESTRICT,
  name                  text        NOT NULL COLLATE "unaccent_case_insensitive",
  description           text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  deleted_at            timestamptz,

  CONSTRAINT tenant_roles_name_chk CHECK (btrim(name) <> ''),
  CONSTRAINT tenant_roles_id_tenant_uq UNIQUE (id, tenant_id)
);

CREATE UNIQUE INDEX tenant_roles_tenant_name_uq
  ON tenants.tenant_roles (tenant_id, name) WHERE deleted_at IS NULL;
CREATE INDEX tenant_roles_tenant_application_idx
  ON tenants.tenant_roles (tenant_id, client_application_id) WHERE deleted_at IS NULL;

ALTER TABLE tenants.tenant_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.tenant_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_roles_tenant_isolation
  ON tenants.tenant_roles
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS tenant_roles_audit_trg ON tenants.tenant_roles;
CREATE TRIGGER tenant_roles_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.tenant_roles
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.tenant_roles IS
  'Roles administrables por tenant para conceder opciones de menú a usuarios.';

CREATE TABLE tenants.role_menu_options (
  id              uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id       uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  tenant_role_id  uuid        NOT NULL,
  menu_option_id  uuid        NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT role_menu_options_uq
    UNIQUE (tenant_id, tenant_role_id, menu_option_id),
  CONSTRAINT role_menu_options_role_fk
    FOREIGN KEY (tenant_role_id, tenant_id)
    REFERENCES tenants.tenant_roles (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT role_menu_options_option_fk
    FOREIGN KEY (menu_option_id, tenant_id)
    REFERENCES tenants.menu_options (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX role_menu_options_tenant_role_idx
  ON tenants.role_menu_options (tenant_id, tenant_role_id);
CREATE INDEX role_menu_options_tenant_option_idx
  ON tenants.role_menu_options (tenant_id, menu_option_id);

ALTER TABLE tenants.role_menu_options ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.role_menu_options FORCE ROW LEVEL SECURITY;
CREATE POLICY role_menu_options_tenant_isolation
  ON tenants.role_menu_options
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS role_menu_options_audit_trg ON tenants.role_menu_options;
CREATE TRIGGER role_menu_options_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.role_menu_options
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.role_menu_options IS
  'Opciones de menú concedidas a un rol.';

-- Constraints compuestos necesarios para las FKs de asignaciones
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'user_credentials_id_tenant_uq'
       AND conrelid = 'tenants.user_credentials'::regclass
  ) THEN
    ALTER TABLE tenants.user_credentials
      ADD CONSTRAINT user_credentials_id_tenant_uq UNIQUE (id, tenant_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'employees_party_tenant_uq'
       AND conrelid = 'tenants.employees'::regclass
  ) THEN
    ALTER TABLE tenants.employees
      ADD CONSTRAINT employees_party_tenant_uq UNIQUE (party_id, tenant_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'jobs_id_tenant_uq'
       AND conrelid = 'tenants.jobs'::regclass
  ) THEN
    ALTER TABLE tenants.jobs
      ADD CONSTRAINT jobs_id_tenant_uq UNIQUE (id, tenant_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'departments_id_tenant_uq'
       AND conrelid = 'tenants.departments'::regclass
  ) THEN
    ALTER TABLE tenants.departments
      ADD CONSTRAINT departments_id_tenant_uq UNIQUE (id, tenant_id);
  END IF;
END
$$;

CREATE TABLE tenants.user_role_assignments (
  id                   uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id            uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  user_credentials_id  uuid        NOT NULL,
  tenant_role_id       uuid        NOT NULL,
  is_granted           boolean     NOT NULL DEFAULT true,
  assigned_at          timestamptz NOT NULL DEFAULT now(),
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT user_role_assignments_uq
    UNIQUE (tenant_id, user_credentials_id, tenant_role_id),
  CONSTRAINT user_role_assignments_user_fk
    FOREIGN KEY (user_credentials_id, tenant_id)
    REFERENCES tenants.user_credentials (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT user_role_assignments_role_fk
    FOREIGN KEY (tenant_role_id, tenant_id)
    REFERENCES tenants.tenant_roles (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX user_role_assignments_tenant_user_idx
  ON tenants.user_role_assignments (tenant_id, user_credentials_id);
CREATE INDEX user_role_assignments_tenant_role_idx
  ON tenants.user_role_assignments (tenant_id, tenant_role_id);

ALTER TABLE tenants.user_role_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.user_role_assignments FORCE ROW LEVEL SECURITY;
CREATE POLICY user_role_assignments_tenant_isolation
  ON tenants.user_role_assignments
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS user_role_assignments_audit_trg ON tenants.user_role_assignments;
CREATE TRIGGER user_role_assignments_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.user_role_assignments
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.user_role_assignments IS
  'Asignación de roles a usuarios. is_granted=false anula los permisos del rol.';

-- ============================================================================
-- 4) PERMISOS ORGANIZACIONALES Y OVERRIDES
-- ============================================================================

CREATE TABLE tenants.department_menu_options (
  id              uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id       uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_id   uuid        NOT NULL,
  menu_option_id  uuid        NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_menu_options_uq
    UNIQUE (tenant_id, department_id, menu_option_id),
  CONSTRAINT department_menu_options_id_tenant_uq
    UNIQUE (id, tenant_id, department_id, menu_option_id),
  CONSTRAINT department_menu_options_department_fk
    FOREIGN KEY (department_id, tenant_id)
    REFERENCES tenants.departments (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT department_menu_options_option_fk
    FOREIGN KEY (menu_option_id, tenant_id)
    REFERENCES tenants.menu_options (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_menu_options_tenant_department_idx
  ON tenants.department_menu_options (tenant_id, department_id);
CREATE INDEX department_menu_options_tenant_option_idx
  ON tenants.department_menu_options (tenant_id, menu_option_id);

ALTER TABLE tenants.department_menu_options ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_menu_options FORCE ROW LEVEL SECURITY;
CREATE POLICY department_menu_options_tenant_isolation
  ON tenants.department_menu_options
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_menu_options_audit_trg ON tenants.department_menu_options;
CREATE TRIGGER department_menu_options_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_menu_options
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.department_menu_option_validities (
  id                         uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id                  uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  department_menu_option_id  uuid        NOT NULL,
  department_id              uuid        NOT NULL,
  menu_option_id             uuid        NOT NULL,
  start_date                 timestamptz NOT NULL,
  end_date                   timestamptz,
  created_at                 timestamptz NOT NULL DEFAULT now(),
  updated_at                 timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT department_menu_option_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT department_menu_option_validities_parent_fk
    FOREIGN KEY (department_menu_option_id, tenant_id, department_id, menu_option_id)
    REFERENCES tenants.department_menu_options (id, tenant_id, department_id, menu_option_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX department_menu_option_validities_tenant_department_idx
  ON tenants.department_menu_option_validities (tenant_id, department_id);
CREATE INDEX department_menu_option_validities_tenant_option_idx
  ON tenants.department_menu_option_validities (tenant_id, menu_option_id);

ALTER TABLE tenants.department_menu_option_validities
  ADD CONSTRAINT department_menu_option_validities_no_overlap
  EXCLUDE USING GIST (
    tenant_id WITH =,
    department_id WITH =,
    menu_option_id WITH =,
    tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamptz), '[)') WITH &&
  );

ALTER TABLE tenants.department_menu_option_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.department_menu_option_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY department_menu_option_validities_tenant_isolation
  ON tenants.department_menu_option_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS department_menu_option_validities_audit_trg
  ON tenants.department_menu_option_validities;
CREATE TRIGGER department_menu_option_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.department_menu_option_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.position_menu_options (
  id              uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id       uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  job_id          uuid        NOT NULL,
  menu_option_id  uuid        NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT position_menu_options_uq
    UNIQUE (tenant_id, job_id, menu_option_id),
  CONSTRAINT position_menu_options_id_tenant_uq
    UNIQUE (id, tenant_id, job_id, menu_option_id),
  CONSTRAINT position_menu_options_job_fk
    FOREIGN KEY (job_id, tenant_id)
    REFERENCES tenants.jobs (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT,
  CONSTRAINT position_menu_options_option_fk
    FOREIGN KEY (menu_option_id, tenant_id)
    REFERENCES tenants.menu_options (id, tenant_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX position_menu_options_tenant_job_idx
  ON tenants.position_menu_options (tenant_id, job_id);
CREATE INDEX position_menu_options_tenant_option_idx
  ON tenants.position_menu_options (tenant_id, menu_option_id);

ALTER TABLE tenants.position_menu_options ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.position_menu_options FORCE ROW LEVEL SECURITY;
CREATE POLICY position_menu_options_tenant_isolation
  ON tenants.position_menu_options
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS position_menu_options_audit_trg ON tenants.position_menu_options;
CREATE TRIGGER position_menu_options_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.position_menu_options
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.position_menu_option_validities (
  id                      uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id               uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  position_menu_option_id uuid        NOT NULL,
  job_id                  uuid        NOT NULL,
  menu_option_id          uuid        NOT NULL,
  start_date              timestamptz NOT NULL,
  end_date                timestamptz,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT position_menu_option_validities_dates_chk
    CHECK (end_date IS NULL OR end_date > start_date),
  CONSTRAINT position_menu_option_validities_parent_fk
    FOREIGN KEY (position_menu_option_id, tenant_id, job_id, menu_option_id)
    REFERENCES tenants.position_menu_options (id, tenant_id, job_id, menu_option_id)
    ON UPDATE RESTRICT ON DELETE RESTRICT
);

CREATE INDEX position_menu_option_validities_tenant_job_idx
  ON tenants.position_menu_option_validities (tenant_id, job_id);
CREATE INDEX position_menu_option_validities_tenant_option_idx
  ON tenants.position_menu_option_validities (tenant_id, menu_option_id);

ALTER TABLE tenants.position_menu_option_validities
  ADD CONSTRAINT position_menu_option_validities_no_overlap
  EXCLUDE USING GIST (
    tenant_id WITH =,
    job_id WITH =,
    menu_option_id WITH =,
    tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamptz), '[)') WITH &&
  );

ALTER TABLE tenants.position_menu_option_validities ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.position_menu_option_validities FORCE ROW LEVEL SECURITY;
CREATE POLICY position_menu_option_validities_tenant_isolation
  ON tenants.position_menu_option_validities
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS position_menu_option_validities_audit_trg
  ON tenants.position_menu_option_validities;
CREATE TRIGGER position_menu_option_validities_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.position_menu_option_validities
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

CREATE TABLE tenants.employee_menu_option_overrides (
  id                  uuid        PRIMARY KEY DEFAULT uuidv7(),
  tenant_id           uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  employee_party_id   uuid        NOT NULL REFERENCES tenants.employees (party_id) ON DELETE RESTRICT,
  system_function_id  uuid        NOT NULL REFERENCES global.system_functions (id) ON DELETE RESTRICT,
  is_granted          boolean     NOT NULL DEFAULT true,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT employee_menu_option_overrides_uq
    UNIQUE (tenant_id, employee_party_id, system_function_id)
);

CREATE INDEX employee_menu_option_overrides_tenant_employee_idx
  ON tenants.employee_menu_option_overrides (tenant_id, employee_party_id);
CREATE INDEX employee_menu_option_overrides_tenant_function_idx
  ON tenants.employee_menu_option_overrides (tenant_id, system_function_id);

ALTER TABLE tenants.employee_menu_option_overrides ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.employee_menu_option_overrides FORCE ROW LEVEL SECURITY;
CREATE POLICY employee_menu_option_overrides_tenant_isolation
  ON tenants.employee_menu_option_overrides
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

DROP TRIGGER IF EXISTS employee_menu_option_overrides_audit_trg
  ON tenants.employee_menu_option_overrides;
CREATE TRIGGER employee_menu_option_overrides_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.employee_menu_option_overrides
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.employee_menu_option_overrides IS
  'Concesión o denegación directa de funciones a un empleado; prevalece sobre herencia.';
