-- ============================================================================
-- atlas_post_schema.sql
-- Complemento de esquema para una base creada desde ent://ent/schema con Atlas.
--
-- Atlas crea las 18 tablas de Ent. Este script añade lo que Ent no expresa:
--   * RLS ENABLE + FORCE y políticas fail-closed en todas las tablas tenants.*
--   * checks de integridad y la restricción EXCLUDE de asignaciones.
--   * triggers de auditoría.
--   * collation ICU en columnas de búsqueda.
--   * seed de roles TENANT/ADMIN.
--   * funciones resolve_golden_party y updated_at de sesiones.
--
-- Ejecutar una sola vez sobre una base vacía creada por Atlas.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. RLS en todas las tablas tenant-scoped
-- ----------------------------------------------------------------------------
DO $$
DECLARE
  v_table text;
  v_tables text[] := ARRAY[
    'parties','person_profiles','organization_profiles','party_roles',
    'party_merge_log','sessions','user_credentials','password_reset_tokens',
    'job_families','jobs','departments','employees','employee_assignments',
    'party_contact_mechanisms','addresses'
  ];
BEGIN
  FOREACH v_table IN ARRAY v_tables LOOP
    EXECUTE format('ALTER TABLE tenants.%I ENABLE ROW LEVEL SECURITY', v_table);
    EXECUTE format('ALTER TABLE tenants.%I FORCE ROW LEVEL SECURITY', v_table);
    EXECUTE format('DROP POLICY IF EXISTS %I ON tenants.%I', v_table || '_tenant_isolation', v_table);
    EXECUTE format(
      'CREATE POLICY %I ON tenants.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid)',
      v_table || '_tenant_isolation', v_table
    );
  END LOOP;
END $$;

-- ----------------------------------------------------------------------------
-- 2. Checks de integridad
-- ----------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='contact_mechanisms_type_chk' AND conrelid='global.contact_mechanisms'::regclass) THEN
    ALTER TABLE global.contact_mechanisms ADD CONSTRAINT contact_mechanisms_type_chk CHECK (type IN ('EMAIL','PHONE','MOBILE','FAX','WEBSITE'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='contact_mechanisms_value_chk' AND conrelid='global.contact_mechanisms'::regclass) THEN
    ALTER TABLE global.contact_mechanisms ADD CONSTRAINT contact_mechanisms_value_chk CHECK (btrim(value) <> '');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='parties_record_status_chk' AND conrelid='tenants.parties'::regclass) THEN
    ALTER TABLE tenants.parties ADD CONSTRAINT parties_record_status_chk CHECK (record_status IN ('ACTIVE','MERGED','ARCHIVED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='parties_canonical_not_self_chk' AND conrelid='tenants.parties'::regclass) THEN
    ALTER TABLE tenants.parties ADD CONSTRAINT parties_canonical_not_self_chk CHECK (canonical_party_id IS NULL OR canonical_party_id <> id);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='party_merge_log_distinct_chk' AND conrelid='tenants.party_merge_log'::regclass) THEN
    ALTER TABLE tenants.party_merge_log ADD CONSTRAINT party_merge_log_distinct_chk CHECK (source_party_id <> target_party_id);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_token_hash_sha256_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_token_hash_sha256_chk CHECK (octet_length(token_hash) = 32);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_expires_after_created_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_expires_after_created_chk CHECK (expires_at > created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_revoked_after_created_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_revoked_after_created_chk CHECK (revoked_at IS NULL OR revoked_at >= created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_last_used_after_created_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_last_used_after_created_chk CHECK (last_used_at IS NULL OR last_used_at >= created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_deleted_after_created_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_deleted_after_created_chk CHECK (deleted_at IS NULL OR deleted_at >= created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='sessions_metadata_object_chk' AND conrelid='tenants.sessions'::regclass) THEN
    ALTER TABLE tenants.sessions ADD CONSTRAINT sessions_metadata_object_chk CHECK (jsonb_typeof(metadata) = 'object');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='job_families_name_chk' AND conrelid='tenants.job_families'::regclass) THEN
    ALTER TABLE tenants.job_families ADD CONSTRAINT job_families_name_chk CHECK (btrim(name) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='job_families_code_chk' AND conrelid='tenants.job_families'::regclass) THEN
    ALTER TABLE tenants.job_families ADD CONSTRAINT job_families_code_chk CHECK (btrim(code) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='jobs_title_chk' AND conrelid='tenants.jobs'::regclass) THEN
    ALTER TABLE tenants.jobs ADD CONSTRAINT jobs_title_chk CHECK (btrim(title) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='jobs_code_chk' AND conrelid='tenants.jobs'::regclass) THEN
    ALTER TABLE tenants.jobs ADD CONSTRAINT jobs_code_chk CHECK (btrim(code) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='jobs_min_salary_chk' AND conrelid='tenants.jobs'::regclass) THEN
    ALTER TABLE tenants.jobs ADD CONSTRAINT jobs_min_salary_chk CHECK (min_salary IS NULL OR min_salary >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='jobs_max_salary_chk' AND conrelid='tenants.jobs'::regclass) THEN
    ALTER TABLE tenants.jobs ADD CONSTRAINT jobs_max_salary_chk CHECK (max_salary IS NULL OR max_salary >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='jobs_salary_range_chk' AND conrelid='tenants.jobs'::regclass) THEN
    ALTER TABLE tenants.jobs ADD CONSTRAINT jobs_salary_range_chk CHECK (min_salary IS NULL OR max_salary IS NULL OR min_salary <= max_salary);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='departments_name_chk' AND conrelid='tenants.departments'::regclass) THEN
    ALTER TABLE tenants.departments ADD CONSTRAINT departments_name_chk CHECK (btrim(name) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='departments_code_chk' AND conrelid='tenants.departments'::regclass) THEN
    ALTER TABLE tenants.departments ADD CONSTRAINT departments_code_chk CHECK (btrim(code) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='departments_parent_not_self_chk' AND conrelid='tenants.departments'::regclass) THEN
    ALTER TABLE tenants.departments ADD CONSTRAINT departments_parent_not_self_chk CHECK (parent_department_id IS NULL OR parent_department_id <> id);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employees_code_chk' AND conrelid='tenants.employees'::regclass) THEN
    ALTER TABLE tenants.employees ADD CONSTRAINT employees_code_chk CHECK (btrim(employee_code) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employees_status_chk' AND conrelid='tenants.employees'::regclass) THEN
    ALTER TABLE tenants.employees ADD CONSTRAINT employees_status_chk CHECK (status IN ('ACTIVE','ON_LEAVE','TERMINATED'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employees_not_own_manager_chk' AND conrelid='tenants.employees'::regclass) THEN
    ALTER TABLE tenants.employees ADD CONSTRAINT employees_not_own_manager_chk CHECK (manager_party_id IS NULL OR manager_party_id <> party_id);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employee_assignments_dates_chk' AND conrelid='tenants.employee_assignments'::regclass) THEN
    ALTER TABLE tenants.employee_assignments ADD CONSTRAINT employee_assignments_dates_chk CHECK (end_date IS NULL OR end_date > start_date);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employee_assignments_salary_chk' AND conrelid='tenants.employee_assignments'::regclass) THEN
    ALTER TABLE tenants.employee_assignments ADD CONSTRAINT employee_assignments_salary_chk CHECK (salary >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='employee_assignments_no_overlap' AND conrelid='tenants.employee_assignments'::regclass) THEN
    ALTER TABLE tenants.employee_assignments ADD CONSTRAINT employee_assignments_no_overlap
      EXCLUDE USING GIST (
        tenant_id WITH =,
        employee_party_id WITH =,
        tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamptz), '[)') WITH &&
      );
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='party_contact_mechanisms_order_chk' AND conrelid='tenants.party_contact_mechanisms'::regclass) THEN
    ALTER TABLE tenants.party_contact_mechanisms ADD CONSTRAINT party_contact_mechanisms_order_chk CHECK (contact_mechanism_order >= 0);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='party_contact_mechanisms_dates_chk' AND conrelid='tenants.party_contact_mechanisms'::regclass) THEN
    ALTER TABLE tenants.party_contact_mechanisms ADD CONSTRAINT party_contact_mechanisms_dates_chk CHECK (end_date IS NULL OR end_date > start_date);
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='addresses_type_chk' AND conrelid='tenants.addresses'::regclass) THEN
    ALTER TABLE tenants.addresses ADD CONSTRAINT addresses_type_chk CHECK (type IN ('LEGAL','FISCAL','POSTAL','DELIVERY'));
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='addresses_line1_chk' AND conrelid='tenants.addresses'::regclass) THEN
    ALTER TABLE tenants.addresses ADD CONSTRAINT addresses_line1_chk CHECK (btrim(line1) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='addresses_city_chk' AND conrelid='tenants.addresses'::regclass) THEN
    ALTER TABLE tenants.addresses ADD CONSTRAINT addresses_city_chk CHECK (btrim(city) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='addresses_country_chk' AND conrelid='tenants.addresses'::regclass) THEN
    ALTER TABLE tenants.addresses ADD CONSTRAINT addresses_country_chk CHECK (country ~ '^[A-Z]{2}$');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='user_credentials_password_chk' AND conrelid='tenants.user_credentials'::regclass) THEN
    ALTER TABLE tenants.user_credentials ADD CONSTRAINT user_credentials_password_chk CHECK (btrim(password_hash) <> '');
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='user_credentials_backup_codes_chk' AND conrelid='tenants.user_credentials'::regclass) THEN
    ALTER TABLE tenants.user_credentials ADD CONSTRAINT user_credentials_backup_codes_chk CHECK (jsonb_typeof(backup_codes_hash) = 'array');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='password_reset_tokens_hash_chk' AND conrelid='tenants.password_reset_tokens'::regclass) THEN
    ALTER TABLE tenants.password_reset_tokens ADD CONSTRAINT password_reset_tokens_hash_chk CHECK (octet_length(token_hash) = 32);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='password_reset_tokens_expiry_chk' AND conrelid='tenants.password_reset_tokens'::regclass) THEN
    ALTER TABLE tenants.password_reset_tokens ADD CONSTRAINT password_reset_tokens_expiry_chk CHECK (expires_at > created_at);
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='password_reset_tokens_used_chk' AND conrelid='tenants.password_reset_tokens'::regclass) THEN
    ALTER TABLE tenants.password_reset_tokens ADD CONSTRAINT password_reset_tokens_used_chk CHECK (used_at IS NULL OR used_at >= created_at);
  END IF;
END $$;

-- ----------------------------------------------------------------------------
-- 3. Índices adicionales
-- ----------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS party_merge_log_tenant_merged_at_idx ON tenants.party_merge_log (tenant_id, merged_at);
CREATE INDEX IF NOT EXISTS party_merge_log_source_idx ON tenants.party_merge_log (tenant_id, source_party_id);
CREATE INDEX IF NOT EXISTS party_merge_log_target_idx ON tenants.party_merge_log (tenant_id, target_party_id);
CREATE INDEX IF NOT EXISTS sessions_tenant_idx ON tenants.sessions (tenant_id);
CREATE INDEX IF NOT EXISTS sessions_active_user_idx ON tenants.sessions (tenant_id, user_id) WHERE revoked_at IS NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON tenants.sessions (tenant_id, expires_at) WHERE revoked_at IS NULL AND deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- 4. Triggers de updated_at de sesiones y auditoría
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION tenants.fn_sessions_set_updated_at()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS sessions_set_updated_at_trg ON tenants.sessions;
CREATE TRIGGER sessions_set_updated_at_trg
  BEFORE UPDATE ON tenants.sessions
  FOR EACH ROW EXECUTE FUNCTION tenants.fn_sessions_set_updated_at();

DO $$
DECLARE
  v_table text;
  v_tables text[] := ARRAY[
    'parties','person_profiles','organization_profiles','party_roles',
    'job_families','jobs','departments','employees','employee_assignments',
    'party_contact_mechanisms','addresses'
  ];
BEGIN
  FOREACH v_table IN ARRAY v_tables LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS %I ON tenants.%I', v_table || '_audit_trg', v_table);
    EXECUTE format(
      'CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON tenants.%I FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change()',
      v_table || '_audit_trg', v_table
    );
  END LOOP;
END $$;

DROP TRIGGER IF EXISTS user_credentials_audit_trg ON tenants.user_credentials;
CREATE TRIGGER user_credentials_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.user_credentials
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change('password_hash','totp_secret','backup_codes_hash');

DROP TRIGGER IF EXISTS password_reset_tokens_audit_trg ON tenants.password_reset_tokens;
CREATE TRIGGER password_reset_tokens_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.password_reset_tokens
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change('token_hash');

-- ----------------------------------------------------------------------------
-- 5. Golden Record: función y política de merge log
-- ----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION tenants.resolve_golden_party(p_party_id uuid)
RETURNS uuid
LANGUAGE sql
STABLE
SET search_path = pg_catalog, tenants
AS $$
  WITH RECURSIVE chain AS (
    SELECT p.id, p.canonical_party_id, 0 AS depth
      FROM tenants.parties p
     WHERE p.id = p_party_id
    UNION ALL
    SELECT p.id, p.canonical_party_id, c.depth + 1
      FROM tenants.parties p
      JOIN chain c ON p.id = c.canonical_party_id
     WHERE c.depth < 100
  )
  SELECT id FROM chain WHERE canonical_party_id IS NULL ORDER BY depth DESC LIMIT 1;
$$;

-- ----------------------------------------------------------------------------
-- 6. Collation ICU y unicidad case-insensitive del contacto global
-- ----------------------------------------------------------------------------
DROP INDEX IF EXISTS tenants.user_credentials_party_contact_mechanism_id_key;
DROP INDEX IF EXISTS tenants.user_credentials_party_id_key;
DROP INDEX IF EXISTS global.contactmechanism_type_value;

ALTER TABLE global.contact_mechanisms
  ALTER COLUMN value TYPE text COLLATE "unaccent_case_insensitive";
ALTER TABLE tenants.job_families
  ALTER COLUMN name TYPE text COLLATE "unaccent_case_insensitive";
ALTER TABLE tenants.jobs
  ALTER COLUMN title TYPE text COLLATE "unaccent_case_insensitive";
ALTER TABLE tenants.departments
  ALTER COLUMN name TYPE text COLLATE "unaccent_case_insensitive";
ALTER TABLE tenants.addresses
  ALTER COLUMN line1 TYPE text COLLATE "unaccent_case_insensitive",
  ALTER COLUMN line2 TYPE text COLLATE "unaccent_case_insensitive",
  ALTER COLUMN city TYPE text COLLATE "unaccent_case_insensitive",
  ALTER COLUMN state TYPE text COLLATE "unaccent_case_insensitive",
  ALTER COLUMN postal_code TYPE text COLLATE "unaccent_case_insensitive";

DROP INDEX IF EXISTS global.contact_mechanisms_type_value_uq;
CREATE UNIQUE INDEX contact_mechanisms_type_value_uq
  ON global.contact_mechanisms (type, value);

-- ----------------------------------------------------------------------------
-- 7. Roles de sistema
-- ----------------------------------------------------------------------------
INSERT INTO global.role_types (id, code, name, description, is_system, created_at, updated_at)
VALUES
  (uuidv7(), 'TENANT', 'Tenant', 'Organization that owns the tenant', true, now(), now()),
  (uuidv7(), 'ADMIN',  'Tenant Administrator', 'First administrator of the tenant', true, now(), now())
ON CONFLICT DO NOTHING;
