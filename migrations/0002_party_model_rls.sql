-- ============================================================================
-- 0002_party_model_rls.sql
-- Row Level Security for the tenant-scoped Party Model tables.
--
-- Prerequisites:
--   * Tables created from ent/schema using Atlas (the `Schema` annotation on
--     each Ent schema is honored by Atlas; Ent's default migrator is not
--     schema-aware).
--   * global.subscriptions and global.role_types exist.
--   * tenants.parties, tenants.person_profiles, tenants.organization_profiles
--     and tenants.party_roles exist.
--
-- Execute atomically:
--   psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 --single-transaction \
--        -f migrations/0002_party_model_rls.sql
-- ============================================================================

DO $$
DECLARE
  v_missing text[] := ARRAY[]::text[];
BEGIN
  IF to_regclass('global.subscriptions') IS NULL THEN
    v_missing := array_append(v_missing, 'global.subscriptions');
  END IF;
  IF to_regclass('global.role_types') IS NULL THEN
    v_missing := array_append(v_missing, 'global.role_types');
  END IF;
  IF to_regclass('tenants.parties') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.parties');
  END IF;
  IF to_regclass('tenants.person_profiles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.person_profiles');
  END IF;
  IF to_regclass('tenants.organization_profiles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.organization_profiles');
  END IF;
  IF to_regclass('tenants.party_roles') IS NULL THEN
    v_missing := array_append(v_missing, 'tenants.party_roles');
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se puede aplicar RLS; faltan tablas: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica primero las migraciones de Ent con Atlas (schemas global y tenants).';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- tenants.parties
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.parties ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.parties FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS parties_tenant_isolation ON tenants.parties;
CREATE POLICY parties_tenant_isolation
  ON tenants.parties
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY parties_tenant_isolation ON tenants.parties IS
  'Aislamiento fail-closed por app.current_tenant_id.';

-- ----------------------------------------------------------------------------
-- tenants.person_profiles
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.person_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.person_profiles FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS person_profiles_tenant_isolation ON tenants.person_profiles;
CREATE POLICY person_profiles_tenant_isolation
  ON tenants.person_profiles
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY person_profiles_tenant_isolation ON tenants.person_profiles IS
  'Aislamiento fail-closed por app.current_tenant_id.';

-- ----------------------------------------------------------------------------
-- tenants.organization_profiles
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.organization_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.organization_profiles FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS organization_profiles_tenant_isolation ON tenants.organization_profiles;
CREATE POLICY organization_profiles_tenant_isolation
  ON tenants.organization_profiles
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY organization_profiles_tenant_isolation ON tenants.organization_profiles IS
  'Aislamiento fail-closed por app.current_tenant_id.';

-- ----------------------------------------------------------------------------
-- tenants.party_roles
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.party_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.party_roles FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS party_roles_tenant_isolation ON tenants.party_roles;
CREATE POLICY party_roles_tenant_isolation
  ON tenants.party_roles
  FOR ALL
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  )
  WITH CHECK (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid
  );

COMMENT ON POLICY party_roles_tenant_isolation ON tenants.party_roles IS
  'Aislamiento fail-closed por app.current_tenant_id.';
