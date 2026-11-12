-- ============================================================================
-- 0010_attached_documents.sql
-- Documentos adjuntos con almacenamiento híbrido (CAS + metadatos relacionales).
--
--   * El contenido vive fuera de PostgreSQL. storage_key es el SHA-256
--     hexadecimal del archivo y también su nombre/ruta en el almacenamiento.
--   * document_hash (bytea) es la clave de particionado HASH (64 particiones)
--     y permite deduplicar por tenant sin leer el archivo.
--   * origin_table/origin_id son una referencia polimórfica: no llevan FK.
--   * RLS + trigger de auditoría.
--
-- Prerrequisitos: PostgreSQL 18+ (uuidv7), global.subscriptions y
-- audit.fn_log_change().
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
  IF to_regprocedure('audit.fn_log_change()') IS NULL THEN
    v_missing := array_append(v_missing, 'audit.fn_log_change()');
  END IF;
  IF to_regclass('tenants.attached_documents') IS NOT NULL THEN
    RAISE EXCEPTION
      'tenants.attached_documents ya existe; esta migración no es reejecutable';
  END IF;

  IF cardinality(v_missing) > 0 THEN
    RAISE EXCEPTION
      'No se pueden crear attached_documents; faltan prerequisitos: %',
      array_to_string(v_missing, ', ')
      USING HINT = 'Aplica las migraciones anteriores y verifica el esquema audit.';
  END IF;
END
$$;

-- ----------------------------------------------------------------------------
-- Tabla particionada por HASH del contenido
-- ----------------------------------------------------------------------------
CREATE TABLE tenants.attached_documents (
  id               uuid        NOT NULL DEFAULT uuidv7(),
  tenant_id        uuid        NOT NULL REFERENCES global.subscriptions (id) ON DELETE RESTRICT,
  origin_table     text        NOT NULL,
  origin_id        uuid        NOT NULL,
  original_name    text        NOT NULL,
  storage_key      text        NOT NULL,
  storage_provider text        NOT NULL,
  mime_type        text        NOT NULL,
  size_bytes       bigint      NOT NULL,
  document_hash    bytea       NOT NULL,
  description      text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT attached_documents_pkey PRIMARY KEY (id, document_hash),

  CONSTRAINT attached_documents_origin_table_chk
    CHECK (btrim(origin_table) <> ''),
  CONSTRAINT attached_documents_original_name_chk
    CHECK (btrim(original_name) <> '' AND original_name !~ '[/\\]'),
  CONSTRAINT attached_documents_storage_key_chk
    CHECK (storage_key ~ '^[0-9a-f]{64}$'),
  CONSTRAINT attached_documents_storage_provider_chk
    CHECK (btrim(storage_provider) <> ''),
  CONSTRAINT attached_documents_mime_type_chk
    CHECK (btrim(mime_type) <> ''),
  CONSTRAINT attached_documents_size_chk
    CHECK (size_bytes >= 0),
  CONSTRAINT attached_documents_hash_chk
    CHECK (octet_length(document_hash) = 32)
) PARTITION BY HASH (document_hash);

-- 64 particiones: p00 .. p63.
DO $$
DECLARE
  i integer;
BEGIN
  FOR i IN 0..63 LOOP
    EXECUTE format(
      'CREATE TABLE tenants.attached_documents_p%s PARTITION OF tenants.attached_documents FOR VALUES WITH (MODULUS 64, REMAINDER %s)',
      lpad(i::text, 2, '0'),
      i
    );
  END LOOP;
END
$$;

-- ----------------------------------------------------------------------------
-- Índices (comienzan por tenant_id por la política RLS)
-- ----------------------------------------------------------------------------
CREATE INDEX attached_documents_tenant_origin_idx
  ON tenants.attached_documents (tenant_id, origin_table, origin_id)
  WHERE deleted_at IS NULL;
CREATE INDEX attached_documents_tenant_hash_idx
  ON tenants.attached_documents (tenant_id, document_hash)
  WHERE deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- RLS
-- ----------------------------------------------------------------------------
ALTER TABLE tenants.attached_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants.attached_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY attached_documents_tenant_isolation
  ON tenants.attached_documents
  FOR ALL
  USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid)
  WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::uuid);

-- Las particiones se pueden consultar directamente; se protegen igual que la
-- tabla padre para que ninguna ruta de acceso evada la RLS.
DO $$
DECLARE
  i integer;
  partition_name text;
BEGIN
  FOR i IN 0..63 LOOP
    partition_name := 'attached_documents_p' || lpad(i::text, 2, '0');
    EXECUTE format('ALTER TABLE tenants.%I ENABLE ROW LEVEL SECURITY', partition_name);
    EXECUTE format('ALTER TABLE tenants.%I FORCE ROW LEVEL SECURITY', partition_name);
    EXECUTE format(
      'CREATE POLICY %I ON tenants.%I FOR ALL USING (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid) WITH CHECK (tenant_id = NULLIF(current_setting(''app.current_tenant_id'', true), '''')::uuid)',
      partition_name || '_tenant_isolation',
      partition_name
    );
  END LOOP;
END
$$;

-- ----------------------------------------------------------------------------
-- Auditoría (el trigger de la tabla padre se propaga a las particiones)
-- ----------------------------------------------------------------------------
DROP TRIGGER IF EXISTS attached_documents_audit_trg ON tenants.attached_documents;
CREATE TRIGGER attached_documents_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.attached_documents
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();

COMMENT ON TABLE tenants.attached_documents IS
  'Metadatos de documentos; el contenido vive fuera de la base de datos (CAS por SHA-256).';
COMMENT ON COLUMN tenants.attached_documents.storage_key IS
  'SHA-256 hexadecimal del archivo; nombre/ruta en el almacenamiento.';
COMMENT ON COLUMN tenants.attached_documents.document_hash IS
  'SHA-256 binario; clave de particionado HASH y deduplicación por tenant.';
COMMENT ON COLUMN tenants.attached_documents.origin_table IS
  'Tabla de negocio a la que pertenece el documento (referencia polimórfica, sin FK).';
COMMENT ON COLUMN tenants.attached_documents.origin_id IS
  'ID de la fila de origin_table (referencia polimórfica, sin FK).';
