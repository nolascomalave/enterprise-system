--
-- PostgreSQL database dump
--

\restrict 9bAq2HaXtKr8wu083tKmht0kwHDUshNlZGMhtg6nCbZ43HkB9eQqA6iyVOlYjsd

-- Dumped from database version 18.3 (Homebrew)
-- Dumped by pg_dump version 18.3 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: audit; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA audit;


--
-- Name: global; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA global;


--
-- Name: tenants; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA tenants;


--
-- Name: fn_deny_modification(); Type: FUNCTION; Schema: audit; Owner: -
--

CREATE FUNCTION audit.fn_deny_modification() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path TO 'pg_catalog'
    AS $$
BEGIN
  RAISE EXCEPTION 'audit.change_log es append-only: % no permitido', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;


--
-- Name: FUNCTION fn_deny_modification(); Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON FUNCTION audit.fn_deny_modification() IS 'Bloquea UPDATE/DELETE/TRUNCATE sobre audit.change_log.';


--
-- Name: fn_log_change(); Type: FUNCTION; Schema: audit; Owner: -
--

CREATE FUNCTION audit.fn_log_change() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'audit'
    AS $$
DECLARE
  v_old        jsonb;
  v_new        jsonb;
  v_changed    jsonb := '{}'::jsonb;
  v_operation  text;
  v_changed_by uuid;
  v_tenant_id  uuid;
  v_session_id uuid;
  v_record_id  text;
  v_key        text;
BEGIN
  v_changed_by := NULLIF(current_setting('app.current_user_id', true), '')::uuid;
  v_tenant_id  := NULLIF(current_setting('app.current_tenant_id', true), '')::uuid;
  v_session_id := NULLIF(current_setting('app.current_session_id', true), '')::uuid;

  IF TG_OP = 'INSERT' THEN
    v_new        := to_jsonb(NEW);
    v_operation  := 'INSERT';
    v_changed    := v_new;
  ELSIF TG_OP = 'DELETE' THEN
    v_old        := to_jsonb(OLD);
    v_operation  := 'DELETE';
    v_changed    := v_old;
  ELSE
    v_old := to_jsonb(OLD);
    v_new := to_jsonb(NEW);

    IF (v_old->>'deleted_at') IS NULL AND (v_new->>'deleted_at') IS NOT NULL THEN
      v_operation := 'SOFT_DELETE';
    ELSE
      v_operation := 'UPDATE';
    END IF;

    SELECT COALESCE(jsonb_object_agg(COALESCE(n.key, o.key), n.value), '{}'::jsonb)
      INTO v_changed
      FROM jsonb_each(v_new) AS n
      FULL OUTER JOIN jsonb_each(v_old) AS o ON o.key = n.key
     WHERE n.value IS DISTINCT FROM o.value;

  END IF;

  -- La mayoría de las tablas usan `id`; employees usa `party_id` como PK.
  v_record_id := COALESCE(
    (COALESCE(v_new, v_old))->>'id',
    (COALESCE(v_new, v_old))->>'party_id'
  );

  -- Sensitive columns can be redacted per trigger via TG_ARGV, e.g.
  -- audit.fn_log_change('password_hash', 'totp_secret').
  IF TG_NARGS > 0 THEN
    FOREACH v_key IN ARRAY TG_ARGV LOOP
      IF v_old IS NOT NULL THEN
        v_old := v_old - v_key;
      END IF;
      IF v_new IS NOT NULL THEN
        v_new := v_new - v_key;
      END IF;
      v_changed := v_changed - v_key;
    END LOOP;
  END IF;

  IF v_tenant_id IS NULL THEN
    v_tenant_id := (COALESCE(v_new, v_old)->>'tenant_id')::uuid;
  END IF;

  INSERT INTO audit.change_log (
    table_name,
    record_id,
    operation,
    old_data,
    new_data,
    changed_fields,
    changed_by,
    tenant_id,
    session_id
  ) VALUES (
    TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME,
    v_record_id,
    v_operation,
    v_old,
    v_new,
    v_changed,
    v_changed_by,
    v_tenant_id,
    v_session_id
  );

  RETURN NULL;
END;
$$;


--
-- Name: FUNCTION fn_log_change(); Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON FUNCTION audit.fn_log_change() IS 'Registra INSERT/UPDATE/SOFT_DELETE/DELETE en audit.change_log y calcula changed_fields.';


--
-- Name: fn_log_tenant_relationship_invite(); Type: FUNCTION; Schema: audit; Owner: -
--

CREATE FUNCTION audit.fn_log_tenant_relationship_invite() RETURNS trigger
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path TO 'pg_catalog', 'audit', 'global'
    AS $$
DECLARE
  v_old         jsonb;
  v_new         jsonb;
  v_changed     jsonb := '{}'::jsonb;
  v_operation   text;
  v_changed_by  uuid;
  v_tenant_id   uuid;
  v_session_id  uuid;
  v_record_id   text;
  v_key         text;
  v_relationship_id uuid;
BEGIN
  v_changed_by := NULLIF(current_setting('app.current_user_id', true), '')::uuid;
  v_session_id := NULLIF(current_setting('app.current_session_id', true), '')::uuid;

  IF TG_OP = 'INSERT' THEN
    v_new := to_jsonb(NEW);
    v_operation := 'INSERT';
    v_changed := v_new;
  ELSIF TG_OP = 'DELETE' THEN
    v_old := to_jsonb(OLD);
    v_operation := 'DELETE';
    v_changed := v_old;
  ELSE
    v_old := to_jsonb(OLD);
    v_new := to_jsonb(NEW);
    v_operation := 'UPDATE';
    SELECT COALESCE(jsonb_object_agg(COALESCE(n.key, o.key), n.value), '{}'::jsonb)
      INTO v_changed
      FROM jsonb_each(v_new) AS n
      FULL OUTER JOIN jsonb_each(v_old) AS o ON o.key = n.key
     WHERE n.value IS DISTINCT FROM o.value;
  END IF;

  v_record_id := COALESCE(v_new, v_old)->>'id';
  v_relationship_id := (COALESCE(v_new, v_old)->>'relationship_id')::uuid;

  -- El histórico hereda el tenant holding de la relación.
  IF v_relationship_id IS NOT NULL THEN
    SELECT r.tenant_id_holding INTO v_tenant_id
      FROM global.tenant_relationships r
     WHERE r.id = v_relationship_id;
  END IF;

  IF v_tenant_id IS NULL THEN
    v_tenant_id := NULLIF(current_setting('app.current_tenant_id', true), '')::uuid;
  END IF;

  -- Redacción de token_hash: nunca se audita el hash completo.
  IF TG_NARGS > 0 THEN
    FOREACH v_key IN ARRAY TG_ARGV LOOP
      IF v_old IS NOT NULL THEN
        v_old := v_old - v_key;
      END IF;
      IF v_new IS NOT NULL THEN
        v_new := v_new - v_key;
      END IF;
      v_changed := v_changed - v_key;
    END LOOP;
  END IF;

  INSERT INTO audit.change_log (
    table_name, record_id, operation, old_data, new_data,
    changed_fields, changed_by, tenant_id, session_id
  ) VALUES (
    TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME,
    v_record_id,
    v_operation,
    v_old,
    v_new,
    v_changed,
    v_changed_by,
    v_tenant_id,
    v_session_id
  );

  RETURN NULL;
END;
$$;


--
-- Name: fn_sessions_set_updated_at(); Type: FUNCTION; Schema: tenants; Owner: -
--

CREATE FUNCTION tenants.fn_sessions_set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;


--
-- Name: resolve_golden_party(uuid); Type: FUNCTION; Schema: tenants; Owner: -
--

CREATE FUNCTION tenants.resolve_golden_party(p_party_id uuid) RETURNS uuid
    LANGUAGE sql STABLE
    SET search_path TO 'pg_catalog', 'tenants'
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


SET default_tablespace = '';

--
-- Name: change_log; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log (
    id uuid DEFAULT uuidv7() NOT NULL,
    table_name text NOT NULL,
    record_id text NOT NULL,
    operation text NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
)
PARTITION BY RANGE (changed_at);


--
-- Name: TABLE change_log; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON TABLE audit.change_log IS 'Auditoría central append-only. Particionada mensualmente por changed_at; sin FKs a usuarios ni sesiones.';


--
-- Name: COLUMN change_log.id; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.id IS 'UUIDv7 del evento de auditoría; la PK es compuesta porque changed_at es la clave de partición.';


--
-- Name: COLUMN change_log.table_name; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.table_name IS 'Tabla auditada en formato schema.table.';


--
-- Name: COLUMN change_log.record_id; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.record_id IS 'Valor textual del id de la fila auditada; no es FK.';


--
-- Name: COLUMN change_log.operation; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.operation IS 'INSERT, UPDATE, SOFT_DELETE o DELETE.';


--
-- Name: COLUMN change_log.old_data; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.old_data IS 'Fila completa antes del cambio (NULL en INSERT).';


--
-- Name: COLUMN change_log.new_data; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.new_data IS 'Fila completa después del cambio (NULL en DELETE).';


--
-- Name: COLUMN change_log.changed_fields; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.changed_fields IS 'Solo los campos que cambiaron (o la fila completa en INSERT/DELETE).';


--
-- Name: COLUMN change_log.changed_by; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.changed_by IS 'Valor de app.current_user_id; sin FK para preservar el historial.';


--
-- Name: COLUMN change_log.tenant_id; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.tenant_id IS 'Valor de app.current_tenant_id (fallback: tenant_id de la fila); sin FK.';


--
-- Name: COLUMN change_log.session_id; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.session_id IS 'Valor de app.current_session_id; sin FK para preservar el historial.';


--
-- Name: COLUMN change_log.changed_at; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON COLUMN audit.change_log.changed_at IS 'Momento exacto del cambio; clock_timestamp() para no depender del inicio de la transacción.';


SET default_table_access_method = heap;

--
-- Name: change_log_2026_11; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2026_11 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2026_12; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2026_12 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_01; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_01 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_02; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_02 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_03; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_03 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_04; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_04 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_05; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_05 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_06; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_06 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_07; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_07 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_08; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_08 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_09; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_09 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_2027_10; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_2027_10 (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: change_log_default; Type: TABLE; Schema: audit; Owner: -
--

CREATE TABLE audit.change_log_default (
    id uuid DEFAULT uuidv7() CONSTRAINT change_log_id_not_null NOT NULL,
    table_name text CONSTRAINT change_log_table_name_not_null NOT NULL,
    record_id text CONSTRAINT change_log_record_id_not_null NOT NULL,
    operation text CONSTRAINT change_log_operation_not_null NOT NULL,
    old_data jsonb,
    new_data jsonb,
    changed_fields jsonb DEFAULT '{}'::jsonb CONSTRAINT change_log_changed_fields_not_null NOT NULL,
    changed_by uuid,
    tenant_id uuid,
    session_id uuid,
    changed_at timestamp with time zone DEFAULT clock_timestamp() CONSTRAINT change_log_changed_at_not_null NOT NULL,
    CONSTRAINT change_log_changed_fields_object_chk CHECK ((jsonb_typeof(changed_fields) = 'object'::text)),
    CONSTRAINT change_log_new_data_object_chk CHECK (((new_data IS NULL) OR (jsonb_typeof(new_data) = 'object'::text))),
    CONSTRAINT change_log_old_data_object_chk CHECK (((old_data IS NULL) OR (jsonb_typeof(old_data) = 'object'::text))),
    CONSTRAINT change_log_operation_chk CHECK ((operation = ANY (ARRAY['INSERT'::text, 'UPDATE'::text, 'SOFT_DELETE'::text, 'DELETE'::text]))),
    CONSTRAINT change_log_record_id_chk CHECK ((btrim(record_id) <> ''::text)),
    CONSTRAINT change_log_table_name_chk CHECK ((btrim(table_name) <> ''::text))
);


--
-- Name: client_applications; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.client_applications (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid,
    name text NOT NULL,
    client_id text NOT NULL,
    client_secret_hash text NOT NULL,
    scopes text[] DEFAULT '{}'::text[] NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    client_type text DEFAULT 'THIRD_PARTY'::text NOT NULL,
    application_scope text DEFAULT 'TENANT_SPECIFIC'::text NOT NULL,
    CONSTRAINT client_applications_application_scope_chk CHECK ((application_scope = ANY (ARRAY['PLATFORM'::text, 'TENANT_SPECIFIC'::text]))),
    CONSTRAINT client_applications_client_id_chk CHECK ((btrim(client_id) <> ''::text)),
    CONSTRAINT client_applications_client_type_chk CHECK ((client_type = ANY (ARRAY['FIRST_PARTY'::text, 'THIRD_PARTY'::text]))),
    CONSTRAINT client_applications_name_chk CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT client_applications_scope_tenant_chk CHECK ((((application_scope = 'TENANT_SPECIFIC'::text) AND (tenant_id IS NOT NULL)) OR ((application_scope = 'PLATFORM'::text) AND (tenant_id IS NULL) AND (client_type = 'FIRST_PARTY'::text)))),
    CONSTRAINT client_applications_scopes_chk CHECK ((cardinality(scopes) > 0)),
    CONSTRAINT client_applications_secret_chk CHECK ((btrim(client_secret_hash) <> ''::text))
);


--
-- Name: TABLE client_applications; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.client_applications IS 'Aplicaciones cliente OAuth2 M2M. Sin RLS: el token endpoint busca por client_id.';


--
-- Name: COLUMN client_applications.client_secret_hash; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.client_applications.client_secret_hash IS 'Hash bcrypt del secreto; el valor plano nunca se persiste.';


--
-- Name: COLUMN client_applications.scopes; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.client_applications.scopes IS 'Scopes concedidos, p. ej. parties:read, org:write, documents:write.';


--
-- Name: COLUMN client_applications.client_type; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.client_applications.client_type IS 'FIRST_PARTY para apps oficiales; THIRD_PARTY para integraciones de clientes.';


--
-- Name: COLUMN client_applications.application_scope; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.client_applications.application_scope IS 'PLATFORM para apps globales sin tenant; TENANT_SPECIFIC para apps de un tenant.';


--
-- Name: contact_mechanisms; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.contact_mechanisms (
    id uuid DEFAULT uuidv7() NOT NULL,
    type character varying NOT NULL,
    value text NOT NULL COLLATE public.unaccent_case_insensitive,
    CONSTRAINT contact_mechanisms_type_chk CHECK (((type)::text = ANY ((ARRAY['EMAIL'::character varying, 'PHONE'::character varying, 'MOBILE'::character varying, 'FAX'::character varying, 'WEBSITE'::character varying])::text[]))),
    CONSTRAINT contact_mechanisms_value_chk CHECK ((btrim(value) <> ''::text))
);


--
-- Name: countries; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.countries (
    code text NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    phone_code text,
    currency_code text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT countries_code_chk CHECK ((code ~ '^[A-Z]{2}$'::text)),
    CONSTRAINT countries_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE countries; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.countries IS 'Catálogo global de países; code es ISO 3166-1 alpha-2. Sin RLS.';


--
-- Name: identity_document_type_translations; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.identity_document_type_translations (
    id uuid DEFAULT uuidv7() NOT NULL,
    identity_document_type_id uuid CONSTRAINT identity_document_type_trans_identity_document_type_id_not_null NOT NULL,
    locale_code character varying(10) NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    description text COLLATE public.unaccent_case_insensitive,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT identity_document_type_translations_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE identity_document_type_translations; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.identity_document_type_translations IS 'Traducciones de tipos de documento; el fallback final es el code.';


--
-- Name: identity_document_types; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.identity_document_types (
    id uuid DEFAULT uuidv7() NOT NULL,
    code text NOT NULL,
    scope text DEFAULT 'PERSON'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT identity_document_types_code_chk CHECK ((code ~ '^[A-Z_]+$'::text)),
    CONSTRAINT identity_document_types_scope_chk CHECK ((scope = ANY (ARRAY['PERSON'::text, 'ORGANIZATION'::text])))
);


--
-- Name: TABLE identity_document_types; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.identity_document_types IS 'Tipos globales de documento de identidad. Sin RLS.';


--
-- Name: locales; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.locales (
    code character varying(10) NOT NULL,
    name text NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT locales_code_chk CHECK ((btrim((code)::text) <> ''::text)),
    CONSTRAINT locales_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE locales; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.locales IS 'Locales BCP 47 soportados. Exactamente uno marcado is_default.';


--
-- Name: location_translations; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.location_translations (
    id uuid DEFAULT uuidv7() NOT NULL,
    location_id uuid NOT NULL,
    language_code text NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT location_translations_language_chk CHECK ((btrim(language_code) <> ''::text)),
    CONSTRAINT location_translations_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE location_translations; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.location_translations IS 'Traducciones por idioma con PK compuesta (location_id, language_code); id surrogate para Ent.';


--
-- Name: COLUMN location_translations.language_code; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.location_translations.language_code IS 'Locale BCP 47; FK a global.locales(code).';


--
-- Name: locations; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.locations (
    id uuid DEFAULT uuidv7() NOT NULL,
    location_type text NOT NULL,
    parent_id uuid,
    country_code text NOT NULL,
    iso_code text,
    external_id text,
    default_name text NOT NULL COLLATE public.unaccent_case_insensitive,
    path public.ltree NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT locations_name_chk CHECK ((btrim(default_name) <> ''::text)),
    CONSTRAINT locations_parent_not_self_chk CHECK (((parent_id IS NULL) OR (parent_id <> id))),
    CONSTRAINT locations_type_chk CHECK ((location_type = ANY (ARRAY['REGION'::text, 'SUBREGION'::text, 'STATE'::text, 'PROVINCE'::text, 'CITY'::text, 'MUNICIPALITY'::text, 'COUNTY'::text])))
);


--
-- Name: TABLE locations; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.locations IS 'Jerarquía geográfica global; path ltree (world.<país>.<...>). Sin RLS.';


--
-- Name: COLUMN locations.path; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.locations.path IS 'Ruta ltree normalizada; permite ancestros con @> y descendientes con <@.';


--
-- Name: menu_template_functions; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.menu_template_functions (
    id uuid DEFAULT uuidv7() NOT NULL,
    template_id uuid NOT NULL,
    system_function_id uuid NOT NULL,
    is_required boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE menu_template_functions; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.menu_template_functions IS 'Funciones requeridas u opcionales incluidas en una plantilla de menú.';


--
-- Name: menu_templates; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.menu_templates (
    id uuid DEFAULT uuidv7() NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    client_type text NOT NULL,
    application_scope text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT menu_templates_application_scope_chk CHECK ((application_scope = ANY (ARRAY['PLATFORM'::text, 'TENANT_SPECIFIC'::text]))),
    CONSTRAINT menu_templates_client_type_chk CHECK ((client_type = ANY (ARRAY['FIRST_PARTY'::text, 'THIRD_PARTY'::text]))),
    CONSTRAINT menu_templates_code_chk CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT menu_templates_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE menu_templates; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.menu_templates IS 'Plantillas de menú compartidas para diferentes tipos de aplicación cliente.';


--
-- Name: name_types; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.name_types (
    id uuid DEFAULT uuidv7() NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    description text,
    is_system boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT name_types_code_chk CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT name_types_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE name_types; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.name_types IS 'Catálogo global de tipos de nombre. Sin tenant_id ni RLS.';


--
-- Name: role_type_translations; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.role_type_translations (
    id uuid DEFAULT uuidv7() NOT NULL,
    role_type_id uuid NOT NULL,
    locale_code character varying(10) NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    description text COLLATE public.unaccent_case_insensitive,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT role_type_translations_name_chk CHECK ((btrim(name) <> ''::text))
);


--
-- Name: TABLE role_type_translations; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.role_type_translations IS 'Traducciones de tipos de rol; el fallback final es el code.';


--
-- Name: role_types; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.role_types (
    id uuid DEFAULT uuidv7() NOT NULL,
    code character varying NOT NULL,
    is_system boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: subscriptions; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.subscriptions (
    id uuid DEFAULT uuidv7() NOT NULL,
    name character varying NOT NULL,
    slug character varying NOT NULL,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: system_functions; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.system_functions (
    id uuid DEFAULT uuidv7() NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    description text,
    function_scope text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT system_functions_code_chk CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT system_functions_name_chk CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT system_functions_scope_chk CHECK ((function_scope = ANY (ARRAY['PLATFORM'::text, 'TENANT'::text])))
);


--
-- Name: TABLE system_functions; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.system_functions IS 'Catálogo de funciones de negocio que el RBAC puede conceder o exigir.';


--
-- Name: tenant_relationship_invites; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.tenant_relationship_invites (
    id uuid DEFAULT uuidv7() NOT NULL,
    relationship_id uuid NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    canceled_at timestamp with time zone,
    canceled_by_tenant_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_relationship_invites_cancel_pair_chk CHECK ((((canceled_at IS NULL) AND (canceled_by_tenant_id IS NULL)) OR ((canceled_at IS NOT NULL) AND (canceled_by_tenant_id IS NOT NULL)))),
    CONSTRAINT tenant_relationship_invites_expiry_chk CHECK ((expires_at > created_at))
);

ALTER TABLE ONLY global.tenant_relationship_invites FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE tenant_relationship_invites; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.tenant_relationship_invites IS 'Invitaciones por relación. No lleva tenant_id: el tenant se hereda de tenant_relationships.';


--
-- Name: COLUMN tenant_relationship_invites.token_hash; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.tenant_relationship_invites.token_hash IS 'SHA-256 del código de invitación; nunca se guarda el código en claro.';


--
-- Name: tenant_relationships; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.tenant_relationships (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id_holding uuid NOT NULL,
    tenant_id_subsidiary uuid NOT NULL,
    relationship_type text NOT NULL,
    status text DEFAULT 'PENDING'::text NOT NULL,
    requested_at timestamp with time zone DEFAULT now() NOT NULL,
    approved_at timestamp with time zone,
    rejected_at timestamp with time zone,
    canceled_at timestamp with time zone,
    ends_at timestamp with time zone,
    rejection_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_relationships_distinct_tenants_chk CHECK ((tenant_id_holding <> tenant_id_subsidiary)),
    CONSTRAINT tenant_relationships_status_chk CHECK ((status = ANY (ARRAY['PENDING'::text, 'APPROVED'::text, 'REJECTED'::text, 'CANCELED'::text]))),
    CONSTRAINT tenant_relationships_type_chk CHECK ((btrim(relationship_type) <> ''::text))
);

ALTER TABLE ONLY global.tenant_relationships FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE tenant_relationships; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.tenant_relationships IS 'Relaciones holding/subsidiary. Una sola PENDING por holding, subsidiary y tipo.';


--
-- Name: COLUMN tenant_relationships.relationship_type; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON COLUMN global.tenant_relationships.relationship_type IS 'Tipo de relación (p. ej. OWNERSHIP, MANAGEMENT).';


--
-- Name: tenant_transfers; Type: TABLE; Schema: global; Owner: -
--

CREATE TABLE global.tenant_transfers (
    id uuid DEFAULT uuidv7() NOT NULL,
    relationship_id uuid NOT NULL,
    requested_by_tenant_id uuid NOT NULL,
    status text DEFAULT 'TRANSFER_REQUESTED'::text NOT NULL,
    requested_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    canceled_at timestamp with time zone,
    canceled_by_tenant_id uuid,
    transfer_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT tenant_transfers_cancel_pair_chk CHECK ((((canceled_at IS NULL) AND (canceled_by_tenant_id IS NULL)) OR ((canceled_at IS NOT NULL) AND (canceled_by_tenant_id IS NOT NULL)))),
    CONSTRAINT tenant_transfers_status_chk CHECK ((status = ANY (ARRAY['TRANSFER_REQUESTED'::text, 'TRANSFER_IN_PROGRESS'::text, 'TRANSFER_COMPLETED'::text, 'TRANSFER_FAILED'::text, 'TRANSFER_CANCELED'::text])))
);

ALTER TABLE ONLY global.tenant_transfers FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE tenant_transfers; Type: COMMENT; Schema: global; Owner: -
--

COMMENT ON TABLE global.tenant_transfers IS 'Transferencias de subsidiarias entre holdings; RLS explícito por relación y requested_by_tenant_id.';


--
-- Name: address_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.address_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    address_id uuid NOT NULL,
    type text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT address_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date))),
    CONSTRAINT address_validities_type_chk CHECK ((type = ANY (ARRAY['LEGAL'::text, 'FISCAL'::text, 'POSTAL'::text, 'DELIVERY'::text])))
);

ALTER TABLE ONLY tenants.address_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE address_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.address_validities IS 'Vigencias de direcciones; una primaria abierta por party/tipo.';


--
-- Name: addresses; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.addresses (
    id uuid DEFAULT uuidv7() NOT NULL,
    type character varying NOT NULL,
    line1 text NOT NULL COLLATE public.unaccent_case_insensitive,
    line2 text COLLATE public.unaccent_case_insensitive,
    postal_code text COLLATE public.unaccent_case_insensitive,
    country character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    party_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    city_location_id uuid NOT NULL,
    CONSTRAINT addresses_country_chk CHECK (((country)::text ~ '^[A-Z]{2}$'::text)),
    CONSTRAINT addresses_line1_chk CHECK ((btrim(line1) <> ''::text)),
    CONSTRAINT addresses_type_chk CHECK (((type)::text = ANY ((ARRAY['LEGAL'::character varying, 'FISCAL'::character varying, 'POSTAL'::character varying, 'DELIVERY'::character varying])::text[])))
);

ALTER TABLE ONLY tenants.addresses FORCE ROW LEVEL SECURITY;


--
-- Name: COLUMN addresses.city_location_id; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.addresses.city_location_id IS 'Ciudad/municipio en global.locations; reemplaza city/state de texto.';


--
-- Name: attached_documents; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    origin_table text NOT NULL,
    origin_id uuid NOT NULL,
    original_name text NOT NULL,
    storage_key text NOT NULL,
    storage_provider text NOT NULL,
    mime_type text NOT NULL,
    size_bytes bigint NOT NULL,
    document_hash bytea NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
)
PARTITION BY HASH (document_hash);

ALTER TABLE ONLY tenants.attached_documents FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE attached_documents; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.attached_documents IS 'Metadatos de documentos; el contenido vive fuera de la base de datos (CAS por SHA-256).';


--
-- Name: COLUMN attached_documents.origin_table; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.attached_documents.origin_table IS 'Tabla de negocio a la que pertenece el documento (referencia polimórfica, sin FK).';


--
-- Name: COLUMN attached_documents.origin_id; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.attached_documents.origin_id IS 'ID de la fila de origin_table (referencia polimórfica, sin FK).';


--
-- Name: COLUMN attached_documents.storage_key; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.attached_documents.storage_key IS 'SHA-256 hexadecimal del archivo; nombre/ruta en el almacenamiento.';


--
-- Name: COLUMN attached_documents.document_hash; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.attached_documents.document_hash IS 'SHA-256 binario; clave de particionado HASH y deduplicación por tenant.';


--
-- Name: attached_documents_p00; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p00 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p00 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p01; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p01 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p01 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p02; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p02 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p02 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p03; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p03 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p03 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p04; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p04 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p04 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p05; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p05 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p05 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p06; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p06 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p06 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p07; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p07 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p07 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p08; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p08 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p08 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p09; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p09 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p09 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p10; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p10 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p10 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p11; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p11 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p11 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p12; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p12 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p12 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p13; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p13 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p13 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p14; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p14 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p14 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p15; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p15 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p15 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p16; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p16 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p16 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p17; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p17 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p17 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p18; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p18 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p18 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p19; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p19 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p19 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p20; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p20 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p20 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p21; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p21 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p21 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p22; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p22 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p22 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p23; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p23 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p23 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p24; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p24 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p24 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p25; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p25 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p25 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p26; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p26 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p26 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p27; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p27 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p27 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p28; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p28 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p28 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p29; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p29 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p29 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p30; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p30 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p30 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p31; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p31 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p31 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p32; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p32 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p32 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p33; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p33 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p33 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p34; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p34 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p34 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p35; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p35 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p35 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p36; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p36 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p36 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p37; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p37 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p37 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p38; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p38 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p38 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p39; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p39 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p39 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p40; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p40 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p40 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p41; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p41 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p41 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p42; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p42 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p42 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p43; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p43 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p43 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p44; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p44 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p44 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p45; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p45 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p45 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p46; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p46 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p46 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p47; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p47 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p47 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p48; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p48 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p48 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p49; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p49 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p49 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p50; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p50 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p50 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p51; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p51 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p51 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p52; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p52 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p52 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p53; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p53 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p53 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p54; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p54 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p54 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p55; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p55 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p55 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p56; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p56 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p56 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p57; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p57 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p57 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p58; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p58 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p58 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p59; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p59 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p59 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p60; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p60 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p60 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p61; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p61 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p61 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p62; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p62 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p62 FORCE ROW LEVEL SECURITY;


--
-- Name: attached_documents_p63; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.attached_documents_p63 (
    id uuid DEFAULT uuidv7() CONSTRAINT attached_documents_id_not_null NOT NULL,
    tenant_id uuid CONSTRAINT attached_documents_tenant_id_not_null NOT NULL,
    origin_table text CONSTRAINT attached_documents_origin_table_not_null NOT NULL,
    origin_id uuid CONSTRAINT attached_documents_origin_id_not_null NOT NULL,
    original_name text CONSTRAINT attached_documents_original_name_not_null NOT NULL,
    storage_key text CONSTRAINT attached_documents_storage_key_not_null NOT NULL,
    storage_provider text CONSTRAINT attached_documents_storage_provider_not_null NOT NULL,
    mime_type text CONSTRAINT attached_documents_mime_type_not_null NOT NULL,
    size_bytes bigint CONSTRAINT attached_documents_size_bytes_not_null NOT NULL,
    document_hash bytea CONSTRAINT attached_documents_document_hash_not_null NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_created_at_not_null NOT NULL,
    updated_at timestamp with time zone DEFAULT now() CONSTRAINT attached_documents_updated_at_not_null NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT attached_documents_hash_chk CHECK ((octet_length(document_hash) = 32)),
    CONSTRAINT attached_documents_mime_type_chk CHECK ((btrim(mime_type) <> ''::text)),
    CONSTRAINT attached_documents_origin_table_chk CHECK ((btrim(origin_table) <> ''::text)),
    CONSTRAINT attached_documents_original_name_chk CHECK (((btrim(original_name) <> ''::text) AND (original_name !~ '[/\\]'::text))),
    CONSTRAINT attached_documents_size_chk CHECK ((size_bytes >= 0)),
    CONSTRAINT attached_documents_storage_key_chk CHECK ((storage_key ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT attached_documents_storage_provider_chk CHECK ((btrim(storage_provider) <> ''::text))
);

ALTER TABLE ONLY tenants.attached_documents_p63 FORCE ROW LEVEL SECURITY;


--
-- Name: corporate_groups; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.corporate_groups (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT corporate_groups_name_chk CHECK ((btrim(name) <> ''::text))
);

ALTER TABLE ONLY tenants.corporate_groups FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE corporate_groups; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.corporate_groups IS 'Agrupación lógica de departamentos dentro de un tenant.';


--
-- Name: department_address_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_address_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_address_id uuid NOT NULL,
    department_id uuid NOT NULL,
    type text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT department_address_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date))),
    CONSTRAINT department_address_validities_type_chk CHECK ((type = ANY (ARRAY['LEGAL'::text, 'FISCAL'::text, 'POSTAL'::text, 'DELIVERY'::text])))
);

ALTER TABLE ONLY tenants.department_address_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_address_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_address_validities IS 'Vigencias de direcciones de department; una primaria por tipo.';


--
-- Name: department_addresses; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_addresses (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_id uuid NOT NULL,
    city_location_id uuid NOT NULL,
    type text NOT NULL,
    line1 text NOT NULL COLLATE public.unaccent_case_insensitive,
    line2 text COLLATE public.unaccent_case_insensitive,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT department_addresses_line1_chk CHECK ((btrim(line1) <> ''::text)),
    CONSTRAINT department_addresses_type_chk CHECK ((type = ANY (ARRAY['LEGAL'::text, 'FISCAL'::text, 'POSTAL'::text, 'DELIVERY'::text])))
);

ALTER TABLE ONLY tenants.department_addresses FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_addresses; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_addresses IS 'Direcciones MDM por department con FK compuesta de tenant.';


--
-- Name: department_contact_mechanism_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_contact_mechanism_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_contact_mechanism_id uuid CONSTRAINT department_contact_mechanis_department_contact_mechani_not_null NOT NULL,
    department_id uuid NOT NULL,
    contact_type text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT department_contact_mechanism_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date))),
    CONSTRAINT department_contact_mechanism_validities_type_chk CHECK ((contact_type = ANY (ARRAY['EMAIL'::text, 'PHONE'::text, 'MOBILE'::text, 'FAX'::text, 'WEBSITE'::text])))
);

ALTER TABLE ONLY tenants.department_contact_mechanism_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_contact_mechanism_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_contact_mechanism_validities IS 'Vigencias de contactos de department; una primaria por tipo.';


--
-- Name: department_contact_mechanisms; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_contact_mechanisms (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_id uuid NOT NULL,
    contact_mechanism_id uuid NOT NULL,
    contact_mechanism_order integer DEFAULT 0 NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT department_contact_mechanisms_order_chk CHECK ((contact_mechanism_order >= 0))
);

ALTER TABLE ONLY tenants.department_contact_mechanisms FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_contact_mechanisms; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_contact_mechanisms IS 'Contactos MDM por department con FK compuesta de tenant.';


--
-- Name: department_group_assignments; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_group_assignments (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_id uuid NOT NULL,
    corporate_group_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.department_group_assignments FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_group_assignments; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_group_assignments IS 'Asignación N:M entre departamentos y grupos corporativos del mismo tenant.';


--
-- Name: department_menu_option_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_menu_option_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_menu_option_id uuid CONSTRAINT department_menu_option_valid_department_menu_option_id_not_null NOT NULL,
    department_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT department_menu_option_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date)))
);

ALTER TABLE ONLY tenants.department_menu_option_validities FORCE ROW LEVEL SECURITY;


--
-- Name: department_menu_options; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_menu_options (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.department_menu_options FORCE ROW LEVEL SECURITY;


--
-- Name: department_translations; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.department_translations (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    department_id uuid NOT NULL,
    locale_code character varying(10) NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    description text COLLATE public.unaccent_case_insensitive,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT department_translations_name_chk CHECK ((btrim(name) <> ''::text))
);

ALTER TABLE ONLY tenants.department_translations FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE department_translations; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.department_translations IS 'Traducciones de departments; fallback final a departments.name.';


--
-- Name: departments; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.departments (
    id uuid DEFAULT uuidv7() NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    code character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    parent_department_id uuid,
    manager_party_id uuid,
    tenant_id uuid NOT NULL,
    unit_type text DEFAULT 'DEPARTMENT'::text NOT NULL,
    CONSTRAINT departments_code_chk CHECK ((btrim((code)::text) <> ''::text)),
    CONSTRAINT departments_name_chk CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT departments_parent_not_self_chk CHECK (((parent_department_id IS NULL) OR (parent_department_id <> id))),
    CONSTRAINT departments_unit_type_chk CHECK ((unit_type = ANY (ARRAY['DIVISION'::text, 'BRANCH'::text, 'OFFICE'::text, 'DEPARTMENT'::text])))
);

ALTER TABLE ONLY tenants.departments FORCE ROW LEVEL SECURITY;


--
-- Name: COLUMN departments.unit_type; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON COLUMN tenants.departments.unit_type IS 'Tipo de unidad: DIVISION, BRANCH, OFFICE o DEPARTMENT.';


--
-- Name: employee_assignments; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.employee_assignments (
    id uuid DEFAULT uuidv7() NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    salary numeric(14,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    department_id uuid NOT NULL,
    employee_party_id uuid NOT NULL,
    job_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT employee_assignments_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date))),
    CONSTRAINT employee_assignments_salary_chk CHECK ((salary >= (0)::numeric))
);

ALTER TABLE ONLY tenants.employee_assignments FORCE ROW LEVEL SECURITY;


--
-- Name: employee_menu_option_overrides; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.employee_menu_option_overrides (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    employee_party_id uuid NOT NULL,
    system_function_id uuid NOT NULL,
    is_granted boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.employee_menu_option_overrides FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE employee_menu_option_overrides; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.employee_menu_option_overrides IS 'Concesión o denegación directa de funciones a un empleado; prevalece sobre herencia.';


--
-- Name: employees; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.employees (
    party_id uuid NOT NULL,
    employee_code character varying NOT NULL,
    hire_date timestamp with time zone NOT NULL,
    status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    department_id uuid NOT NULL,
    manager_party_id uuid,
    job_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT employees_code_chk CHECK ((btrim((employee_code)::text) <> ''::text)),
    CONSTRAINT employees_not_own_manager_chk CHECK (((manager_party_id IS NULL) OR (manager_party_id <> party_id))),
    CONSTRAINT employees_status_chk CHECK (((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'ON_LEAVE'::character varying, 'TERMINATED'::character varying])::text[])))
);

ALTER TABLE ONLY tenants.employees FORCE ROW LEVEL SECURITY;


--
-- Name: job_families; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.job_families (
    id uuid DEFAULT uuidv7() NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    code character varying NOT NULL,
    description character varying,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    tenant_id uuid NOT NULL,
    CONSTRAINT job_families_code_chk CHECK ((btrim((code)::text) <> ''::text)),
    CONSTRAINT job_families_name_chk CHECK ((btrim(name) <> ''::text))
);

ALTER TABLE ONLY tenants.job_families FORCE ROW LEVEL SECURITY;


--
-- Name: jobs; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.jobs (
    id uuid DEFAULT uuidv7() NOT NULL,
    title text NOT NULL COLLATE public.unaccent_case_insensitive,
    code character varying NOT NULL,
    description character varying,
    min_salary numeric(14,2),
    max_salary numeric(14,2),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    job_family_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT jobs_code_chk CHECK ((btrim((code)::text) <> ''::text)),
    CONSTRAINT jobs_max_salary_chk CHECK (((max_salary IS NULL) OR (max_salary >= (0)::numeric))),
    CONSTRAINT jobs_min_salary_chk CHECK (((min_salary IS NULL) OR (min_salary >= (0)::numeric))),
    CONSTRAINT jobs_salary_range_chk CHECK (((min_salary IS NULL) OR (max_salary IS NULL) OR (min_salary <= max_salary))),
    CONSTRAINT jobs_title_chk CHECK ((btrim(title) <> ''::text))
);

ALTER TABLE ONLY tenants.jobs FORCE ROW LEVEL SECURITY;


--
-- Name: menu_categories; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.menu_categories (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    parent_id uuid,
    code text NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT menu_categories_code_chk CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT menu_categories_name_chk CHECK ((btrim(name) <> ''::text)),
    CONSTRAINT menu_categories_parent_not_self_chk CHECK (((parent_id IS NULL) OR (parent_id <> id))),
    CONSTRAINT menu_categories_sort_order_chk CHECK ((sort_order >= 0))
);

ALTER TABLE ONLY tenants.menu_categories FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE menu_categories; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.menu_categories IS 'Categorías jerárquicas de menú propias de cada tenant.';


--
-- Name: menu_option_categories; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.menu_option_categories (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    menu_category_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT menu_option_categories_sort_order_chk CHECK ((sort_order >= 0))
);

ALTER TABLE ONLY tenants.menu_option_categories FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE menu_option_categories; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.menu_option_categories IS 'Relación N:M entre categorías y opciones de menú con ordenamiento.';


--
-- Name: menu_option_functions; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.menu_option_functions (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    system_function_id uuid NOT NULL,
    is_required boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.menu_option_functions FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE menu_option_functions; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.menu_option_functions IS 'Funciones de sistema requeridas u opcionales para visualizar una opción de menú.';


--
-- Name: menu_options; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.menu_options (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    template_id uuid,
    code text NOT NULL,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    icon text,
    route text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT menu_options_code_chk CHECK ((btrim(code) <> ''::text)),
    CONSTRAINT menu_options_name_chk CHECK ((btrim(name) <> ''::text))
);

ALTER TABLE ONLY tenants.menu_options FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE menu_options; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.menu_options IS 'Opciones de menú configurables por tenant, opcionalmente basadas en plantillas.';


--
-- Name: organization_profiles; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.organization_profiles (
    id uuid DEFAULT uuidv7() NOT NULL,
    legal_name character varying NOT NULL,
    trade_name character varying,
    tax_id character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    party_id uuid NOT NULL,
    tenant_id uuid NOT NULL
);

ALTER TABLE ONLY tenants.organization_profiles FORCE ROW LEVEL SECURITY;


--
-- Name: parties; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.parties (
    id uuid DEFAULT uuidv7() NOT NULL,
    party_type character varying NOT NULL,
    display_name character varying NOT NULL,
    record_status character varying DEFAULT 'ACTIVE'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    canonical_party_id uuid,
    tenant_id uuid NOT NULL,
    CONSTRAINT parties_canonical_not_self_chk CHECK (((canonical_party_id IS NULL) OR (canonical_party_id <> id))),
    CONSTRAINT parties_record_status_chk CHECK (((record_status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'MERGED'::character varying, 'ARCHIVED'::character varying])::text[])))
);

ALTER TABLE ONLY tenants.parties FORCE ROW LEVEL SECURITY;


--
-- Name: party_contact_mechanism_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_contact_mechanism_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_contact_mechanism_id uuid CONSTRAINT party_contact_mechanism_val_party_contact_mechanism_id_not_null NOT NULL,
    party_id uuid NOT NULL,
    contact_mechanism_id uuid CONSTRAINT party_contact_mechanism_validitie_contact_mechanism_id_not_null NOT NULL,
    contact_type text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT party_contact_mechanism_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date))),
    CONSTRAINT party_contact_mechanism_validities_type_chk CHECK ((contact_type = ANY (ARRAY['EMAIL'::text, 'PHONE'::text, 'MOBILE'::text, 'FAX'::text, 'WEBSITE'::text])))
);

ALTER TABLE ONLY tenants.party_contact_mechanism_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_contact_mechanism_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_contact_mechanism_validities IS 'Vigencias de contactos; solo una primaria abierta por party/tipo.';


--
-- Name: party_contact_mechanisms; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_contact_mechanisms (
    id uuid DEFAULT uuidv7() NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    is_verified boolean DEFAULT false NOT NULL,
    contact_mechanism_order bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    contact_mechanism_id uuid NOT NULL,
    party_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT party_contact_mechanisms_order_chk CHECK ((contact_mechanism_order >= 0))
);

ALTER TABLE ONLY tenants.party_contact_mechanisms FORCE ROW LEVEL SECURITY;


--
-- Name: party_identity_document_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_identity_document_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_identity_document_id uuid CONSTRAINT party_identity_document_val_party_identity_document_id_not_null NOT NULL,
    party_id uuid NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT party_identity_document_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date)))
);

ALTER TABLE ONLY tenants.party_identity_document_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_identity_document_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_identity_document_validities IS 'Vigencias de documentos; una primaria abierta por party.';


--
-- Name: party_identity_documents; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_identity_documents (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    document_type_id uuid NOT NULL,
    issuing_country_code text NOT NULL,
    issuing_jurisdiction_id uuid,
    document_number text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT party_identity_documents_number_chk CHECK ((btrim(document_number) <> ''::text))
);

ALTER TABLE ONLY tenants.party_identity_documents FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_identity_documents; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_identity_documents IS 'Documentos de identidad maestros; las fechas viven en validities.';


--
-- Name: party_merge_log; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_merge_log (
    id uuid DEFAULT uuidv7() NOT NULL,
    merged_at timestamp with time zone DEFAULT now() NOT NULL,
    merged_by uuid,
    source_party_id uuid NOT NULL,
    target_party_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT party_merge_log_distinct_chk CHECK ((source_party_id <> target_party_id))
);

ALTER TABLE ONLY tenants.party_merge_log FORCE ROW LEVEL SECURITY;


--
-- Name: party_name_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_name_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_name_id uuid NOT NULL,
    party_id uuid NOT NULL,
    name_type_id uuid NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT party_name_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date)))
);

ALTER TABLE ONLY tenants.party_name_validities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_name_validities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_name_validities IS 'Vigencias de nombres; solo una primaria abierta por party/tipo.';


--
-- Name: party_names; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_names (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    name_type_id uuid NOT NULL,
    value text NOT NULL COLLATE public.unaccent_case_insensitive,
    is_primary boolean DEFAULT false NOT NULL,
    ordering integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT party_names_ordering_chk CHECK ((ordering >= 0)),
    CONSTRAINT party_names_value_chk CHECK ((btrim(value) <> ''::text))
);

ALTER TABLE ONLY tenants.party_names FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_names; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_names IS 'Nombres MDM por party; los valores no llevan fechas.';


--
-- Name: party_nationalities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_nationalities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    party_id uuid NOT NULL,
    country_code text NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.party_nationalities FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE party_nationalities; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.party_nationalities IS 'Nacionalidades por party; una primaria por party.';


--
-- Name: party_roles; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.party_roles (
    id uuid DEFAULT uuidv7() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    party_id uuid NOT NULL,
    role_type_id uuid NOT NULL,
    tenant_id uuid NOT NULL
);

ALTER TABLE ONLY tenants.party_roles FORCE ROW LEVEL SECURITY;


--
-- Name: password_reset_tokens; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.password_reset_tokens (
    id uuid DEFAULT uuidv7() NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    party_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT password_reset_tokens_expiry_chk CHECK ((expires_at > created_at)),
    CONSTRAINT password_reset_tokens_hash_chk CHECK ((octet_length(token_hash) = 32)),
    CONSTRAINT password_reset_tokens_used_chk CHECK (((used_at IS NULL) OR (used_at >= created_at)))
);

ALTER TABLE ONLY tenants.password_reset_tokens FORCE ROW LEVEL SECURITY;


--
-- Name: person_profiles; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.person_profiles (
    id uuid DEFAULT uuidv7() NOT NULL,
    first_name character varying NOT NULL,
    middle_name character varying,
    last_name character varying NOT NULL,
    second_last_name character varying,
    birth_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    party_id uuid NOT NULL,
    tenant_id uuid NOT NULL
);

ALTER TABLE ONLY tenants.person_profiles FORCE ROW LEVEL SECURITY;


--
-- Name: position_menu_option_validities; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.position_menu_option_validities (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    position_menu_option_id uuid CONSTRAINT position_menu_option_validitie_position_menu_option_id_not_null NOT NULL,
    job_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT position_menu_option_validities_dates_chk CHECK (((end_date IS NULL) OR (end_date > start_date)))
);

ALTER TABLE ONLY tenants.position_menu_option_validities FORCE ROW LEVEL SECURITY;


--
-- Name: position_menu_options; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.position_menu_options (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    job_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.position_menu_options FORCE ROW LEVEL SECURITY;


--
-- Name: role_menu_options; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.role_menu_options (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    tenant_role_id uuid NOT NULL,
    menu_option_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.role_menu_options FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE role_menu_options; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.role_menu_options IS 'Opciones de menú concedidas a un rol.';


--
-- Name: sessions; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.sessions (
    id uuid DEFAULT uuidv7() NOT NULL,
    token_hash bytea NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone,
    last_used_at timestamp with time zone,
    metadata jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT sessions_deleted_after_created_chk CHECK (((deleted_at IS NULL) OR (deleted_at >= created_at))),
    CONSTRAINT sessions_expires_after_created_chk CHECK ((expires_at > created_at)),
    CONSTRAINT sessions_last_used_after_created_chk CHECK (((last_used_at IS NULL) OR (last_used_at >= created_at))),
    CONSTRAINT sessions_metadata_object_chk CHECK ((jsonb_typeof(metadata) = 'object'::text)),
    CONSTRAINT sessions_revoked_after_created_chk CHECK (((revoked_at IS NULL) OR (revoked_at >= created_at))),
    CONSTRAINT sessions_token_hash_sha256_chk CHECK ((octet_length(token_hash) = 32))
);

ALTER TABLE ONLY tenants.sessions FORCE ROW LEVEL SECURITY;


--
-- Name: tenant_roles; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.tenant_roles (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    client_application_id uuid,
    name text NOT NULL COLLATE public.unaccent_case_insensitive,
    description text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT tenant_roles_name_chk CHECK ((btrim(name) <> ''::text))
);

ALTER TABLE ONLY tenants.tenant_roles FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE tenant_roles; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.tenant_roles IS 'Roles administrables por tenant para conceder opciones de menú a usuarios.';


--
-- Name: user_credentials; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.user_credentials (
    id uuid DEFAULT uuidv7() NOT NULL,
    password_hash character varying NOT NULL,
    totp_secret character varying,
    totp_enabled boolean DEFAULT false NOT NULL,
    backup_codes_hash jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    party_id uuid NOT NULL,
    party_contact_mechanism_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    CONSTRAINT user_credentials_backup_codes_chk CHECK ((jsonb_typeof(backup_codes_hash) = 'array'::text)),
    CONSTRAINT user_credentials_password_chk CHECK ((btrim((password_hash)::text) <> ''::text))
);

ALTER TABLE ONLY tenants.user_credentials FORCE ROW LEVEL SECURITY;


--
-- Name: user_role_assignments; Type: TABLE; Schema: tenants; Owner: -
--

CREATE TABLE tenants.user_role_assignments (
    id uuid DEFAULT uuidv7() NOT NULL,
    tenant_id uuid NOT NULL,
    user_credentials_id uuid NOT NULL,
    tenant_role_id uuid NOT NULL,
    is_granted boolean DEFAULT true NOT NULL,
    assigned_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY tenants.user_role_assignments FORCE ROW LEVEL SECURITY;


--
-- Name: TABLE user_role_assignments; Type: COMMENT; Schema: tenants; Owner: -
--

COMMENT ON TABLE tenants.user_role_assignments IS 'Asignación de roles a usuarios. is_granted=false anula los permisos del rol.';


--
-- Name: change_log_2026_11; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2026_11 FOR VALUES FROM ('2026-10-31 20:00:00-04') TO ('2026-11-30 20:00:00-04');


--
-- Name: change_log_2026_12; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2026_12 FOR VALUES FROM ('2026-11-30 20:00:00-04') TO ('2026-12-31 20:00:00-04');


--
-- Name: change_log_2027_01; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_01 FOR VALUES FROM ('2026-12-31 20:00:00-04') TO ('2027-01-31 20:00:00-04');


--
-- Name: change_log_2027_02; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_02 FOR VALUES FROM ('2027-01-31 20:00:00-04') TO ('2027-02-28 20:00:00-04');


--
-- Name: change_log_2027_03; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_03 FOR VALUES FROM ('2027-02-28 20:00:00-04') TO ('2027-03-31 20:00:00-04');


--
-- Name: change_log_2027_04; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_04 FOR VALUES FROM ('2027-03-31 20:00:00-04') TO ('2027-04-30 20:00:00-04');


--
-- Name: change_log_2027_05; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_05 FOR VALUES FROM ('2027-04-30 20:00:00-04') TO ('2027-05-31 20:00:00-04');


--
-- Name: change_log_2027_06; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_06 FOR VALUES FROM ('2027-05-31 20:00:00-04') TO ('2027-06-30 20:00:00-04');


--
-- Name: change_log_2027_07; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_07 FOR VALUES FROM ('2027-06-30 20:00:00-04') TO ('2027-07-31 20:00:00-04');


--
-- Name: change_log_2027_08; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_08 FOR VALUES FROM ('2027-07-31 20:00:00-04') TO ('2027-08-31 20:00:00-04');


--
-- Name: change_log_2027_09; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_09 FOR VALUES FROM ('2027-08-31 20:00:00-04') TO ('2027-09-30 20:00:00-04');


--
-- Name: change_log_2027_10; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_2027_10 FOR VALUES FROM ('2027-09-30 20:00:00-04') TO ('2027-10-31 20:00:00-04');


--
-- Name: change_log_default; Type: TABLE ATTACH; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log ATTACH PARTITION audit.change_log_default DEFAULT;


--
-- Name: attached_documents_p00; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p00 FOR VALUES WITH (modulus 64, remainder 0);


--
-- Name: attached_documents_p01; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p01 FOR VALUES WITH (modulus 64, remainder 1);


--
-- Name: attached_documents_p02; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p02 FOR VALUES WITH (modulus 64, remainder 2);


--
-- Name: attached_documents_p03; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p03 FOR VALUES WITH (modulus 64, remainder 3);


--
-- Name: attached_documents_p04; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p04 FOR VALUES WITH (modulus 64, remainder 4);


--
-- Name: attached_documents_p05; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p05 FOR VALUES WITH (modulus 64, remainder 5);


--
-- Name: attached_documents_p06; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p06 FOR VALUES WITH (modulus 64, remainder 6);


--
-- Name: attached_documents_p07; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p07 FOR VALUES WITH (modulus 64, remainder 7);


--
-- Name: attached_documents_p08; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p08 FOR VALUES WITH (modulus 64, remainder 8);


--
-- Name: attached_documents_p09; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p09 FOR VALUES WITH (modulus 64, remainder 9);


--
-- Name: attached_documents_p10; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p10 FOR VALUES WITH (modulus 64, remainder 10);


--
-- Name: attached_documents_p11; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p11 FOR VALUES WITH (modulus 64, remainder 11);


--
-- Name: attached_documents_p12; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p12 FOR VALUES WITH (modulus 64, remainder 12);


--
-- Name: attached_documents_p13; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p13 FOR VALUES WITH (modulus 64, remainder 13);


--
-- Name: attached_documents_p14; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p14 FOR VALUES WITH (modulus 64, remainder 14);


--
-- Name: attached_documents_p15; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p15 FOR VALUES WITH (modulus 64, remainder 15);


--
-- Name: attached_documents_p16; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p16 FOR VALUES WITH (modulus 64, remainder 16);


--
-- Name: attached_documents_p17; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p17 FOR VALUES WITH (modulus 64, remainder 17);


--
-- Name: attached_documents_p18; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p18 FOR VALUES WITH (modulus 64, remainder 18);


--
-- Name: attached_documents_p19; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p19 FOR VALUES WITH (modulus 64, remainder 19);


--
-- Name: attached_documents_p20; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p20 FOR VALUES WITH (modulus 64, remainder 20);


--
-- Name: attached_documents_p21; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p21 FOR VALUES WITH (modulus 64, remainder 21);


--
-- Name: attached_documents_p22; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p22 FOR VALUES WITH (modulus 64, remainder 22);


--
-- Name: attached_documents_p23; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p23 FOR VALUES WITH (modulus 64, remainder 23);


--
-- Name: attached_documents_p24; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p24 FOR VALUES WITH (modulus 64, remainder 24);


--
-- Name: attached_documents_p25; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p25 FOR VALUES WITH (modulus 64, remainder 25);


--
-- Name: attached_documents_p26; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p26 FOR VALUES WITH (modulus 64, remainder 26);


--
-- Name: attached_documents_p27; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p27 FOR VALUES WITH (modulus 64, remainder 27);


--
-- Name: attached_documents_p28; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p28 FOR VALUES WITH (modulus 64, remainder 28);


--
-- Name: attached_documents_p29; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p29 FOR VALUES WITH (modulus 64, remainder 29);


--
-- Name: attached_documents_p30; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p30 FOR VALUES WITH (modulus 64, remainder 30);


--
-- Name: attached_documents_p31; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p31 FOR VALUES WITH (modulus 64, remainder 31);


--
-- Name: attached_documents_p32; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p32 FOR VALUES WITH (modulus 64, remainder 32);


--
-- Name: attached_documents_p33; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p33 FOR VALUES WITH (modulus 64, remainder 33);


--
-- Name: attached_documents_p34; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p34 FOR VALUES WITH (modulus 64, remainder 34);


--
-- Name: attached_documents_p35; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p35 FOR VALUES WITH (modulus 64, remainder 35);


--
-- Name: attached_documents_p36; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p36 FOR VALUES WITH (modulus 64, remainder 36);


--
-- Name: attached_documents_p37; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p37 FOR VALUES WITH (modulus 64, remainder 37);


--
-- Name: attached_documents_p38; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p38 FOR VALUES WITH (modulus 64, remainder 38);


--
-- Name: attached_documents_p39; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p39 FOR VALUES WITH (modulus 64, remainder 39);


--
-- Name: attached_documents_p40; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p40 FOR VALUES WITH (modulus 64, remainder 40);


--
-- Name: attached_documents_p41; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p41 FOR VALUES WITH (modulus 64, remainder 41);


--
-- Name: attached_documents_p42; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p42 FOR VALUES WITH (modulus 64, remainder 42);


--
-- Name: attached_documents_p43; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p43 FOR VALUES WITH (modulus 64, remainder 43);


--
-- Name: attached_documents_p44; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p44 FOR VALUES WITH (modulus 64, remainder 44);


--
-- Name: attached_documents_p45; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p45 FOR VALUES WITH (modulus 64, remainder 45);


--
-- Name: attached_documents_p46; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p46 FOR VALUES WITH (modulus 64, remainder 46);


--
-- Name: attached_documents_p47; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p47 FOR VALUES WITH (modulus 64, remainder 47);


--
-- Name: attached_documents_p48; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p48 FOR VALUES WITH (modulus 64, remainder 48);


--
-- Name: attached_documents_p49; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p49 FOR VALUES WITH (modulus 64, remainder 49);


--
-- Name: attached_documents_p50; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p50 FOR VALUES WITH (modulus 64, remainder 50);


--
-- Name: attached_documents_p51; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p51 FOR VALUES WITH (modulus 64, remainder 51);


--
-- Name: attached_documents_p52; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p52 FOR VALUES WITH (modulus 64, remainder 52);


--
-- Name: attached_documents_p53; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p53 FOR VALUES WITH (modulus 64, remainder 53);


--
-- Name: attached_documents_p54; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p54 FOR VALUES WITH (modulus 64, remainder 54);


--
-- Name: attached_documents_p55; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p55 FOR VALUES WITH (modulus 64, remainder 55);


--
-- Name: attached_documents_p56; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p56 FOR VALUES WITH (modulus 64, remainder 56);


--
-- Name: attached_documents_p57; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p57 FOR VALUES WITH (modulus 64, remainder 57);


--
-- Name: attached_documents_p58; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p58 FOR VALUES WITH (modulus 64, remainder 58);


--
-- Name: attached_documents_p59; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p59 FOR VALUES WITH (modulus 64, remainder 59);


--
-- Name: attached_documents_p60; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p60 FOR VALUES WITH (modulus 64, remainder 60);


--
-- Name: attached_documents_p61; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p61 FOR VALUES WITH (modulus 64, remainder 61);


--
-- Name: attached_documents_p62; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p62 FOR VALUES WITH (modulus 64, remainder 62);


--
-- Name: attached_documents_p63; Type: TABLE ATTACH; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents ATTACH PARTITION tenants.attached_documents_p63 FOR VALUES WITH (modulus 64, remainder 63);


--
-- Name: change_log change_log_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log
    ADD CONSTRAINT change_log_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2026_11 change_log_2026_11_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2026_11
    ADD CONSTRAINT change_log_2026_11_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2026_12 change_log_2026_12_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2026_12
    ADD CONSTRAINT change_log_2026_12_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_01 change_log_2027_01_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_01
    ADD CONSTRAINT change_log_2027_01_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_02 change_log_2027_02_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_02
    ADD CONSTRAINT change_log_2027_02_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_03 change_log_2027_03_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_03
    ADD CONSTRAINT change_log_2027_03_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_04 change_log_2027_04_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_04
    ADD CONSTRAINT change_log_2027_04_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_05 change_log_2027_05_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_05
    ADD CONSTRAINT change_log_2027_05_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_06 change_log_2027_06_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_06
    ADD CONSTRAINT change_log_2027_06_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_07 change_log_2027_07_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_07
    ADD CONSTRAINT change_log_2027_07_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_08 change_log_2027_08_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_08
    ADD CONSTRAINT change_log_2027_08_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_09 change_log_2027_09_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_09
    ADD CONSTRAINT change_log_2027_09_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_2027_10 change_log_2027_10_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_2027_10
    ADD CONSTRAINT change_log_2027_10_pkey PRIMARY KEY (id, changed_at);


--
-- Name: change_log_default change_log_default_pkey; Type: CONSTRAINT; Schema: audit; Owner: -
--

ALTER TABLE ONLY audit.change_log_default
    ADD CONSTRAINT change_log_default_pkey PRIMARY KEY (id, changed_at);


--
-- Name: client_applications client_applications_client_id_key; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.client_applications
    ADD CONSTRAINT client_applications_client_id_key UNIQUE (client_id);


--
-- Name: client_applications client_applications_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.client_applications
    ADD CONSTRAINT client_applications_pkey PRIMARY KEY (id);


--
-- Name: contact_mechanisms contact_mechanisms_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.contact_mechanisms
    ADD CONSTRAINT contact_mechanisms_pkey PRIMARY KEY (id);


--
-- Name: countries countries_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.countries
    ADD CONSTRAINT countries_pkey PRIMARY KEY (code);


--
-- Name: identity_document_type_translations identity_document_type_translations_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_type_translations
    ADD CONSTRAINT identity_document_type_translations_pkey PRIMARY KEY (id);


--
-- Name: identity_document_type_translations identity_document_type_translations_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_type_translations
    ADD CONSTRAINT identity_document_type_translations_uq UNIQUE (identity_document_type_id, locale_code);


--
-- Name: identity_document_types identity_document_types_code_key; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_types
    ADD CONSTRAINT identity_document_types_code_key UNIQUE (code);


--
-- Name: identity_document_types identity_document_types_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_types
    ADD CONSTRAINT identity_document_types_pkey PRIMARY KEY (id);


--
-- Name: locales locales_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.locales
    ADD CONSTRAINT locales_pkey PRIMARY KEY (code);


--
-- Name: location_translations location_translations_id_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.location_translations
    ADD CONSTRAINT location_translations_id_uq UNIQUE (id);


--
-- Name: location_translations location_translations_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.location_translations
    ADD CONSTRAINT location_translations_pkey PRIMARY KEY (location_id, language_code);


--
-- Name: locations locations_id_country_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.locations
    ADD CONSTRAINT locations_id_country_uq UNIQUE (id, country_code);


--
-- Name: locations locations_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.locations
    ADD CONSTRAINT locations_pkey PRIMARY KEY (id);


--
-- Name: menu_template_functions menu_template_functions_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.menu_template_functions
    ADD CONSTRAINT menu_template_functions_pkey PRIMARY KEY (id);


--
-- Name: menu_template_functions menu_template_functions_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.menu_template_functions
    ADD CONSTRAINT menu_template_functions_uq UNIQUE (template_id, system_function_id);


--
-- Name: menu_templates menu_templates_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.menu_templates
    ADD CONSTRAINT menu_templates_pkey PRIMARY KEY (id);


--
-- Name: name_types name_types_code_key; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.name_types
    ADD CONSTRAINT name_types_code_key UNIQUE (code);


--
-- Name: name_types name_types_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.name_types
    ADD CONSTRAINT name_types_pkey PRIMARY KEY (id);


--
-- Name: role_type_translations role_type_translations_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.role_type_translations
    ADD CONSTRAINT role_type_translations_pkey PRIMARY KEY (id);


--
-- Name: role_type_translations role_type_translations_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.role_type_translations
    ADD CONSTRAINT role_type_translations_uq UNIQUE (role_type_id, locale_code);


--
-- Name: role_types role_types_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.role_types
    ADD CONSTRAINT role_types_pkey PRIMARY KEY (id);


--
-- Name: subscriptions subscriptions_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (id);


--
-- Name: system_functions system_functions_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.system_functions
    ADD CONSTRAINT system_functions_pkey PRIMARY KEY (id);


--
-- Name: tenant_relationship_invites tenant_relationship_invites_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationship_invites
    ADD CONSTRAINT tenant_relationship_invites_pkey PRIMARY KEY (id);


--
-- Name: tenant_relationship_invites tenant_relationship_invites_token_uq; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationship_invites
    ADD CONSTRAINT tenant_relationship_invites_token_uq UNIQUE (token_hash);


--
-- Name: tenant_relationships tenant_relationships_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationships
    ADD CONSTRAINT tenant_relationships_pkey PRIMARY KEY (id);


--
-- Name: tenant_transfers tenant_transfers_pkey; Type: CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_transfers
    ADD CONSTRAINT tenant_transfers_pkey PRIMARY KEY (id);


--
-- Name: address_validities address_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.address_validities
    ADD CONSTRAINT address_validities_pkey PRIMARY KEY (id);


--
-- Name: addresses addresses_id_tenant_party_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.addresses
    ADD CONSTRAINT addresses_id_tenant_party_uq UNIQUE (id, tenant_id, party_id);


--
-- Name: addresses addresses_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.addresses
    ADD CONSTRAINT addresses_pkey PRIMARY KEY (id);


--
-- Name: attached_documents attached_documents_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents
    ADD CONSTRAINT attached_documents_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p00 attached_documents_p00_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p00
    ADD CONSTRAINT attached_documents_p00_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p01 attached_documents_p01_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p01
    ADD CONSTRAINT attached_documents_p01_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p02 attached_documents_p02_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p02
    ADD CONSTRAINT attached_documents_p02_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p03 attached_documents_p03_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p03
    ADD CONSTRAINT attached_documents_p03_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p04 attached_documents_p04_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p04
    ADD CONSTRAINT attached_documents_p04_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p05 attached_documents_p05_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p05
    ADD CONSTRAINT attached_documents_p05_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p06 attached_documents_p06_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p06
    ADD CONSTRAINT attached_documents_p06_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p07 attached_documents_p07_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p07
    ADD CONSTRAINT attached_documents_p07_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p08 attached_documents_p08_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p08
    ADD CONSTRAINT attached_documents_p08_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p09 attached_documents_p09_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p09
    ADD CONSTRAINT attached_documents_p09_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p10 attached_documents_p10_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p10
    ADD CONSTRAINT attached_documents_p10_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p11 attached_documents_p11_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p11
    ADD CONSTRAINT attached_documents_p11_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p12 attached_documents_p12_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p12
    ADD CONSTRAINT attached_documents_p12_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p13 attached_documents_p13_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p13
    ADD CONSTRAINT attached_documents_p13_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p14 attached_documents_p14_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p14
    ADD CONSTRAINT attached_documents_p14_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p15 attached_documents_p15_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p15
    ADD CONSTRAINT attached_documents_p15_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p16 attached_documents_p16_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p16
    ADD CONSTRAINT attached_documents_p16_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p17 attached_documents_p17_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p17
    ADD CONSTRAINT attached_documents_p17_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p18 attached_documents_p18_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p18
    ADD CONSTRAINT attached_documents_p18_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p19 attached_documents_p19_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p19
    ADD CONSTRAINT attached_documents_p19_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p20 attached_documents_p20_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p20
    ADD CONSTRAINT attached_documents_p20_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p21 attached_documents_p21_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p21
    ADD CONSTRAINT attached_documents_p21_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p22 attached_documents_p22_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p22
    ADD CONSTRAINT attached_documents_p22_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p23 attached_documents_p23_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p23
    ADD CONSTRAINT attached_documents_p23_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p24 attached_documents_p24_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p24
    ADD CONSTRAINT attached_documents_p24_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p25 attached_documents_p25_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p25
    ADD CONSTRAINT attached_documents_p25_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p26 attached_documents_p26_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p26
    ADD CONSTRAINT attached_documents_p26_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p27 attached_documents_p27_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p27
    ADD CONSTRAINT attached_documents_p27_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p28 attached_documents_p28_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p28
    ADD CONSTRAINT attached_documents_p28_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p29 attached_documents_p29_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p29
    ADD CONSTRAINT attached_documents_p29_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p30 attached_documents_p30_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p30
    ADD CONSTRAINT attached_documents_p30_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p31 attached_documents_p31_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p31
    ADD CONSTRAINT attached_documents_p31_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p32 attached_documents_p32_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p32
    ADD CONSTRAINT attached_documents_p32_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p33 attached_documents_p33_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p33
    ADD CONSTRAINT attached_documents_p33_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p34 attached_documents_p34_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p34
    ADD CONSTRAINT attached_documents_p34_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p35 attached_documents_p35_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p35
    ADD CONSTRAINT attached_documents_p35_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p36 attached_documents_p36_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p36
    ADD CONSTRAINT attached_documents_p36_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p37 attached_documents_p37_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p37
    ADD CONSTRAINT attached_documents_p37_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p38 attached_documents_p38_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p38
    ADD CONSTRAINT attached_documents_p38_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p39 attached_documents_p39_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p39
    ADD CONSTRAINT attached_documents_p39_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p40 attached_documents_p40_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p40
    ADD CONSTRAINT attached_documents_p40_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p41 attached_documents_p41_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p41
    ADD CONSTRAINT attached_documents_p41_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p42 attached_documents_p42_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p42
    ADD CONSTRAINT attached_documents_p42_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p43 attached_documents_p43_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p43
    ADD CONSTRAINT attached_documents_p43_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p44 attached_documents_p44_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p44
    ADD CONSTRAINT attached_documents_p44_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p45 attached_documents_p45_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p45
    ADD CONSTRAINT attached_documents_p45_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p46 attached_documents_p46_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p46
    ADD CONSTRAINT attached_documents_p46_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p47 attached_documents_p47_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p47
    ADD CONSTRAINT attached_documents_p47_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p48 attached_documents_p48_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p48
    ADD CONSTRAINT attached_documents_p48_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p49 attached_documents_p49_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p49
    ADD CONSTRAINT attached_documents_p49_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p50 attached_documents_p50_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p50
    ADD CONSTRAINT attached_documents_p50_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p51 attached_documents_p51_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p51
    ADD CONSTRAINT attached_documents_p51_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p52 attached_documents_p52_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p52
    ADD CONSTRAINT attached_documents_p52_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p53 attached_documents_p53_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p53
    ADD CONSTRAINT attached_documents_p53_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p54 attached_documents_p54_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p54
    ADD CONSTRAINT attached_documents_p54_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p55 attached_documents_p55_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p55
    ADD CONSTRAINT attached_documents_p55_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p56 attached_documents_p56_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p56
    ADD CONSTRAINT attached_documents_p56_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p57 attached_documents_p57_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p57
    ADD CONSTRAINT attached_documents_p57_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p58 attached_documents_p58_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p58
    ADD CONSTRAINT attached_documents_p58_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p59 attached_documents_p59_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p59
    ADD CONSTRAINT attached_documents_p59_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p60 attached_documents_p60_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p60
    ADD CONSTRAINT attached_documents_p60_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p61 attached_documents_p61_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p61
    ADD CONSTRAINT attached_documents_p61_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p62 attached_documents_p62_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p62
    ADD CONSTRAINT attached_documents_p62_pkey PRIMARY KEY (id, document_hash);


--
-- Name: attached_documents_p63 attached_documents_p63_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.attached_documents_p63
    ADD CONSTRAINT attached_documents_p63_pkey PRIMARY KEY (id, document_hash);


--
-- Name: corporate_groups corporate_groups_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.corporate_groups
    ADD CONSTRAINT corporate_groups_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: corporate_groups corporate_groups_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.corporate_groups
    ADD CONSTRAINT corporate_groups_pkey PRIMARY KEY (id);


--
-- Name: department_address_validities department_address_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_address_validities
    ADD CONSTRAINT department_address_validities_pkey PRIMARY KEY (id);


--
-- Name: department_addresses department_addresses_id_tenant_department_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_addresses
    ADD CONSTRAINT department_addresses_id_tenant_department_uq UNIQUE (id, tenant_id, department_id);


--
-- Name: department_addresses department_addresses_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_addresses
    ADD CONSTRAINT department_addresses_pkey PRIMARY KEY (id);


--
-- Name: department_contact_mechanism_validities department_contact_mechanism_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanism_validities
    ADD CONSTRAINT department_contact_mechanism_validities_pkey PRIMARY KEY (id);


--
-- Name: department_contact_mechanisms department_contact_mechanisms_id_tenant_department_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanisms
    ADD CONSTRAINT department_contact_mechanisms_id_tenant_department_uq UNIQUE (id, tenant_id, department_id);


--
-- Name: department_contact_mechanisms department_contact_mechanisms_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanisms
    ADD CONSTRAINT department_contact_mechanisms_pkey PRIMARY KEY (id);


--
-- Name: department_group_assignments department_group_assignments_id_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_group_assignments
    ADD CONSTRAINT department_group_assignments_id_uq UNIQUE (id);


--
-- Name: department_group_assignments department_group_assignments_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_group_assignments
    ADD CONSTRAINT department_group_assignments_pkey PRIMARY KEY (department_id, corporate_group_id);


--
-- Name: department_menu_option_validities department_menu_option_validities_no_overlap; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_option_validities
    ADD CONSTRAINT department_menu_option_validities_no_overlap EXCLUDE USING gist (tenant_id WITH =, department_id WITH =, menu_option_id WITH =, tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamp with time zone), '[)'::text) WITH &&);


--
-- Name: department_menu_option_validities department_menu_option_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_option_validities
    ADD CONSTRAINT department_menu_option_validities_pkey PRIMARY KEY (id);


--
-- Name: department_menu_options department_menu_options_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_id_tenant_uq UNIQUE (id, tenant_id, department_id, menu_option_id);


--
-- Name: department_menu_options department_menu_options_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_pkey PRIMARY KEY (id);


--
-- Name: department_menu_options department_menu_options_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_uq UNIQUE (tenant_id, department_id, menu_option_id);


--
-- Name: department_translations department_translations_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_translations
    ADD CONSTRAINT department_translations_pkey PRIMARY KEY (id);


--
-- Name: department_translations department_translations_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_translations
    ADD CONSTRAINT department_translations_uq UNIQUE (tenant_id, department_id, locale_code);


--
-- Name: departments departments_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.departments
    ADD CONSTRAINT departments_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: departments departments_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.departments
    ADD CONSTRAINT departments_pkey PRIMARY KEY (id);


--
-- Name: employee_assignments employee_assignments_no_overlap; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_no_overlap EXCLUDE USING gist (tenant_id WITH =, employee_party_id WITH =, tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamp with time zone), '[)'::text) WITH &&);


--
-- Name: employee_assignments employee_assignments_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_pkey PRIMARY KEY (id);


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_menu_option_overrides
    ADD CONSTRAINT employee_menu_option_overrides_pkey PRIMARY KEY (id);


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_menu_option_overrides
    ADD CONSTRAINT employee_menu_option_overrides_uq UNIQUE (tenant_id, employee_party_id, system_function_id);


--
-- Name: employees employees_party_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_party_tenant_uq UNIQUE (party_id, tenant_id);


--
-- Name: employees employees_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (party_id);


--
-- Name: job_families job_families_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.job_families
    ADD CONSTRAINT job_families_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.jobs
    ADD CONSTRAINT jobs_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: menu_categories menu_categories_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_categories
    ADD CONSTRAINT menu_categories_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: menu_categories menu_categories_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_categories
    ADD CONSTRAINT menu_categories_pkey PRIMARY KEY (id);


--
-- Name: menu_option_categories menu_option_categories_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_categories
    ADD CONSTRAINT menu_option_categories_pkey PRIMARY KEY (id);


--
-- Name: menu_option_categories menu_option_categories_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_categories
    ADD CONSTRAINT menu_option_categories_uq UNIQUE (tenant_id, menu_category_id, menu_option_id);


--
-- Name: menu_option_functions menu_option_functions_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_functions
    ADD CONSTRAINT menu_option_functions_pkey PRIMARY KEY (id);


--
-- Name: menu_option_functions menu_option_functions_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_functions
    ADD CONSTRAINT menu_option_functions_uq UNIQUE (tenant_id, menu_option_id, system_function_id);


--
-- Name: menu_options menu_options_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_options
    ADD CONSTRAINT menu_options_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: menu_options menu_options_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_options
    ADD CONSTRAINT menu_options_pkey PRIMARY KEY (id);


--
-- Name: organization_profiles organization_profiles_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.organization_profiles
    ADD CONSTRAINT organization_profiles_pkey PRIMARY KEY (id);


--
-- Name: parties parties_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.parties
    ADD CONSTRAINT parties_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: parties parties_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.parties
    ADD CONSTRAINT parties_pkey PRIMARY KEY (id);


--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanism_validities
    ADD CONSTRAINT party_contact_mechanism_validities_pkey PRIMARY KEY (id);


--
-- Name: party_contact_mechanisms party_contact_mechanisms_id_tenant_party_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanisms
    ADD CONSTRAINT party_contact_mechanisms_id_tenant_party_uq UNIQUE (id, tenant_id, party_id);


--
-- Name: party_contact_mechanisms party_contact_mechanisms_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanisms
    ADD CONSTRAINT party_contact_mechanisms_pkey PRIMARY KEY (id);


--
-- Name: party_identity_document_validities party_identity_document_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_document_validities
    ADD CONSTRAINT party_identity_document_validities_pkey PRIMARY KEY (id);


--
-- Name: party_identity_documents party_identity_documents_id_tenant_party_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_id_tenant_party_uq UNIQUE (id, tenant_id, party_id);


--
-- Name: party_identity_documents party_identity_documents_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_pkey PRIMARY KEY (id);


--
-- Name: party_merge_log party_merge_log_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_merge_log
    ADD CONSTRAINT party_merge_log_pkey PRIMARY KEY (id);


--
-- Name: party_name_validities party_name_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_name_validities
    ADD CONSTRAINT party_name_validities_pkey PRIMARY KEY (id);


--
-- Name: party_names party_names_id_tenant_party_type_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_names
    ADD CONSTRAINT party_names_id_tenant_party_type_uq UNIQUE (id, tenant_id, party_id, name_type_id);


--
-- Name: party_names party_names_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_names
    ADD CONSTRAINT party_names_pkey PRIMARY KEY (id);


--
-- Name: party_nationalities party_nationalities_id_tenant_party_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_nationalities
    ADD CONSTRAINT party_nationalities_id_tenant_party_uq UNIQUE (id, tenant_id, party_id);


--
-- Name: party_nationalities party_nationalities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_nationalities
    ADD CONSTRAINT party_nationalities_pkey PRIMARY KEY (id);


--
-- Name: party_roles party_roles_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_roles
    ADD CONSTRAINT party_roles_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: person_profiles person_profiles_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.person_profiles
    ADD CONSTRAINT person_profiles_pkey PRIMARY KEY (id);


--
-- Name: position_menu_option_validities position_menu_option_validities_no_overlap; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_option_validities
    ADD CONSTRAINT position_menu_option_validities_no_overlap EXCLUDE USING gist (tenant_id WITH =, job_id WITH =, menu_option_id WITH =, tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamp with time zone), '[)'::text) WITH &&);


--
-- Name: position_menu_option_validities position_menu_option_validities_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_option_validities
    ADD CONSTRAINT position_menu_option_validities_pkey PRIMARY KEY (id);


--
-- Name: position_menu_options position_menu_options_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_id_tenant_uq UNIQUE (id, tenant_id, job_id, menu_option_id);


--
-- Name: position_menu_options position_menu_options_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_pkey PRIMARY KEY (id);


--
-- Name: position_menu_options position_menu_options_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_uq UNIQUE (tenant_id, job_id, menu_option_id);


--
-- Name: role_menu_options role_menu_options_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.role_menu_options
    ADD CONSTRAINT role_menu_options_pkey PRIMARY KEY (id);


--
-- Name: role_menu_options role_menu_options_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.role_menu_options
    ADD CONSTRAINT role_menu_options_uq UNIQUE (tenant_id, tenant_role_id, menu_option_id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: tenant_roles tenant_roles_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.tenant_roles
    ADD CONSTRAINT tenant_roles_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: tenant_roles tenant_roles_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.tenant_roles
    ADD CONSTRAINT tenant_roles_pkey PRIMARY KEY (id);


--
-- Name: user_credentials user_credentials_id_tenant_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_credentials
    ADD CONSTRAINT user_credentials_id_tenant_uq UNIQUE (id, tenant_id);


--
-- Name: user_credentials user_credentials_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_credentials
    ADD CONSTRAINT user_credentials_pkey PRIMARY KEY (id);


--
-- Name: user_role_assignments user_role_assignments_pkey; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_role_assignments
    ADD CONSTRAINT user_role_assignments_pkey PRIMARY KEY (id);


--
-- Name: user_role_assignments user_role_assignments_uq; Type: CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_role_assignments
    ADD CONSTRAINT user_role_assignments_uq UNIQUE (tenant_id, user_credentials_id, tenant_role_id);


--
-- Name: change_log_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_changed_at_idx ON ONLY audit.change_log USING btree (changed_at);


--
-- Name: INDEX change_log_changed_at_idx; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON INDEX audit.change_log_changed_at_idx IS 'Consultas por rango temporal (retención y auditoría por fecha).';


--
-- Name: change_log_2026_11_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_11_changed_at_idx ON audit.change_log_2026_11 USING btree (changed_at);


--
-- Name: change_log_changed_by_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_changed_by_idx ON ONLY audit.change_log USING btree (changed_by, changed_at DESC);


--
-- Name: INDEX change_log_changed_by_idx; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON INDEX audit.change_log_changed_by_idx IS 'Actividad de auditoría por usuario.';


--
-- Name: change_log_2026_11_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_11_changed_by_changed_at_idx ON audit.change_log_2026_11 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_changed_fields_gin; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_changed_fields_gin ON ONLY audit.change_log USING gin (changed_fields jsonb_path_ops);


--
-- Name: INDEX change_log_changed_fields_gin; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON INDEX audit.change_log_changed_fields_gin IS 'Búsqueda por campo cambiado dentro de changed_fields.';


--
-- Name: change_log_2026_11_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_11_changed_fields_idx ON audit.change_log_2026_11 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_table_record_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_table_record_idx ON ONLY audit.change_log USING btree (table_name, record_id, changed_at DESC);


--
-- Name: INDEX change_log_table_record_idx; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON INDEX audit.change_log_table_record_idx IS 'Historial de una fila concreta de una tabla.';


--
-- Name: change_log_2026_11_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_11_table_name_record_id_changed_at_idx ON audit.change_log_2026_11 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_tenant_changed_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_tenant_changed_idx ON ONLY audit.change_log USING btree (tenant_id, changed_at DESC);


--
-- Name: INDEX change_log_tenant_changed_idx; Type: COMMENT; Schema: audit; Owner: -
--

COMMENT ON INDEX audit.change_log_tenant_changed_idx IS 'Actividad de auditoría por tenant.';


--
-- Name: change_log_2026_11_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_11_tenant_id_changed_at_idx ON audit.change_log_2026_11 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2026_12_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_12_changed_at_idx ON audit.change_log_2026_12 USING btree (changed_at);


--
-- Name: change_log_2026_12_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_12_changed_by_changed_at_idx ON audit.change_log_2026_12 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2026_12_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_12_changed_fields_idx ON audit.change_log_2026_12 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2026_12_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_12_table_name_record_id_changed_at_idx ON audit.change_log_2026_12 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2026_12_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2026_12_tenant_id_changed_at_idx ON audit.change_log_2026_12 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_01_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_01_changed_at_idx ON audit.change_log_2027_01 USING btree (changed_at);


--
-- Name: change_log_2027_01_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_01_changed_by_changed_at_idx ON audit.change_log_2027_01 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_01_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_01_changed_fields_idx ON audit.change_log_2027_01 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_01_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_01_table_name_record_id_changed_at_idx ON audit.change_log_2027_01 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_01_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_01_tenant_id_changed_at_idx ON audit.change_log_2027_01 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_02_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_02_changed_at_idx ON audit.change_log_2027_02 USING btree (changed_at);


--
-- Name: change_log_2027_02_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_02_changed_by_changed_at_idx ON audit.change_log_2027_02 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_02_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_02_changed_fields_idx ON audit.change_log_2027_02 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_02_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_02_table_name_record_id_changed_at_idx ON audit.change_log_2027_02 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_02_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_02_tenant_id_changed_at_idx ON audit.change_log_2027_02 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_03_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_03_changed_at_idx ON audit.change_log_2027_03 USING btree (changed_at);


--
-- Name: change_log_2027_03_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_03_changed_by_changed_at_idx ON audit.change_log_2027_03 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_03_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_03_changed_fields_idx ON audit.change_log_2027_03 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_03_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_03_table_name_record_id_changed_at_idx ON audit.change_log_2027_03 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_03_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_03_tenant_id_changed_at_idx ON audit.change_log_2027_03 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_04_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_04_changed_at_idx ON audit.change_log_2027_04 USING btree (changed_at);


--
-- Name: change_log_2027_04_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_04_changed_by_changed_at_idx ON audit.change_log_2027_04 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_04_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_04_changed_fields_idx ON audit.change_log_2027_04 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_04_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_04_table_name_record_id_changed_at_idx ON audit.change_log_2027_04 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_04_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_04_tenant_id_changed_at_idx ON audit.change_log_2027_04 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_05_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_05_changed_at_idx ON audit.change_log_2027_05 USING btree (changed_at);


--
-- Name: change_log_2027_05_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_05_changed_by_changed_at_idx ON audit.change_log_2027_05 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_05_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_05_changed_fields_idx ON audit.change_log_2027_05 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_05_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_05_table_name_record_id_changed_at_idx ON audit.change_log_2027_05 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_05_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_05_tenant_id_changed_at_idx ON audit.change_log_2027_05 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_06_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_06_changed_at_idx ON audit.change_log_2027_06 USING btree (changed_at);


--
-- Name: change_log_2027_06_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_06_changed_by_changed_at_idx ON audit.change_log_2027_06 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_06_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_06_changed_fields_idx ON audit.change_log_2027_06 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_06_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_06_table_name_record_id_changed_at_idx ON audit.change_log_2027_06 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_06_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_06_tenant_id_changed_at_idx ON audit.change_log_2027_06 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_07_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_07_changed_at_idx ON audit.change_log_2027_07 USING btree (changed_at);


--
-- Name: change_log_2027_07_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_07_changed_by_changed_at_idx ON audit.change_log_2027_07 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_07_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_07_changed_fields_idx ON audit.change_log_2027_07 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_07_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_07_table_name_record_id_changed_at_idx ON audit.change_log_2027_07 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_07_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_07_tenant_id_changed_at_idx ON audit.change_log_2027_07 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_08_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_08_changed_at_idx ON audit.change_log_2027_08 USING btree (changed_at);


--
-- Name: change_log_2027_08_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_08_changed_by_changed_at_idx ON audit.change_log_2027_08 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_08_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_08_changed_fields_idx ON audit.change_log_2027_08 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_08_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_08_table_name_record_id_changed_at_idx ON audit.change_log_2027_08 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_08_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_08_tenant_id_changed_at_idx ON audit.change_log_2027_08 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_09_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_09_changed_at_idx ON audit.change_log_2027_09 USING btree (changed_at);


--
-- Name: change_log_2027_09_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_09_changed_by_changed_at_idx ON audit.change_log_2027_09 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_09_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_09_changed_fields_idx ON audit.change_log_2027_09 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_09_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_09_table_name_record_id_changed_at_idx ON audit.change_log_2027_09 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_09_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_09_tenant_id_changed_at_idx ON audit.change_log_2027_09 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_2027_10_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_10_changed_at_idx ON audit.change_log_2027_10 USING btree (changed_at);


--
-- Name: change_log_2027_10_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_10_changed_by_changed_at_idx ON audit.change_log_2027_10 USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_2027_10_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_10_changed_fields_idx ON audit.change_log_2027_10 USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_2027_10_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_10_table_name_record_id_changed_at_idx ON audit.change_log_2027_10 USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_2027_10_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_2027_10_tenant_id_changed_at_idx ON audit.change_log_2027_10 USING btree (tenant_id, changed_at DESC);


--
-- Name: change_log_default_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_default_changed_at_idx ON audit.change_log_default USING btree (changed_at);


--
-- Name: change_log_default_changed_by_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_default_changed_by_changed_at_idx ON audit.change_log_default USING btree (changed_by, changed_at DESC);


--
-- Name: change_log_default_changed_fields_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_default_changed_fields_idx ON audit.change_log_default USING gin (changed_fields jsonb_path_ops);


--
-- Name: change_log_default_table_name_record_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_default_table_name_record_id_changed_at_idx ON audit.change_log_default USING btree (table_name, record_id, changed_at DESC);


--
-- Name: change_log_default_tenant_id_changed_at_idx; Type: INDEX; Schema: audit; Owner: -
--

CREATE INDEX change_log_default_tenant_id_changed_at_idx ON audit.change_log_default USING btree (tenant_id, changed_at DESC);


--
-- Name: client_applications_active_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX client_applications_active_idx ON global.client_applications USING btree (client_id) WHERE (is_active AND (deleted_at IS NULL));


--
-- Name: client_applications_platform_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX client_applications_platform_idx ON global.client_applications USING btree (application_scope, client_type) WHERE (is_active AND (deleted_at IS NULL));


--
-- Name: client_applications_tenant_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX client_applications_tenant_idx ON global.client_applications USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: contact_mechanisms_type_value_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX contact_mechanisms_type_value_uq ON global.contact_mechanisms USING btree (type, value);


--
-- Name: identity_document_type_translations_locale_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX identity_document_type_translations_locale_idx ON global.identity_document_type_translations USING btree (locale_code);


--
-- Name: locales_one_default_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX locales_one_default_uq ON global.locales USING btree (is_default) WHERE is_default;


--
-- Name: location_translations_language_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX location_translations_language_idx ON global.location_translations USING btree (language_code);


--
-- Name: location_translations_name_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX location_translations_name_idx ON global.location_translations USING btree (name);


--
-- Name: locations_country_external_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX locations_country_external_uq ON global.locations USING btree (country_code, external_id) WHERE (external_id IS NOT NULL);


--
-- Name: locations_country_iso_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX locations_country_iso_uq ON global.locations USING btree (country_code, iso_code) WHERE (iso_code IS NOT NULL);


--
-- Name: locations_country_parent_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX locations_country_parent_idx ON global.locations USING btree (country_code, parent_id);


--
-- Name: locations_country_type_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX locations_country_type_idx ON global.locations USING btree (country_code, location_type);


--
-- Name: locations_parent_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX locations_parent_idx ON global.locations USING btree (parent_id);


--
-- Name: locations_path_gist_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX locations_path_gist_idx ON global.locations USING gist (path);


--
-- Name: locations_path_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX locations_path_uq ON global.locations USING btree (path);


--
-- Name: menu_template_functions_function_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX menu_template_functions_function_idx ON global.menu_template_functions USING btree (system_function_id);


--
-- Name: menu_template_functions_template_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX menu_template_functions_template_idx ON global.menu_template_functions USING btree (template_id);


--
-- Name: menu_templates_code_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX menu_templates_code_uq ON global.menu_templates USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: role_type_translations_locale_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX role_type_translations_locale_idx ON global.role_type_translations USING btree (locale_code);


--
-- Name: roletype_code; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX roletype_code ON global.role_types USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: subscription_slug; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX subscription_slug ON global.subscriptions USING btree (slug) WHERE (deleted_at IS NULL);


--
-- Name: system_functions_code_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX system_functions_code_uq ON global.system_functions USING btree (code) WHERE (deleted_at IS NULL);


--
-- Name: system_functions_scope_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX system_functions_scope_idx ON global.system_functions USING btree (function_scope) WHERE (deleted_at IS NULL);


--
-- Name: tenant_relationship_invites_relationship_active_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_relationship_invites_relationship_active_idx ON global.tenant_relationship_invites USING btree (relationship_id) WHERE ((canceled_at IS NULL) AND (consumed_at IS NULL));


--
-- Name: tenant_relationship_invites_relationship_cancelled_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_relationship_invites_relationship_cancelled_idx ON global.tenant_relationship_invites USING btree (relationship_id) WHERE (canceled_at IS NOT NULL);


--
-- Name: tenant_relationships_holding_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_relationships_holding_idx ON global.tenant_relationships USING btree (tenant_id_holding, status);


--
-- Name: tenant_relationships_pending_uq; Type: INDEX; Schema: global; Owner: -
--

CREATE UNIQUE INDEX tenant_relationships_pending_uq ON global.tenant_relationships USING btree (tenant_id_holding, tenant_id_subsidiary, relationship_type) WHERE (status = 'PENDING'::text);


--
-- Name: tenant_relationships_status_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_relationships_status_idx ON global.tenant_relationships USING btree (status);


--
-- Name: tenant_relationships_subsidiary_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_relationships_subsidiary_idx ON global.tenant_relationships USING btree (tenant_id_subsidiary, status);


--
-- Name: tenant_transfers_relationship_id_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_transfers_relationship_id_idx ON global.tenant_transfers USING btree (relationship_id);


--
-- Name: tenant_transfers_requested_by_tenant_id_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_transfers_requested_by_tenant_id_idx ON global.tenant_transfers USING btree (requested_by_tenant_id);


--
-- Name: tenant_transfers_status_idx; Type: INDEX; Schema: global; Owner: -
--

CREATE INDEX tenant_transfers_status_idx ON global.tenant_transfers USING btree (status);


--
-- Name: address_tenant_id_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX address_tenant_id_party_id ON tenants.addresses USING btree (tenant_id, party_id) WHERE (deleted_at IS NULL);


--
-- Name: address_tenant_party_type_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX address_tenant_party_type_idx ON tenants.addresses USING btree (tenant_id, party_id, type) WHERE (deleted_at IS NULL);


--
-- Name: address_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX address_validities_open_uq ON tenants.address_validities USING btree (tenant_id, address_id) WHERE ((end_date IS NULL) AND (deleted_at IS NULL));


--
-- Name: address_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX address_validities_primary_active_uq ON tenants.address_validities USING btree (tenant_id, party_id, type) WHERE (is_primary AND (end_date IS NULL) AND (deleted_at IS NULL));


--
-- Name: address_validities_tenant_party_type_start_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX address_validities_tenant_party_type_start_idx ON tenants.address_validities USING btree (tenant_id, party_id, type, start_date DESC);


--
-- Name: addresses_tenant_city_location_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX addresses_tenant_city_location_idx ON tenants.addresses USING btree (tenant_id, city_location_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_tenant_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_tenant_hash_idx ON ONLY tenants.attached_documents USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p00_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p00_tenant_id_document_hash_idx ON tenants.attached_documents_p00 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_tenant_origin_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_tenant_origin_idx ON ONLY tenants.attached_documents USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p00_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p00_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p00 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p01_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p01_tenant_id_document_hash_idx ON tenants.attached_documents_p01 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p01_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p01_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p01 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p02_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p02_tenant_id_document_hash_idx ON tenants.attached_documents_p02 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p02_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p02_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p02 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p03_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p03_tenant_id_document_hash_idx ON tenants.attached_documents_p03 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p03_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p03_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p03 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p04_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p04_tenant_id_document_hash_idx ON tenants.attached_documents_p04 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p04_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p04_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p04 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p05_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p05_tenant_id_document_hash_idx ON tenants.attached_documents_p05 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p05_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p05_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p05 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p06_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p06_tenant_id_document_hash_idx ON tenants.attached_documents_p06 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p06_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p06_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p06 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p07_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p07_tenant_id_document_hash_idx ON tenants.attached_documents_p07 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p07_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p07_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p07 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p08_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p08_tenant_id_document_hash_idx ON tenants.attached_documents_p08 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p08_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p08_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p08 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p09_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p09_tenant_id_document_hash_idx ON tenants.attached_documents_p09 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p09_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p09_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p09 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p10_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p10_tenant_id_document_hash_idx ON tenants.attached_documents_p10 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p10_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p10_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p10 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p11_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p11_tenant_id_document_hash_idx ON tenants.attached_documents_p11 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p11_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p11_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p11 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p12_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p12_tenant_id_document_hash_idx ON tenants.attached_documents_p12 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p12_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p12_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p12 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p13_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p13_tenant_id_document_hash_idx ON tenants.attached_documents_p13 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p13_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p13_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p13 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p14_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p14_tenant_id_document_hash_idx ON tenants.attached_documents_p14 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p14_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p14_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p14 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p15_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p15_tenant_id_document_hash_idx ON tenants.attached_documents_p15 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p15_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p15_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p15 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p16_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p16_tenant_id_document_hash_idx ON tenants.attached_documents_p16 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p16_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p16_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p16 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p17_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p17_tenant_id_document_hash_idx ON tenants.attached_documents_p17 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p17_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p17_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p17 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p18_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p18_tenant_id_document_hash_idx ON tenants.attached_documents_p18 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p18_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p18_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p18 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p19_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p19_tenant_id_document_hash_idx ON tenants.attached_documents_p19 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p19_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p19_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p19 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p20_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p20_tenant_id_document_hash_idx ON tenants.attached_documents_p20 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p20_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p20_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p20 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p21_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p21_tenant_id_document_hash_idx ON tenants.attached_documents_p21 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p21_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p21_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p21 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p22_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p22_tenant_id_document_hash_idx ON tenants.attached_documents_p22 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p22_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p22_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p22 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p23_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p23_tenant_id_document_hash_idx ON tenants.attached_documents_p23 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p23_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p23_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p23 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p24_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p24_tenant_id_document_hash_idx ON tenants.attached_documents_p24 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p24_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p24_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p24 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p25_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p25_tenant_id_document_hash_idx ON tenants.attached_documents_p25 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p25_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p25_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p25 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p26_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p26_tenant_id_document_hash_idx ON tenants.attached_documents_p26 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p26_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p26_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p26 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p27_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p27_tenant_id_document_hash_idx ON tenants.attached_documents_p27 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p27_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p27_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p27 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p28_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p28_tenant_id_document_hash_idx ON tenants.attached_documents_p28 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p28_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p28_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p28 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p29_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p29_tenant_id_document_hash_idx ON tenants.attached_documents_p29 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p29_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p29_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p29 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p30_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p30_tenant_id_document_hash_idx ON tenants.attached_documents_p30 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p30_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p30_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p30 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p31_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p31_tenant_id_document_hash_idx ON tenants.attached_documents_p31 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p31_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p31_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p31 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p32_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p32_tenant_id_document_hash_idx ON tenants.attached_documents_p32 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p32_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p32_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p32 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p33_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p33_tenant_id_document_hash_idx ON tenants.attached_documents_p33 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p33_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p33_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p33 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p34_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p34_tenant_id_document_hash_idx ON tenants.attached_documents_p34 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p34_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p34_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p34 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p35_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p35_tenant_id_document_hash_idx ON tenants.attached_documents_p35 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p35_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p35_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p35 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p36_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p36_tenant_id_document_hash_idx ON tenants.attached_documents_p36 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p36_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p36_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p36 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p37_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p37_tenant_id_document_hash_idx ON tenants.attached_documents_p37 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p37_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p37_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p37 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p38_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p38_tenant_id_document_hash_idx ON tenants.attached_documents_p38 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p38_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p38_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p38 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p39_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p39_tenant_id_document_hash_idx ON tenants.attached_documents_p39 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p39_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p39_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p39 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p40_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p40_tenant_id_document_hash_idx ON tenants.attached_documents_p40 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p40_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p40_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p40 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p41_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p41_tenant_id_document_hash_idx ON tenants.attached_documents_p41 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p41_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p41_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p41 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p42_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p42_tenant_id_document_hash_idx ON tenants.attached_documents_p42 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p42_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p42_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p42 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p43_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p43_tenant_id_document_hash_idx ON tenants.attached_documents_p43 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p43_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p43_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p43 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p44_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p44_tenant_id_document_hash_idx ON tenants.attached_documents_p44 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p44_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p44_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p44 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p45_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p45_tenant_id_document_hash_idx ON tenants.attached_documents_p45 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p45_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p45_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p45 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p46_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p46_tenant_id_document_hash_idx ON tenants.attached_documents_p46 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p46_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p46_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p46 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p47_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p47_tenant_id_document_hash_idx ON tenants.attached_documents_p47 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p47_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p47_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p47 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p48_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p48_tenant_id_document_hash_idx ON tenants.attached_documents_p48 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p48_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p48_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p48 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p49_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p49_tenant_id_document_hash_idx ON tenants.attached_documents_p49 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p49_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p49_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p49 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p50_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p50_tenant_id_document_hash_idx ON tenants.attached_documents_p50 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p50_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p50_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p50 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p51_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p51_tenant_id_document_hash_idx ON tenants.attached_documents_p51 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p51_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p51_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p51 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p52_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p52_tenant_id_document_hash_idx ON tenants.attached_documents_p52 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p52_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p52_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p52 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p53_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p53_tenant_id_document_hash_idx ON tenants.attached_documents_p53 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p53_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p53_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p53 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p54_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p54_tenant_id_document_hash_idx ON tenants.attached_documents_p54 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p54_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p54_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p54 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p55_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p55_tenant_id_document_hash_idx ON tenants.attached_documents_p55 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p55_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p55_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p55 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p56_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p56_tenant_id_document_hash_idx ON tenants.attached_documents_p56 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p56_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p56_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p56 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p57_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p57_tenant_id_document_hash_idx ON tenants.attached_documents_p57 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p57_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p57_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p57 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p58_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p58_tenant_id_document_hash_idx ON tenants.attached_documents_p58 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p58_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p58_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p58 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p59_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p59_tenant_id_document_hash_idx ON tenants.attached_documents_p59 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p59_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p59_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p59 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p60_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p60_tenant_id_document_hash_idx ON tenants.attached_documents_p60 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p60_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p60_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p60 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p61_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p61_tenant_id_document_hash_idx ON tenants.attached_documents_p61 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p61_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p61_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p61 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p62_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p62_tenant_id_document_hash_idx ON tenants.attached_documents_p62 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p62_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p62_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p62 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p63_tenant_id_document_hash_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p63_tenant_id_document_hash_idx ON tenants.attached_documents_p63 USING btree (tenant_id, document_hash) WHERE (deleted_at IS NULL);


--
-- Name: attached_documents_p63_tenant_id_origin_table_origin_id_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX attached_documents_p63_tenant_id_origin_table_origin_id_idx ON tenants.attached_documents_p63 USING btree (tenant_id, origin_table, origin_id) WHERE (deleted_at IS NULL);


--
-- Name: corporate_groups_tenant_active_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX corporate_groups_tenant_active_idx ON tenants.corporate_groups USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: corporate_groups_tenant_name_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX corporate_groups_tenant_name_uq ON tenants.corporate_groups USING btree (tenant_id, name) WHERE (deleted_at IS NULL);


--
-- Name: department_address_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX department_address_validities_open_uq ON tenants.department_address_validities USING btree (tenant_id, department_address_id) WHERE (end_date IS NULL);


--
-- Name: department_address_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX department_address_validities_primary_active_uq ON tenants.department_address_validities USING btree (tenant_id, department_id, type) WHERE (is_primary AND (end_date IS NULL));


--
-- Name: department_address_validities_tenant_department_start_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_address_validities_tenant_department_start_idx ON tenants.department_address_validities USING btree (tenant_id, department_id, start_date DESC);


--
-- Name: department_addresses_tenant_city_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_addresses_tenant_city_idx ON tenants.department_addresses USING btree (tenant_id, city_location_id) WHERE (deleted_at IS NULL);


--
-- Name: department_addresses_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_addresses_tenant_department_idx ON tenants.department_addresses USING btree (tenant_id, department_id) WHERE (deleted_at IS NULL);


--
-- Name: department_contact_mechanism_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX department_contact_mechanism_validities_open_uq ON tenants.department_contact_mechanism_validities USING btree (tenant_id, department_contact_mechanism_id) WHERE (end_date IS NULL);


--
-- Name: department_contact_mechanism_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX department_contact_mechanism_validities_primary_active_uq ON tenants.department_contact_mechanism_validities USING btree (tenant_id, department_id, contact_type) WHERE (is_primary AND (end_date IS NULL));


--
-- Name: department_contact_mechanism_validities_tenant_department_start; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_contact_mechanism_validities_tenant_department_start ON tenants.department_contact_mechanism_validities USING btree (tenant_id, department_id, start_date DESC);


--
-- Name: department_contact_mechanisms_tenant_contact_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_contact_mechanisms_tenant_contact_idx ON tenants.department_contact_mechanisms USING btree (tenant_id, contact_mechanism_id) WHERE (deleted_at IS NULL);


--
-- Name: department_contact_mechanisms_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_contact_mechanisms_tenant_department_idx ON tenants.department_contact_mechanisms USING btree (tenant_id, department_id) WHERE (deleted_at IS NULL);


--
-- Name: department_group_assignments_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_group_assignments_tenant_department_idx ON tenants.department_group_assignments USING btree (tenant_id, department_id);


--
-- Name: department_group_assignments_tenant_group_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_group_assignments_tenant_group_idx ON tenants.department_group_assignments USING btree (tenant_id, corporate_group_id);


--
-- Name: department_menu_option_validities_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_menu_option_validities_tenant_department_idx ON tenants.department_menu_option_validities USING btree (tenant_id, department_id);


--
-- Name: department_menu_option_validities_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_menu_option_validities_tenant_option_idx ON tenants.department_menu_option_validities USING btree (tenant_id, menu_option_id);


--
-- Name: department_menu_options_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_menu_options_tenant_department_idx ON tenants.department_menu_options USING btree (tenant_id, department_id);


--
-- Name: department_menu_options_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_menu_options_tenant_option_idx ON tenants.department_menu_options USING btree (tenant_id, menu_option_id);


--
-- Name: department_tenant_id_code; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX department_tenant_id_code ON tenants.departments USING btree (tenant_id, code) WHERE (deleted_at IS NULL);


--
-- Name: department_tenant_id_manager_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_tenant_id_manager_party_id ON tenants.departments USING btree (tenant_id, manager_party_id) WHERE (deleted_at IS NULL);


--
-- Name: department_tenant_id_name; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_tenant_id_name ON tenants.departments USING btree (tenant_id, name) WHERE (deleted_at IS NULL);


--
-- Name: department_tenant_id_parent_department_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_tenant_id_parent_department_id ON tenants.departments USING btree (tenant_id, parent_department_id) WHERE (deleted_at IS NULL);


--
-- Name: department_translations_locale_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_translations_locale_idx ON tenants.department_translations USING btree (locale_code);


--
-- Name: department_translations_tenant_department_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX department_translations_tenant_department_idx ON tenants.department_translations USING btree (tenant_id, department_id);


--
-- Name: employee_menu_option_overrides_tenant_employee_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employee_menu_option_overrides_tenant_employee_idx ON tenants.employee_menu_option_overrides USING btree (tenant_id, employee_party_id);


--
-- Name: employee_menu_option_overrides_tenant_function_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employee_menu_option_overrides_tenant_function_idx ON tenants.employee_menu_option_overrides USING btree (tenant_id, system_function_id);


--
-- Name: employee_tenant_id_department_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employee_tenant_id_department_id ON tenants.employees USING btree (tenant_id, department_id) WHERE (deleted_at IS NULL);


--
-- Name: employee_tenant_id_employee_code; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX employee_tenant_id_employee_code ON tenants.employees USING btree (tenant_id, employee_code) WHERE (deleted_at IS NULL);


--
-- Name: employee_tenant_id_job_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employee_tenant_id_job_id ON tenants.employees USING btree (tenant_id, job_id) WHERE (deleted_at IS NULL);


--
-- Name: employee_tenant_id_manager_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employee_tenant_id_manager_party_id ON tenants.employees USING btree (tenant_id, manager_party_id) WHERE (deleted_at IS NULL);


--
-- Name: employeeassignment_tenant_id_department_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employeeassignment_tenant_id_department_id ON tenants.employee_assignments USING btree (tenant_id, department_id);


--
-- Name: employeeassignment_tenant_id_employee_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX employeeassignment_tenant_id_employee_party_id ON tenants.employee_assignments USING btree (tenant_id, employee_party_id) WHERE (end_date IS NULL);


--
-- Name: employeeassignment_tenant_id_employee_party_id_start_date; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employeeassignment_tenant_id_employee_party_id_start_date ON tenants.employee_assignments USING btree (tenant_id, employee_party_id, start_date);


--
-- Name: employeeassignment_tenant_id_job_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX employeeassignment_tenant_id_job_id ON tenants.employee_assignments USING btree (tenant_id, job_id);


--
-- Name: job_tenant_id_code; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX job_tenant_id_code ON tenants.jobs USING btree (tenant_id, code) WHERE (deleted_at IS NULL);


--
-- Name: job_tenant_id_job_family_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX job_tenant_id_job_family_id ON tenants.jobs USING btree (tenant_id, job_family_id) WHERE (deleted_at IS NULL);


--
-- Name: job_tenant_id_title; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX job_tenant_id_title ON tenants.jobs USING btree (tenant_id, title) WHERE (deleted_at IS NULL);


--
-- Name: jobfamily_tenant_id_code; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX jobfamily_tenant_id_code ON tenants.job_families USING btree (tenant_id, code) WHERE (deleted_at IS NULL);


--
-- Name: jobfamily_tenant_id_name; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX jobfamily_tenant_id_name ON tenants.job_families USING btree (tenant_id, name) WHERE (deleted_at IS NULL);


--
-- Name: menu_categories_tenant_code_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX menu_categories_tenant_code_uq ON tenants.menu_categories USING btree (tenant_id, code) WHERE (deleted_at IS NULL);


--
-- Name: menu_categories_tenant_parent_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_categories_tenant_parent_idx ON tenants.menu_categories USING btree (tenant_id, parent_id) WHERE (deleted_at IS NULL);


--
-- Name: menu_categories_tenant_sort_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_categories_tenant_sort_idx ON tenants.menu_categories USING btree (tenant_id, sort_order) WHERE (deleted_at IS NULL);


--
-- Name: menu_option_categories_tenant_category_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_option_categories_tenant_category_idx ON tenants.menu_option_categories USING btree (tenant_id, menu_category_id);


--
-- Name: menu_option_categories_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_option_categories_tenant_option_idx ON tenants.menu_option_categories USING btree (tenant_id, menu_option_id);


--
-- Name: menu_option_functions_tenant_function_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_option_functions_tenant_function_idx ON tenants.menu_option_functions USING btree (tenant_id, system_function_id);


--
-- Name: menu_option_functions_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_option_functions_tenant_option_idx ON tenants.menu_option_functions USING btree (tenant_id, menu_option_id);


--
-- Name: menu_options_tenant_code_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX menu_options_tenant_code_uq ON tenants.menu_options USING btree (tenant_id, code) WHERE (deleted_at IS NULL);


--
-- Name: menu_options_tenant_name_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_options_tenant_name_idx ON tenants.menu_options USING btree (tenant_id, name) WHERE (deleted_at IS NULL);


--
-- Name: menu_options_tenant_template_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX menu_options_tenant_template_idx ON tenants.menu_options USING btree (tenant_id, template_id) WHERE (deleted_at IS NULL);


--
-- Name: organization_profiles_party_id_key; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX organization_profiles_party_id_key ON tenants.organization_profiles USING btree (party_id);


--
-- Name: organizationprofile_tenant_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX organizationprofile_tenant_id ON tenants.organization_profiles USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: organizationprofile_tenant_id_tax_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX organizationprofile_tenant_id_tax_id ON tenants.organization_profiles USING btree (tenant_id, tax_id) WHERE (deleted_at IS NULL);


--
-- Name: party_contact_mechanism_validities_active_contact_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_contact_mechanism_validities_active_contact_uq ON tenants.party_contact_mechanism_validities USING btree (tenant_id, contact_mechanism_id) WHERE (end_date IS NULL);


--
-- Name: party_contact_mechanism_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_contact_mechanism_validities_open_uq ON tenants.party_contact_mechanism_validities USING btree (tenant_id, party_contact_mechanism_id) WHERE (end_date IS NULL);


--
-- Name: party_contact_mechanism_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_contact_mechanism_validities_primary_active_uq ON tenants.party_contact_mechanism_validities USING btree (tenant_id, party_id, contact_type) WHERE (is_primary AND (end_date IS NULL));


--
-- Name: party_contact_mechanism_validities_tenant_relation_start_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_contact_mechanism_validities_tenant_relation_start_idx ON tenants.party_contact_mechanism_validities USING btree (tenant_id, party_contact_mechanism_id, start_date DESC);


--
-- Name: party_identity_document_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_identity_document_validities_open_uq ON tenants.party_identity_document_validities USING btree (tenant_id, party_identity_document_id) WHERE (end_date IS NULL);


--
-- Name: party_identity_document_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_identity_document_validities_primary_active_uq ON tenants.party_identity_document_validities USING btree (tenant_id, party_id) WHERE (is_primary AND (end_date IS NULL));


--
-- Name: party_identity_document_validities_tenant_document_start_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_identity_document_validities_tenant_document_start_idx ON tenants.party_identity_document_validities USING btree (tenant_id, party_identity_document_id, start_date DESC);


--
-- Name: party_identity_documents_tenant_country_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_identity_documents_tenant_country_idx ON tenants.party_identity_documents USING btree (tenant_id, issuing_country_code) WHERE (deleted_at IS NULL);


--
-- Name: party_identity_documents_tenant_jurisdiction_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_identity_documents_tenant_jurisdiction_idx ON tenants.party_identity_documents USING btree (tenant_id, issuing_jurisdiction_id) WHERE (deleted_at IS NULL);


--
-- Name: party_identity_documents_tenant_party_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_identity_documents_tenant_party_idx ON tenants.party_identity_documents USING btree (tenant_id, party_id) WHERE (deleted_at IS NULL);


--
-- Name: party_identity_documents_tenant_type_country_number_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_identity_documents_tenant_type_country_number_uq ON tenants.party_identity_documents USING btree (tenant_id, document_type_id, issuing_country_code, document_number) WHERE (deleted_at IS NULL);


--
-- Name: party_identity_documents_tenant_type_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_identity_documents_tenant_type_idx ON tenants.party_identity_documents USING btree (tenant_id, document_type_id) WHERE (deleted_at IS NULL);


--
-- Name: party_merge_log_source_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_merge_log_source_idx ON tenants.party_merge_log USING btree (tenant_id, source_party_id);


--
-- Name: party_merge_log_target_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_merge_log_target_idx ON tenants.party_merge_log USING btree (tenant_id, target_party_id);


--
-- Name: party_merge_log_tenant_merged_at_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_merge_log_tenant_merged_at_idx ON tenants.party_merge_log USING btree (tenant_id, merged_at);


--
-- Name: party_name_validities_open_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_name_validities_open_uq ON tenants.party_name_validities USING btree (tenant_id, party_name_id) WHERE (end_date IS NULL);


--
-- Name: party_name_validities_primary_active_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_name_validities_primary_active_uq ON tenants.party_name_validities USING btree (tenant_id, party_id, name_type_id) WHERE (is_primary AND (end_date IS NULL));


--
-- Name: party_name_validities_tenant_name_start_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_name_validities_tenant_name_start_idx ON tenants.party_name_validities USING btree (tenant_id, party_name_id, start_date DESC);


--
-- Name: party_names_tenant_party_type_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_names_tenant_party_type_idx ON tenants.party_names USING btree (tenant_id, party_id, name_type_id) WHERE (deleted_at IS NULL);


--
-- Name: party_names_tenant_party_type_value_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_names_tenant_party_type_value_uq ON tenants.party_names USING btree (tenant_id, party_id, name_type_id, value) WHERE (deleted_at IS NULL);


--
-- Name: party_nationalities_primary_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_nationalities_primary_uq ON tenants.party_nationalities USING btree (tenant_id, party_id) WHERE is_primary;


--
-- Name: party_nationalities_tenant_party_country_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX party_nationalities_tenant_party_country_uq ON tenants.party_nationalities USING btree (tenant_id, party_id, country_code);


--
-- Name: party_nationalities_tenant_party_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_nationalities_tenant_party_idx ON tenants.party_nationalities USING btree (tenant_id, party_id);


--
-- Name: party_tenant_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_tenant_id ON tenants.parties USING btree (tenant_id);


--
-- Name: party_tenant_id_canonical_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_tenant_id_canonical_party_id ON tenants.parties USING btree (tenant_id, canonical_party_id);


--
-- Name: party_tenant_id_party_type; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX party_tenant_id_party_type ON tenants.parties USING btree (tenant_id, party_type) WHERE (deleted_at IS NULL);


--
-- Name: partycontactmechanism_tenant_id_party_id_contact_mechanism_orde; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX partycontactmechanism_tenant_id_party_id_contact_mechanism_orde ON tenants.party_contact_mechanisms USING btree (tenant_id, party_id, contact_mechanism_order);


--
-- Name: partymergelog_tenant_id_merged_at; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX partymergelog_tenant_id_merged_at ON tenants.party_merge_log USING btree (tenant_id, merged_at);


--
-- Name: partymergelog_tenant_id_source_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX partymergelog_tenant_id_source_party_id ON tenants.party_merge_log USING btree (tenant_id, source_party_id);


--
-- Name: partymergelog_tenant_id_target_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX partymergelog_tenant_id_target_party_id ON tenants.party_merge_log USING btree (tenant_id, target_party_id);


--
-- Name: partyrole_tenant_id_party_id_role_type_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX partyrole_tenant_id_party_id_role_type_id ON tenants.party_roles USING btree (tenant_id, party_id, role_type_id) WHERE (deleted_at IS NULL);


--
-- Name: partyrole_tenant_id_role_type_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX partyrole_tenant_id_role_type_id ON tenants.party_roles USING btree (tenant_id, role_type_id) WHERE (deleted_at IS NULL);


--
-- Name: password_reset_tokens_token_hash_key; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX password_reset_tokens_token_hash_key ON tenants.password_reset_tokens USING btree (token_hash);


--
-- Name: passwordresettoken_tenant_id_expires_at; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX passwordresettoken_tenant_id_expires_at ON tenants.password_reset_tokens USING btree (tenant_id, expires_at);


--
-- Name: passwordresettoken_tenant_id_party_id_expires_at; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX passwordresettoken_tenant_id_party_id_expires_at ON tenants.password_reset_tokens USING btree (tenant_id, party_id, expires_at);


--
-- Name: person_profiles_party_id_key; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX person_profiles_party_id_key ON tenants.person_profiles USING btree (party_id);


--
-- Name: personprofile_tenant_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX personprofile_tenant_id ON tenants.person_profiles USING btree (tenant_id) WHERE (deleted_at IS NULL);


--
-- Name: position_menu_option_validities_tenant_job_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX position_menu_option_validities_tenant_job_idx ON tenants.position_menu_option_validities USING btree (tenant_id, job_id);


--
-- Name: position_menu_option_validities_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX position_menu_option_validities_tenant_option_idx ON tenants.position_menu_option_validities USING btree (tenant_id, menu_option_id);


--
-- Name: position_menu_options_tenant_job_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX position_menu_options_tenant_job_idx ON tenants.position_menu_options USING btree (tenant_id, job_id);


--
-- Name: position_menu_options_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX position_menu_options_tenant_option_idx ON tenants.position_menu_options USING btree (tenant_id, menu_option_id);


--
-- Name: role_menu_options_tenant_option_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX role_menu_options_tenant_option_idx ON tenants.role_menu_options USING btree (tenant_id, menu_option_id);


--
-- Name: role_menu_options_tenant_role_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX role_menu_options_tenant_role_idx ON tenants.role_menu_options USING btree (tenant_id, tenant_role_id);


--
-- Name: session_tenant_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX session_tenant_id ON tenants.sessions USING btree (tenant_id);


--
-- Name: session_tenant_id_user_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX session_tenant_id_user_id ON tenants.sessions USING btree (tenant_id, user_id) WHERE ((revoked_at IS NULL) AND (deleted_at IS NULL));


--
-- Name: sessions_active_user_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX sessions_active_user_idx ON tenants.sessions USING btree (tenant_id, user_id) WHERE ((revoked_at IS NULL) AND (deleted_at IS NULL));


--
-- Name: sessions_expiry_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX sessions_expiry_idx ON tenants.sessions USING btree (tenant_id, expires_at) WHERE ((revoked_at IS NULL) AND (deleted_at IS NULL));


--
-- Name: sessions_tenant_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX sessions_tenant_idx ON tenants.sessions USING btree (tenant_id);


--
-- Name: sessions_token_hash_key; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX sessions_token_hash_key ON tenants.sessions USING btree (token_hash);


--
-- Name: tenant_roles_tenant_application_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX tenant_roles_tenant_application_idx ON tenants.tenant_roles USING btree (tenant_id, client_application_id) WHERE (deleted_at IS NULL);


--
-- Name: tenant_roles_tenant_name_uq; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX tenant_roles_tenant_name_uq ON tenants.tenant_roles USING btree (tenant_id, name) WHERE (deleted_at IS NULL);


--
-- Name: user_role_assignments_tenant_role_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX user_role_assignments_tenant_role_idx ON tenants.user_role_assignments USING btree (tenant_id, tenant_role_id);


--
-- Name: user_role_assignments_tenant_user_idx; Type: INDEX; Schema: tenants; Owner: -
--

CREATE INDEX user_role_assignments_tenant_user_idx ON tenants.user_role_assignments USING btree (tenant_id, user_credentials_id);


--
-- Name: usercredentials_party_contact_mechanism_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX usercredentials_party_contact_mechanism_id ON tenants.user_credentials USING btree (party_contact_mechanism_id) WHERE (deleted_at IS NULL);


--
-- Name: usercredentials_tenant_id_party_id; Type: INDEX; Schema: tenants; Owner: -
--

CREATE UNIQUE INDEX usercredentials_tenant_id_party_id ON tenants.user_credentials USING btree (tenant_id, party_id) WHERE (deleted_at IS NULL);


--
-- Name: change_log_2026_11_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2026_11_changed_at_idx;


--
-- Name: change_log_2026_11_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2026_11_changed_by_changed_at_idx;


--
-- Name: change_log_2026_11_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2026_11_changed_fields_idx;


--
-- Name: change_log_2026_11_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2026_11_pkey;


--
-- Name: change_log_2026_11_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2026_11_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2026_11_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2026_11_tenant_id_changed_at_idx;


--
-- Name: change_log_2026_12_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2026_12_changed_at_idx;


--
-- Name: change_log_2026_12_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2026_12_changed_by_changed_at_idx;


--
-- Name: change_log_2026_12_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2026_12_changed_fields_idx;


--
-- Name: change_log_2026_12_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2026_12_pkey;


--
-- Name: change_log_2026_12_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2026_12_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2026_12_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2026_12_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_01_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_01_changed_at_idx;


--
-- Name: change_log_2027_01_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_01_changed_by_changed_at_idx;


--
-- Name: change_log_2027_01_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_01_changed_fields_idx;


--
-- Name: change_log_2027_01_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_01_pkey;


--
-- Name: change_log_2027_01_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_01_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_01_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_01_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_02_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_02_changed_at_idx;


--
-- Name: change_log_2027_02_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_02_changed_by_changed_at_idx;


--
-- Name: change_log_2027_02_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_02_changed_fields_idx;


--
-- Name: change_log_2027_02_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_02_pkey;


--
-- Name: change_log_2027_02_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_02_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_02_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_02_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_03_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_03_changed_at_idx;


--
-- Name: change_log_2027_03_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_03_changed_by_changed_at_idx;


--
-- Name: change_log_2027_03_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_03_changed_fields_idx;


--
-- Name: change_log_2027_03_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_03_pkey;


--
-- Name: change_log_2027_03_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_03_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_03_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_03_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_04_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_04_changed_at_idx;


--
-- Name: change_log_2027_04_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_04_changed_by_changed_at_idx;


--
-- Name: change_log_2027_04_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_04_changed_fields_idx;


--
-- Name: change_log_2027_04_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_04_pkey;


--
-- Name: change_log_2027_04_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_04_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_04_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_04_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_05_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_05_changed_at_idx;


--
-- Name: change_log_2027_05_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_05_changed_by_changed_at_idx;


--
-- Name: change_log_2027_05_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_05_changed_fields_idx;


--
-- Name: change_log_2027_05_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_05_pkey;


--
-- Name: change_log_2027_05_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_05_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_05_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_05_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_06_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_06_changed_at_idx;


--
-- Name: change_log_2027_06_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_06_changed_by_changed_at_idx;


--
-- Name: change_log_2027_06_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_06_changed_fields_idx;


--
-- Name: change_log_2027_06_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_06_pkey;


--
-- Name: change_log_2027_06_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_06_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_06_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_06_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_07_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_07_changed_at_idx;


--
-- Name: change_log_2027_07_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_07_changed_by_changed_at_idx;


--
-- Name: change_log_2027_07_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_07_changed_fields_idx;


--
-- Name: change_log_2027_07_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_07_pkey;


--
-- Name: change_log_2027_07_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_07_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_07_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_07_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_08_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_08_changed_at_idx;


--
-- Name: change_log_2027_08_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_08_changed_by_changed_at_idx;


--
-- Name: change_log_2027_08_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_08_changed_fields_idx;


--
-- Name: change_log_2027_08_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_08_pkey;


--
-- Name: change_log_2027_08_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_08_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_08_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_08_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_09_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_09_changed_at_idx;


--
-- Name: change_log_2027_09_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_09_changed_by_changed_at_idx;


--
-- Name: change_log_2027_09_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_09_changed_fields_idx;


--
-- Name: change_log_2027_09_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_09_pkey;


--
-- Name: change_log_2027_09_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_09_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_09_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_09_tenant_id_changed_at_idx;


--
-- Name: change_log_2027_10_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_2027_10_changed_at_idx;


--
-- Name: change_log_2027_10_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_2027_10_changed_by_changed_at_idx;


--
-- Name: change_log_2027_10_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_2027_10_changed_fields_idx;


--
-- Name: change_log_2027_10_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_2027_10_pkey;


--
-- Name: change_log_2027_10_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_2027_10_table_name_record_id_changed_at_idx;


--
-- Name: change_log_2027_10_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_2027_10_tenant_id_changed_at_idx;


--
-- Name: change_log_default_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_at_idx ATTACH PARTITION audit.change_log_default_changed_at_idx;


--
-- Name: change_log_default_changed_by_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_by_idx ATTACH PARTITION audit.change_log_default_changed_by_changed_at_idx;


--
-- Name: change_log_default_changed_fields_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_changed_fields_gin ATTACH PARTITION audit.change_log_default_changed_fields_idx;


--
-- Name: change_log_default_pkey; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_pkey ATTACH PARTITION audit.change_log_default_pkey;


--
-- Name: change_log_default_table_name_record_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_table_record_idx ATTACH PARTITION audit.change_log_default_table_name_record_id_changed_at_idx;


--
-- Name: change_log_default_tenant_id_changed_at_idx; Type: INDEX ATTACH; Schema: audit; Owner: -
--

ALTER INDEX audit.change_log_tenant_changed_idx ATTACH PARTITION audit.change_log_default_tenant_id_changed_at_idx;


--
-- Name: attached_documents_p00_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p00_pkey;


--
-- Name: attached_documents_p00_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p00_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p00_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p00_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p01_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p01_pkey;


--
-- Name: attached_documents_p01_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p01_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p01_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p01_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p02_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p02_pkey;


--
-- Name: attached_documents_p02_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p02_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p02_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p02_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p03_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p03_pkey;


--
-- Name: attached_documents_p03_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p03_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p03_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p03_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p04_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p04_pkey;


--
-- Name: attached_documents_p04_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p04_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p04_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p04_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p05_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p05_pkey;


--
-- Name: attached_documents_p05_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p05_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p05_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p05_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p06_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p06_pkey;


--
-- Name: attached_documents_p06_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p06_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p06_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p06_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p07_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p07_pkey;


--
-- Name: attached_documents_p07_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p07_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p07_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p07_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p08_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p08_pkey;


--
-- Name: attached_documents_p08_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p08_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p08_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p08_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p09_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p09_pkey;


--
-- Name: attached_documents_p09_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p09_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p09_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p09_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p10_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p10_pkey;


--
-- Name: attached_documents_p10_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p10_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p10_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p10_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p11_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p11_pkey;


--
-- Name: attached_documents_p11_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p11_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p11_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p11_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p12_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p12_pkey;


--
-- Name: attached_documents_p12_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p12_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p12_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p12_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p13_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p13_pkey;


--
-- Name: attached_documents_p13_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p13_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p13_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p13_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p14_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p14_pkey;


--
-- Name: attached_documents_p14_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p14_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p14_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p14_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p15_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p15_pkey;


--
-- Name: attached_documents_p15_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p15_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p15_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p15_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p16_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p16_pkey;


--
-- Name: attached_documents_p16_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p16_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p16_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p16_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p17_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p17_pkey;


--
-- Name: attached_documents_p17_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p17_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p17_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p17_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p18_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p18_pkey;


--
-- Name: attached_documents_p18_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p18_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p18_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p18_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p19_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p19_pkey;


--
-- Name: attached_documents_p19_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p19_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p19_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p19_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p20_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p20_pkey;


--
-- Name: attached_documents_p20_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p20_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p20_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p20_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p21_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p21_pkey;


--
-- Name: attached_documents_p21_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p21_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p21_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p21_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p22_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p22_pkey;


--
-- Name: attached_documents_p22_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p22_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p22_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p22_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p23_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p23_pkey;


--
-- Name: attached_documents_p23_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p23_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p23_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p23_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p24_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p24_pkey;


--
-- Name: attached_documents_p24_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p24_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p24_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p24_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p25_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p25_pkey;


--
-- Name: attached_documents_p25_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p25_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p25_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p25_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p26_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p26_pkey;


--
-- Name: attached_documents_p26_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p26_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p26_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p26_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p27_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p27_pkey;


--
-- Name: attached_documents_p27_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p27_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p27_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p27_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p28_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p28_pkey;


--
-- Name: attached_documents_p28_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p28_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p28_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p28_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p29_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p29_pkey;


--
-- Name: attached_documents_p29_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p29_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p29_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p29_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p30_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p30_pkey;


--
-- Name: attached_documents_p30_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p30_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p30_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p30_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p31_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p31_pkey;


--
-- Name: attached_documents_p31_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p31_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p31_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p31_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p32_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p32_pkey;


--
-- Name: attached_documents_p32_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p32_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p32_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p32_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p33_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p33_pkey;


--
-- Name: attached_documents_p33_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p33_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p33_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p33_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p34_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p34_pkey;


--
-- Name: attached_documents_p34_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p34_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p34_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p34_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p35_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p35_pkey;


--
-- Name: attached_documents_p35_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p35_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p35_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p35_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p36_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p36_pkey;


--
-- Name: attached_documents_p36_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p36_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p36_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p36_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p37_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p37_pkey;


--
-- Name: attached_documents_p37_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p37_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p37_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p37_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p38_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p38_pkey;


--
-- Name: attached_documents_p38_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p38_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p38_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p38_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p39_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p39_pkey;


--
-- Name: attached_documents_p39_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p39_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p39_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p39_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p40_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p40_pkey;


--
-- Name: attached_documents_p40_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p40_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p40_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p40_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p41_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p41_pkey;


--
-- Name: attached_documents_p41_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p41_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p41_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p41_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p42_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p42_pkey;


--
-- Name: attached_documents_p42_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p42_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p42_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p42_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p43_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p43_pkey;


--
-- Name: attached_documents_p43_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p43_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p43_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p43_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p44_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p44_pkey;


--
-- Name: attached_documents_p44_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p44_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p44_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p44_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p45_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p45_pkey;


--
-- Name: attached_documents_p45_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p45_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p45_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p45_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p46_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p46_pkey;


--
-- Name: attached_documents_p46_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p46_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p46_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p46_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p47_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p47_pkey;


--
-- Name: attached_documents_p47_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p47_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p47_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p47_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p48_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p48_pkey;


--
-- Name: attached_documents_p48_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p48_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p48_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p48_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p49_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p49_pkey;


--
-- Name: attached_documents_p49_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p49_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p49_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p49_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p50_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p50_pkey;


--
-- Name: attached_documents_p50_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p50_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p50_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p50_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p51_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p51_pkey;


--
-- Name: attached_documents_p51_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p51_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p51_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p51_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p52_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p52_pkey;


--
-- Name: attached_documents_p52_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p52_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p52_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p52_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p53_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p53_pkey;


--
-- Name: attached_documents_p53_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p53_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p53_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p53_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p54_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p54_pkey;


--
-- Name: attached_documents_p54_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p54_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p54_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p54_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p55_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p55_pkey;


--
-- Name: attached_documents_p55_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p55_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p55_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p55_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p56_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p56_pkey;


--
-- Name: attached_documents_p56_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p56_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p56_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p56_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p57_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p57_pkey;


--
-- Name: attached_documents_p57_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p57_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p57_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p57_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p58_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p58_pkey;


--
-- Name: attached_documents_p58_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p58_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p58_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p58_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p59_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p59_pkey;


--
-- Name: attached_documents_p59_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p59_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p59_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p59_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p60_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p60_pkey;


--
-- Name: attached_documents_p60_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p60_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p60_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p60_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p61_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p61_pkey;


--
-- Name: attached_documents_p61_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p61_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p61_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p61_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p62_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p62_pkey;


--
-- Name: attached_documents_p62_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p62_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p62_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p62_tenant_id_origin_table_origin_id_idx;


--
-- Name: attached_documents_p63_pkey; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_pkey ATTACH PARTITION tenants.attached_documents_p63_pkey;


--
-- Name: attached_documents_p63_tenant_id_document_hash_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_hash_idx ATTACH PARTITION tenants.attached_documents_p63_tenant_id_document_hash_idx;


--
-- Name: attached_documents_p63_tenant_id_origin_table_origin_id_idx; Type: INDEX ATTACH; Schema: tenants; Owner: -
--

ALTER INDEX tenants.attached_documents_tenant_origin_idx ATTACH PARTITION tenants.attached_documents_p63_tenant_id_origin_table_origin_id_idx;


--
-- Name: change_log change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2026_11 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2026_11 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2026_12 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2026_12 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_01 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_01 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_02 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_02 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_03 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_03 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_04 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_04 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_05 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_05 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_06 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_06 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_07 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_07 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_08 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_08 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_09 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_09 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_2027_10 change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_2027_10 FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log_default change_log_deny_truncate; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_truncate BEFORE TRUNCATE ON audit.change_log_default FOR EACH STATEMENT EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: change_log change_log_deny_update_delete; Type: TRIGGER; Schema: audit; Owner: -
--

CREATE TRIGGER change_log_deny_update_delete BEFORE DELETE OR UPDATE ON audit.change_log FOR EACH ROW EXECUTE FUNCTION audit.fn_deny_modification();


--
-- Name: tenant_relationship_invites tenant_relationship_invites_audit_trg; Type: TRIGGER; Schema: global; Owner: -
--

CREATE TRIGGER tenant_relationship_invites_audit_trg AFTER INSERT OR DELETE OR UPDATE ON global.tenant_relationship_invites FOR EACH ROW EXECUTE FUNCTION audit.fn_log_tenant_relationship_invite('token_hash');


--
-- Name: tenant_relationships tenant_relationships_audit_trg; Type: TRIGGER; Schema: global; Owner: -
--

CREATE TRIGGER tenant_relationships_audit_trg AFTER INSERT OR DELETE OR UPDATE ON global.tenant_relationships FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: tenant_transfers tenant_transfers_audit_trg; Type: TRIGGER; Schema: global; Owner: -
--

CREATE TRIGGER tenant_transfers_audit_trg AFTER INSERT OR DELETE OR UPDATE ON global.tenant_transfers FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: address_validities address_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER address_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.address_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: addresses addresses_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER addresses_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.addresses FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: attached_documents attached_documents_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER attached_documents_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.attached_documents FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: corporate_groups corporate_groups_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER corporate_groups_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.corporate_groups FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_address_validities department_address_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_address_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_address_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_addresses department_addresses_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_addresses_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_addresses FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_contact_mechanism_validities department_contact_mechanism_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_contact_mechanism_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_contact_mechanism_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_contact_mechanisms department_contact_mechanisms_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_contact_mechanisms_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_contact_mechanisms FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_group_assignments department_group_assignments_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_group_assignments_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_group_assignments FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_menu_option_validities department_menu_option_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_menu_option_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_menu_option_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_menu_options department_menu_options_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_menu_options_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_menu_options FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: department_translations department_translations_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER department_translations_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.department_translations FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: departments departments_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER departments_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.departments FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: employee_assignments employee_assignments_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER employee_assignments_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.employee_assignments FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER employee_menu_option_overrides_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.employee_menu_option_overrides FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: employees employees_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER employees_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.employees FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: job_families job_families_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER job_families_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.job_families FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: jobs jobs_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER jobs_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.jobs FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: menu_categories menu_categories_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER menu_categories_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.menu_categories FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: menu_option_categories menu_option_categories_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER menu_option_categories_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.menu_option_categories FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: menu_option_functions menu_option_functions_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER menu_option_functions_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.menu_option_functions FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: menu_options menu_options_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER menu_options_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.menu_options FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: organization_profiles organization_profiles_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER organization_profiles_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.organization_profiles FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: parties parties_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER parties_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.parties FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_contact_mechanism_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_contact_mechanism_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_contact_mechanisms party_contact_mechanisms_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_contact_mechanisms_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_contact_mechanisms FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_identity_document_validities party_identity_document_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_identity_document_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_identity_document_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_identity_documents party_identity_documents_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_identity_documents_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_identity_documents FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_name_validities party_name_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_name_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_name_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_names party_names_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_names_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_names FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_nationalities party_nationalities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_nationalities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_nationalities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: party_roles party_roles_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER party_roles_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.party_roles FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: password_reset_tokens password_reset_tokens_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER password_reset_tokens_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.password_reset_tokens FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change('token_hash');


--
-- Name: person_profiles person_profiles_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER person_profiles_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.person_profiles FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: position_menu_option_validities position_menu_option_validities_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER position_menu_option_validities_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.position_menu_option_validities FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: position_menu_options position_menu_options_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER position_menu_options_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.position_menu_options FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: role_menu_options role_menu_options_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER role_menu_options_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.role_menu_options FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: sessions sessions_set_updated_at_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER sessions_set_updated_at_trg BEFORE UPDATE ON tenants.sessions FOR EACH ROW EXECUTE FUNCTION tenants.fn_sessions_set_updated_at();


--
-- Name: tenant_roles tenant_roles_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER tenant_roles_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.tenant_roles FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: user_credentials user_credentials_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER user_credentials_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.user_credentials FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change('password_hash', 'totp_secret', 'backup_codes_hash');


--
-- Name: user_role_assignments user_role_assignments_audit_trg; Type: TRIGGER; Schema: tenants; Owner: -
--

CREATE TRIGGER user_role_assignments_audit_trg AFTER INSERT OR DELETE OR UPDATE ON tenants.user_role_assignments FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change();


--
-- Name: client_applications client_applications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.client_applications
    ADD CONSTRAINT client_applications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: identity_document_type_translations identity_document_type_translati_identity_document_type_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_type_translations
    ADD CONSTRAINT identity_document_type_translati_identity_document_type_id_fkey FOREIGN KEY (identity_document_type_id) REFERENCES global.identity_document_types(id) ON DELETE RESTRICT;


--
-- Name: identity_document_type_translations identity_document_type_translations_locale_code_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.identity_document_type_translations
    ADD CONSTRAINT identity_document_type_translations_locale_code_fkey FOREIGN KEY (locale_code) REFERENCES global.locales(code) ON DELETE RESTRICT;


--
-- Name: location_translations location_translations_locale_fk; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.location_translations
    ADD CONSTRAINT location_translations_locale_fk FOREIGN KEY (language_code) REFERENCES global.locales(code) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: location_translations location_translations_location_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.location_translations
    ADD CONSTRAINT location_translations_location_id_fkey FOREIGN KEY (location_id) REFERENCES global.locations(id) ON DELETE RESTRICT;


--
-- Name: locations locations_country_code_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.locations
    ADD CONSTRAINT locations_country_code_fkey FOREIGN KEY (country_code) REFERENCES global.countries(code) ON DELETE RESTRICT;


--
-- Name: locations locations_parent_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.locations
    ADD CONSTRAINT locations_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES global.locations(id) ON DELETE RESTRICT;


--
-- Name: menu_template_functions menu_template_functions_system_function_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.menu_template_functions
    ADD CONSTRAINT menu_template_functions_system_function_id_fkey FOREIGN KEY (system_function_id) REFERENCES global.system_functions(id) ON DELETE RESTRICT;


--
-- Name: menu_template_functions menu_template_functions_template_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.menu_template_functions
    ADD CONSTRAINT menu_template_functions_template_id_fkey FOREIGN KEY (template_id) REFERENCES global.menu_templates(id) ON DELETE RESTRICT;


--
-- Name: role_type_translations role_type_translations_locale_code_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.role_type_translations
    ADD CONSTRAINT role_type_translations_locale_code_fkey FOREIGN KEY (locale_code) REFERENCES global.locales(code) ON DELETE RESTRICT;


--
-- Name: role_type_translations role_type_translations_role_type_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.role_type_translations
    ADD CONSTRAINT role_type_translations_role_type_id_fkey FOREIGN KEY (role_type_id) REFERENCES global.role_types(id) ON DELETE RESTRICT;


--
-- Name: tenant_relationship_invites tenant_relationship_invites_canceled_by_tenant_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationship_invites
    ADD CONSTRAINT tenant_relationship_invites_canceled_by_tenant_id_fkey FOREIGN KEY (canceled_by_tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: tenant_relationship_invites tenant_relationship_invites_relationship_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationship_invites
    ADD CONSTRAINT tenant_relationship_invites_relationship_id_fkey FOREIGN KEY (relationship_id) REFERENCES global.tenant_relationships(id) ON DELETE RESTRICT;


--
-- Name: tenant_relationships tenant_relationships_tenant_id_holding_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationships
    ADD CONSTRAINT tenant_relationships_tenant_id_holding_fkey FOREIGN KEY (tenant_id_holding) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: tenant_relationships tenant_relationships_tenant_id_subsidiary_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_relationships
    ADD CONSTRAINT tenant_relationships_tenant_id_subsidiary_fkey FOREIGN KEY (tenant_id_subsidiary) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: tenant_transfers tenant_transfers_canceled_by_tenant_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_transfers
    ADD CONSTRAINT tenant_transfers_canceled_by_tenant_id_fkey FOREIGN KEY (canceled_by_tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: tenant_transfers tenant_transfers_relationship_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_transfers
    ADD CONSTRAINT tenant_transfers_relationship_id_fkey FOREIGN KEY (relationship_id) REFERENCES global.tenant_relationships(id) ON DELETE RESTRICT;


--
-- Name: tenant_transfers tenant_transfers_requested_by_tenant_id_fkey; Type: FK CONSTRAINT; Schema: global; Owner: -
--

ALTER TABLE ONLY global.tenant_transfers
    ADD CONSTRAINT tenant_transfers_requested_by_tenant_id_fkey FOREIGN KEY (requested_by_tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: address_validities address_validities_address_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.address_validities
    ADD CONSTRAINT address_validities_address_fk FOREIGN KEY (address_id, tenant_id, party_id) REFERENCES tenants.addresses(id, tenant_id, party_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: address_validities address_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.address_validities
    ADD CONSTRAINT address_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: addresses addresses_city_location_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.addresses
    ADD CONSTRAINT addresses_city_location_id_fkey FOREIGN KEY (city_location_id) REFERENCES global.locations(id) ON DELETE RESTRICT;


--
-- Name: addresses addresses_parties_addresses; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.addresses
    ADD CONSTRAINT addresses_parties_addresses FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: addresses addresses_subscriptions_addresses; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.addresses
    ADD CONSTRAINT addresses_subscriptions_addresses FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: attached_documents attached_documents_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents
    ADD CONSTRAINT attached_documents_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: corporate_groups corporate_groups_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.corporate_groups
    ADD CONSTRAINT corporate_groups_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_address_validities department_address_validities_address_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_address_validities
    ADD CONSTRAINT department_address_validities_address_fk FOREIGN KEY (department_address_id, tenant_id, department_id) REFERENCES tenants.department_addresses(id, tenant_id, department_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_address_validities department_address_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_address_validities
    ADD CONSTRAINT department_address_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_addresses department_addresses_city_location_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_addresses
    ADD CONSTRAINT department_addresses_city_location_id_fkey FOREIGN KEY (city_location_id) REFERENCES global.locations(id) ON DELETE RESTRICT;


--
-- Name: department_addresses department_addresses_department_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_addresses
    ADD CONSTRAINT department_addresses_department_fk FOREIGN KEY (department_id, tenant_id) REFERENCES tenants.departments(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_addresses department_addresses_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_addresses
    ADD CONSTRAINT department_addresses_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_contact_mechanism_validities department_contact_mechanism_validities_contact_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanism_validities
    ADD CONSTRAINT department_contact_mechanism_validities_contact_fk FOREIGN KEY (department_contact_mechanism_id, tenant_id, department_id) REFERENCES tenants.department_contact_mechanisms(id, tenant_id, department_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_contact_mechanism_validities department_contact_mechanism_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanism_validities
    ADD CONSTRAINT department_contact_mechanism_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_contact_mechanisms department_contact_mechanisms_contact_mechanism_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanisms
    ADD CONSTRAINT department_contact_mechanisms_contact_mechanism_id_fkey FOREIGN KEY (contact_mechanism_id) REFERENCES global.contact_mechanisms(id) ON DELETE RESTRICT;


--
-- Name: department_contact_mechanisms department_contact_mechanisms_department_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanisms
    ADD CONSTRAINT department_contact_mechanisms_department_fk FOREIGN KEY (department_id, tenant_id) REFERENCES tenants.departments(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_contact_mechanisms department_contact_mechanisms_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_contact_mechanisms
    ADD CONSTRAINT department_contact_mechanisms_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_group_assignments department_group_assignments_department_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_group_assignments
    ADD CONSTRAINT department_group_assignments_department_fk FOREIGN KEY (department_id, tenant_id) REFERENCES tenants.departments(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_group_assignments department_group_assignments_group_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_group_assignments
    ADD CONSTRAINT department_group_assignments_group_fk FOREIGN KEY (corporate_group_id, tenant_id) REFERENCES tenants.corporate_groups(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_group_assignments department_group_assignments_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_group_assignments
    ADD CONSTRAINT department_group_assignments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_menu_option_validities department_menu_option_validities_parent_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_option_validities
    ADD CONSTRAINT department_menu_option_validities_parent_fk FOREIGN KEY (department_menu_option_id, tenant_id, department_id, menu_option_id) REFERENCES tenants.department_menu_options(id, tenant_id, department_id, menu_option_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_menu_option_validities department_menu_option_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_option_validities
    ADD CONSTRAINT department_menu_option_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_menu_options department_menu_options_department_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_department_fk FOREIGN KEY (department_id, tenant_id) REFERENCES tenants.departments(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_menu_options department_menu_options_option_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_option_fk FOREIGN KEY (menu_option_id, tenant_id) REFERENCES tenants.menu_options(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_menu_options department_menu_options_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_menu_options
    ADD CONSTRAINT department_menu_options_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: department_translations department_translations_department_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_translations
    ADD CONSTRAINT department_translations_department_fk FOREIGN KEY (department_id, tenant_id) REFERENCES tenants.departments(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: department_translations department_translations_locale_code_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_translations
    ADD CONSTRAINT department_translations_locale_code_fkey FOREIGN KEY (locale_code) REFERENCES global.locales(code) ON DELETE RESTRICT;


--
-- Name: department_translations department_translations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.department_translations
    ADD CONSTRAINT department_translations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: departments departments_departments_children; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.departments
    ADD CONSTRAINT departments_departments_children FOREIGN KEY (parent_department_id) REFERENCES tenants.departments(id) ON DELETE RESTRICT;


--
-- Name: departments departments_employees_managed_departments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.departments
    ADD CONSTRAINT departments_employees_managed_departments FOREIGN KEY (manager_party_id) REFERENCES tenants.employees(party_id) ON DELETE RESTRICT;


--
-- Name: departments departments_subscriptions_departments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.departments
    ADD CONSTRAINT departments_subscriptions_departments FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: employee_assignments employee_assignments_departments_assignments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_departments_assignments FOREIGN KEY (department_id) REFERENCES tenants.departments(id) ON DELETE RESTRICT;


--
-- Name: employee_assignments employee_assignments_employees_assignments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_employees_assignments FOREIGN KEY (employee_party_id) REFERENCES tenants.employees(party_id) ON DELETE RESTRICT;


--
-- Name: employee_assignments employee_assignments_jobs_assignments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_jobs_assignments FOREIGN KEY (job_id) REFERENCES tenants.jobs(id) ON DELETE RESTRICT;


--
-- Name: employee_assignments employee_assignments_subscriptions_employee_assignments; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_assignments
    ADD CONSTRAINT employee_assignments_subscriptions_employee_assignments FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_employee_party_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_menu_option_overrides
    ADD CONSTRAINT employee_menu_option_overrides_employee_party_id_fkey FOREIGN KEY (employee_party_id) REFERENCES tenants.employees(party_id) ON DELETE RESTRICT;


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_system_function_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_menu_option_overrides
    ADD CONSTRAINT employee_menu_option_overrides_system_function_id_fkey FOREIGN KEY (system_function_id) REFERENCES global.system_functions(id) ON DELETE RESTRICT;


--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employee_menu_option_overrides
    ADD CONSTRAINT employee_menu_option_overrides_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: employees employees_departments_employees; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_departments_employees FOREIGN KEY (department_id) REFERENCES tenants.departments(id) ON DELETE RESTRICT;


--
-- Name: employees employees_employees_direct_reports; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_employees_direct_reports FOREIGN KEY (manager_party_id) REFERENCES tenants.employees(party_id) ON DELETE RESTRICT;


--
-- Name: employees employees_jobs_employees; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_jobs_employees FOREIGN KEY (job_id) REFERENCES tenants.jobs(id) ON DELETE RESTRICT;


--
-- Name: employees employees_subscriptions_employees; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.employees
    ADD CONSTRAINT employees_subscriptions_employees FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: job_families job_families_subscriptions_job_families; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.job_families
    ADD CONSTRAINT job_families_subscriptions_job_families FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: jobs jobs_job_families_jobs; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.jobs
    ADD CONSTRAINT jobs_job_families_jobs FOREIGN KEY (job_family_id) REFERENCES tenants.job_families(id) ON DELETE RESTRICT;


--
-- Name: jobs jobs_subscriptions_jobs; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.jobs
    ADD CONSTRAINT jobs_subscriptions_jobs FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: menu_categories menu_categories_parent_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_categories
    ADD CONSTRAINT menu_categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES tenants.menu_categories(id) ON DELETE RESTRICT;


--
-- Name: menu_categories menu_categories_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_categories
    ADD CONSTRAINT menu_categories_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: menu_option_categories menu_option_categories_category_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_categories
    ADD CONSTRAINT menu_option_categories_category_fk FOREIGN KEY (menu_category_id, tenant_id) REFERENCES tenants.menu_categories(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: menu_option_categories menu_option_categories_option_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_categories
    ADD CONSTRAINT menu_option_categories_option_fk FOREIGN KEY (menu_option_id, tenant_id) REFERENCES tenants.menu_options(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: menu_option_categories menu_option_categories_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_categories
    ADD CONSTRAINT menu_option_categories_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: menu_option_functions menu_option_functions_option_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_functions
    ADD CONSTRAINT menu_option_functions_option_fk FOREIGN KEY (menu_option_id, tenant_id) REFERENCES tenants.menu_options(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: menu_option_functions menu_option_functions_system_function_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_functions
    ADD CONSTRAINT menu_option_functions_system_function_id_fkey FOREIGN KEY (system_function_id) REFERENCES global.system_functions(id) ON DELETE RESTRICT;


--
-- Name: menu_option_functions menu_option_functions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_option_functions
    ADD CONSTRAINT menu_option_functions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: menu_options menu_options_template_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_options
    ADD CONSTRAINT menu_options_template_id_fkey FOREIGN KEY (template_id) REFERENCES global.menu_templates(id) ON DELETE RESTRICT;


--
-- Name: menu_options menu_options_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.menu_options
    ADD CONSTRAINT menu_options_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: organization_profiles organization_profiles_parties_organization_profile; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.organization_profiles
    ADD CONSTRAINT organization_profiles_parties_organization_profile FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: organization_profiles organization_profiles_subscriptions_organization_profiles; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.organization_profiles
    ADD CONSTRAINT organization_profiles_subscriptions_organization_profiles FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: parties parties_parties_merged_parties; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.parties
    ADD CONSTRAINT parties_parties_merged_parties FOREIGN KEY (canonical_party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: parties parties_subscriptions_parties; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.parties
    ADD CONSTRAINT parties_subscriptions_parties FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_contact_mechanism_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanism_validities
    ADD CONSTRAINT party_contact_mechanism_validities_contact_mechanism_id_fkey FOREIGN KEY (contact_mechanism_id) REFERENCES global.contact_mechanisms(id) ON DELETE RESTRICT;


--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_relation_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanism_validities
    ADD CONSTRAINT party_contact_mechanism_validities_relation_fk FOREIGN KEY (party_contact_mechanism_id, tenant_id, party_id) REFERENCES tenants.party_contact_mechanisms(id, tenant_id, party_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanism_validities
    ADD CONSTRAINT party_contact_mechanism_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_contact_mechanisms party_contact_mechanisms_contact_mechanisms_party_contact_mecha; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanisms
    ADD CONSTRAINT party_contact_mechanisms_contact_mechanisms_party_contact_mecha FOREIGN KEY (contact_mechanism_id) REFERENCES global.contact_mechanisms(id) ON DELETE RESTRICT;


--
-- Name: party_contact_mechanisms party_contact_mechanisms_parties_party_contact_mechanisms; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanisms
    ADD CONSTRAINT party_contact_mechanisms_parties_party_contact_mechanisms FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: party_contact_mechanisms party_contact_mechanisms_subscriptions_party_contact_mechanisms; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_contact_mechanisms
    ADD CONSTRAINT party_contact_mechanisms_subscriptions_party_contact_mechanisms FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_identity_document_validities party_identity_document_validities_document_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_document_validities
    ADD CONSTRAINT party_identity_document_validities_document_fk FOREIGN KEY (party_identity_document_id, tenant_id, party_id) REFERENCES tenants.party_identity_documents(id, tenant_id, party_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_identity_document_validities party_identity_document_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_document_validities
    ADD CONSTRAINT party_identity_document_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_identity_documents party_identity_documents_document_type_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_document_type_id_fkey FOREIGN KEY (document_type_id) REFERENCES global.identity_document_types(id) ON DELETE RESTRICT;


--
-- Name: party_identity_documents party_identity_documents_issuing_country_code_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_issuing_country_code_fkey FOREIGN KEY (issuing_country_code) REFERENCES global.countries(code) ON DELETE RESTRICT;


--
-- Name: party_identity_documents party_identity_documents_jurisdiction_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_jurisdiction_fk FOREIGN KEY (issuing_jurisdiction_id, issuing_country_code) REFERENCES global.locations(id, country_code) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_identity_documents party_identity_documents_party_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_party_fk FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_identity_documents party_identity_documents_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_identity_documents
    ADD CONSTRAINT party_identity_documents_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_merge_log party_merge_log_parties_merge_sources; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_merge_log
    ADD CONSTRAINT party_merge_log_parties_merge_sources FOREIGN KEY (source_party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: party_merge_log party_merge_log_parties_merge_targets; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_merge_log
    ADD CONSTRAINT party_merge_log_parties_merge_targets FOREIGN KEY (target_party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: party_merge_log party_merge_log_subscriptions_party_merge_logs; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_merge_log
    ADD CONSTRAINT party_merge_log_subscriptions_party_merge_logs FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_name_validities party_name_validities_name_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_name_validities
    ADD CONSTRAINT party_name_validities_name_fk FOREIGN KEY (party_name_id, tenant_id, party_id, name_type_id) REFERENCES tenants.party_names(id, tenant_id, party_id, name_type_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_name_validities party_name_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_name_validities
    ADD CONSTRAINT party_name_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_names party_names_name_type_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_names
    ADD CONSTRAINT party_names_name_type_id_fkey FOREIGN KEY (name_type_id) REFERENCES global.name_types(id) ON DELETE RESTRICT;


--
-- Name: party_names party_names_party_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_names
    ADD CONSTRAINT party_names_party_fk FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_names party_names_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_names
    ADD CONSTRAINT party_names_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_nationalities party_nationalities_country_code_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_nationalities
    ADD CONSTRAINT party_nationalities_country_code_fkey FOREIGN KEY (country_code) REFERENCES global.countries(code) ON DELETE RESTRICT;


--
-- Name: party_nationalities party_nationalities_party_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_nationalities
    ADD CONSTRAINT party_nationalities_party_fk FOREIGN KEY (party_id, tenant_id) REFERENCES tenants.parties(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: party_nationalities party_nationalities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_nationalities
    ADD CONSTRAINT party_nationalities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: party_roles party_roles_parties_party_roles; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_roles
    ADD CONSTRAINT party_roles_parties_party_roles FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: party_roles party_roles_role_types_party_roles; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_roles
    ADD CONSTRAINT party_roles_role_types_party_roles FOREIGN KEY (role_type_id) REFERENCES global.role_types(id) ON DELETE RESTRICT;


--
-- Name: party_roles party_roles_subscriptions_party_roles; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.party_roles
    ADD CONSTRAINT party_roles_subscriptions_party_roles FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: password_reset_tokens password_reset_tokens_parties_password_reset_tokens; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_parties_password_reset_tokens FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: password_reset_tokens password_reset_tokens_subscriptions_password_reset_tokens; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_subscriptions_password_reset_tokens FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: person_profiles person_profiles_parties_person_profile; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.person_profiles
    ADD CONSTRAINT person_profiles_parties_person_profile FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: person_profiles person_profiles_subscriptions_person_profiles; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.person_profiles
    ADD CONSTRAINT person_profiles_subscriptions_person_profiles FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: position_menu_option_validities position_menu_option_validities_parent_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_option_validities
    ADD CONSTRAINT position_menu_option_validities_parent_fk FOREIGN KEY (position_menu_option_id, tenant_id, job_id, menu_option_id) REFERENCES tenants.position_menu_options(id, tenant_id, job_id, menu_option_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: position_menu_option_validities position_menu_option_validities_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_option_validities
    ADD CONSTRAINT position_menu_option_validities_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: position_menu_options position_menu_options_job_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_job_fk FOREIGN KEY (job_id, tenant_id) REFERENCES tenants.jobs(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: position_menu_options position_menu_options_option_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_option_fk FOREIGN KEY (menu_option_id, tenant_id) REFERENCES tenants.menu_options(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: position_menu_options position_menu_options_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.position_menu_options
    ADD CONSTRAINT position_menu_options_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: role_menu_options role_menu_options_option_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.role_menu_options
    ADD CONSTRAINT role_menu_options_option_fk FOREIGN KEY (menu_option_id, tenant_id) REFERENCES tenants.menu_options(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: role_menu_options role_menu_options_role_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.role_menu_options
    ADD CONSTRAINT role_menu_options_role_fk FOREIGN KEY (tenant_role_id, tenant_id) REFERENCES tenants.tenant_roles(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: role_menu_options role_menu_options_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.role_menu_options
    ADD CONSTRAINT role_menu_options_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: sessions sessions_parties_sessions; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.sessions
    ADD CONSTRAINT sessions_parties_sessions FOREIGN KEY (user_id) REFERENCES tenants.parties(id) ON DELETE CASCADE;


--
-- Name: sessions sessions_subscriptions_sessions; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.sessions
    ADD CONSTRAINT sessions_subscriptions_sessions FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: tenant_roles tenant_roles_client_application_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.tenant_roles
    ADD CONSTRAINT tenant_roles_client_application_id_fkey FOREIGN KEY (client_application_id) REFERENCES global.client_applications(id) ON DELETE RESTRICT;


--
-- Name: tenant_roles tenant_roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.tenant_roles
    ADD CONSTRAINT tenant_roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: user_credentials user_credentials_parties_credentials; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_credentials
    ADD CONSTRAINT user_credentials_parties_credentials FOREIGN KEY (party_id) REFERENCES tenants.parties(id) ON DELETE RESTRICT;


--
-- Name: user_credentials user_credentials_party_contact_mechanisms_credentials; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_credentials
    ADD CONSTRAINT user_credentials_party_contact_mechanisms_credentials FOREIGN KEY (party_contact_mechanism_id) REFERENCES tenants.party_contact_mechanisms(id) ON DELETE RESTRICT;


--
-- Name: user_credentials user_credentials_subscriptions_user_credentials; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_credentials
    ADD CONSTRAINT user_credentials_subscriptions_user_credentials FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: user_role_assignments user_role_assignments_role_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_role_assignments
    ADD CONSTRAINT user_role_assignments_role_fk FOREIGN KEY (tenant_role_id, tenant_id) REFERENCES tenants.tenant_roles(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: user_role_assignments user_role_assignments_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_role_assignments
    ADD CONSTRAINT user_role_assignments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES global.subscriptions(id) ON DELETE RESTRICT;


--
-- Name: user_role_assignments user_role_assignments_user_fk; Type: FK CONSTRAINT; Schema: tenants; Owner: -
--

ALTER TABLE ONLY tenants.user_role_assignments
    ADD CONSTRAINT user_role_assignments_user_fk FOREIGN KEY (user_credentials_id, tenant_id) REFERENCES tenants.user_credentials(id, tenant_id) ON UPDATE RESTRICT ON DELETE RESTRICT;


--
-- Name: tenant_relationship_invites; Type: ROW SECURITY; Schema: global; Owner: -
--

ALTER TABLE global.tenant_relationship_invites ENABLE ROW LEVEL SECURITY;

--
-- Name: tenant_relationship_invites tenant_relationship_invites_insert; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationship_invites_insert ON global.tenant_relationship_invites FOR INSERT WITH CHECK ((EXISTS ( SELECT 1
   FROM global.tenant_relationships r
  WHERE ((r.id = tenant_relationship_invites.relationship_id) AND (r.tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)))));


--
-- Name: tenant_relationship_invites tenant_relationship_invites_select; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationship_invites_select ON global.tenant_relationship_invites FOR SELECT USING ((EXISTS ( SELECT 1
   FROM global.tenant_relationships r
  WHERE ((r.id = tenant_relationship_invites.relationship_id) AND ((r.tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (r.tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid))))));


--
-- Name: tenant_relationship_invites tenant_relationship_invites_update; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationship_invites_update ON global.tenant_relationship_invites FOR UPDATE USING ((EXISTS ( SELECT 1
   FROM global.tenant_relationships r
  WHERE ((r.id = tenant_relationship_invites.relationship_id) AND ((r.tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (r.tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)))))) WITH CHECK ((EXISTS ( SELECT 1
   FROM global.tenant_relationships r
  WHERE ((r.id = tenant_relationship_invites.relationship_id) AND ((r.tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (r.tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid))))));


--
-- Name: tenant_relationships; Type: ROW SECURITY; Schema: global; Owner: -
--

ALTER TABLE global.tenant_relationships ENABLE ROW LEVEL SECURITY;

--
-- Name: tenant_relationships tenant_relationships_insert; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationships_insert ON global.tenant_relationships FOR INSERT WITH CHECK ((tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: tenant_relationships tenant_relationships_select; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationships_select ON global.tenant_relationships FOR SELECT USING (((tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)));


--
-- Name: tenant_relationships tenant_relationships_update; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_relationships_update ON global.tenant_relationships FOR UPDATE USING (((tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid))) WITH CHECK (((tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)));


--
-- Name: tenant_transfers; Type: ROW SECURITY; Schema: global; Owner: -
--

ALTER TABLE global.tenant_transfers ENABLE ROW LEVEL SECURITY;

--
-- Name: tenant_transfers tenant_transfers_insert; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_transfers_insert ON global.tenant_transfers FOR INSERT WITH CHECK ((requested_by_tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: tenant_transfers tenant_transfers_select; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_transfers_select ON global.tenant_transfers FOR SELECT USING (((requested_by_tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (EXISTS ( SELECT 1
   FROM global.tenant_relationships r
  WHERE ((r.id = tenant_transfers.relationship_id) AND ((r.tenant_id_holding = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid) OR (r.tenant_id_subsidiary = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)))))));


--
-- Name: tenant_transfers tenant_transfers_update; Type: POLICY; Schema: global; Owner: -
--

CREATE POLICY tenant_transfers_update ON global.tenant_transfers FOR UPDATE USING ((requested_by_tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((requested_by_tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: address_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.address_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: address_validities address_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY address_validities_tenant_isolation ON tenants.address_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: addresses; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.addresses ENABLE ROW LEVEL SECURITY;

--
-- Name: addresses addresses_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY addresses_tenant_isolation ON tenants.addresses USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p00; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p00 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p00 attached_documents_p00_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p00_tenant_isolation ON tenants.attached_documents_p00 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p01; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p01 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p01 attached_documents_p01_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p01_tenant_isolation ON tenants.attached_documents_p01 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p02; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p02 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p02 attached_documents_p02_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p02_tenant_isolation ON tenants.attached_documents_p02 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p03; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p03 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p03 attached_documents_p03_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p03_tenant_isolation ON tenants.attached_documents_p03 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p04; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p04 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p04 attached_documents_p04_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p04_tenant_isolation ON tenants.attached_documents_p04 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p05; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p05 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p05 attached_documents_p05_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p05_tenant_isolation ON tenants.attached_documents_p05 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p06; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p06 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p06 attached_documents_p06_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p06_tenant_isolation ON tenants.attached_documents_p06 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p07; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p07 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p07 attached_documents_p07_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p07_tenant_isolation ON tenants.attached_documents_p07 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p08; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p08 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p08 attached_documents_p08_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p08_tenant_isolation ON tenants.attached_documents_p08 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p09; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p09 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p09 attached_documents_p09_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p09_tenant_isolation ON tenants.attached_documents_p09 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p10; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p10 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p10 attached_documents_p10_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p10_tenant_isolation ON tenants.attached_documents_p10 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p11; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p11 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p11 attached_documents_p11_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p11_tenant_isolation ON tenants.attached_documents_p11 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p12; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p12 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p12 attached_documents_p12_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p12_tenant_isolation ON tenants.attached_documents_p12 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p13; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p13 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p13 attached_documents_p13_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p13_tenant_isolation ON tenants.attached_documents_p13 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p14; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p14 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p14 attached_documents_p14_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p14_tenant_isolation ON tenants.attached_documents_p14 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p15; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p15 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p15 attached_documents_p15_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p15_tenant_isolation ON tenants.attached_documents_p15 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p16; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p16 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p16 attached_documents_p16_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p16_tenant_isolation ON tenants.attached_documents_p16 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p17; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p17 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p17 attached_documents_p17_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p17_tenant_isolation ON tenants.attached_documents_p17 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p18; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p18 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p18 attached_documents_p18_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p18_tenant_isolation ON tenants.attached_documents_p18 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p19; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p19 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p19 attached_documents_p19_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p19_tenant_isolation ON tenants.attached_documents_p19 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p20; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p20 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p20 attached_documents_p20_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p20_tenant_isolation ON tenants.attached_documents_p20 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p21; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p21 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p21 attached_documents_p21_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p21_tenant_isolation ON tenants.attached_documents_p21 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p22; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p22 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p22 attached_documents_p22_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p22_tenant_isolation ON tenants.attached_documents_p22 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p23; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p23 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p23 attached_documents_p23_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p23_tenant_isolation ON tenants.attached_documents_p23 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p24; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p24 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p24 attached_documents_p24_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p24_tenant_isolation ON tenants.attached_documents_p24 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p25; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p25 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p25 attached_documents_p25_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p25_tenant_isolation ON tenants.attached_documents_p25 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p26; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p26 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p26 attached_documents_p26_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p26_tenant_isolation ON tenants.attached_documents_p26 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p27; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p27 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p27 attached_documents_p27_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p27_tenant_isolation ON tenants.attached_documents_p27 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p28; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p28 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p28 attached_documents_p28_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p28_tenant_isolation ON tenants.attached_documents_p28 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p29; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p29 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p29 attached_documents_p29_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p29_tenant_isolation ON tenants.attached_documents_p29 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p30; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p30 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p30 attached_documents_p30_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p30_tenant_isolation ON tenants.attached_documents_p30 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p31; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p31 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p31 attached_documents_p31_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p31_tenant_isolation ON tenants.attached_documents_p31 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p32; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p32 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p32 attached_documents_p32_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p32_tenant_isolation ON tenants.attached_documents_p32 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p33; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p33 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p33 attached_documents_p33_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p33_tenant_isolation ON tenants.attached_documents_p33 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p34; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p34 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p34 attached_documents_p34_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p34_tenant_isolation ON tenants.attached_documents_p34 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p35; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p35 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p35 attached_documents_p35_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p35_tenant_isolation ON tenants.attached_documents_p35 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p36; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p36 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p36 attached_documents_p36_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p36_tenant_isolation ON tenants.attached_documents_p36 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p37; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p37 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p37 attached_documents_p37_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p37_tenant_isolation ON tenants.attached_documents_p37 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p38; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p38 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p38 attached_documents_p38_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p38_tenant_isolation ON tenants.attached_documents_p38 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p39; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p39 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p39 attached_documents_p39_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p39_tenant_isolation ON tenants.attached_documents_p39 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p40; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p40 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p40 attached_documents_p40_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p40_tenant_isolation ON tenants.attached_documents_p40 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p41; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p41 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p41 attached_documents_p41_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p41_tenant_isolation ON tenants.attached_documents_p41 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p42; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p42 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p42 attached_documents_p42_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p42_tenant_isolation ON tenants.attached_documents_p42 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p43; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p43 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p43 attached_documents_p43_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p43_tenant_isolation ON tenants.attached_documents_p43 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p44; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p44 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p44 attached_documents_p44_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p44_tenant_isolation ON tenants.attached_documents_p44 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p45; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p45 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p45 attached_documents_p45_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p45_tenant_isolation ON tenants.attached_documents_p45 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p46; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p46 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p46 attached_documents_p46_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p46_tenant_isolation ON tenants.attached_documents_p46 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p47; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p47 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p47 attached_documents_p47_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p47_tenant_isolation ON tenants.attached_documents_p47 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p48; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p48 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p48 attached_documents_p48_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p48_tenant_isolation ON tenants.attached_documents_p48 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p49; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p49 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p49 attached_documents_p49_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p49_tenant_isolation ON tenants.attached_documents_p49 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p50; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p50 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p50 attached_documents_p50_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p50_tenant_isolation ON tenants.attached_documents_p50 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p51; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p51 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p51 attached_documents_p51_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p51_tenant_isolation ON tenants.attached_documents_p51 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p52; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p52 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p52 attached_documents_p52_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p52_tenant_isolation ON tenants.attached_documents_p52 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p53; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p53 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p53 attached_documents_p53_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p53_tenant_isolation ON tenants.attached_documents_p53 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p54; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p54 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p54 attached_documents_p54_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p54_tenant_isolation ON tenants.attached_documents_p54 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p55; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p55 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p55 attached_documents_p55_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p55_tenant_isolation ON tenants.attached_documents_p55 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p56; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p56 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p56 attached_documents_p56_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p56_tenant_isolation ON tenants.attached_documents_p56 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p57; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p57 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p57 attached_documents_p57_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p57_tenant_isolation ON tenants.attached_documents_p57 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p58; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p58 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p58 attached_documents_p58_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p58_tenant_isolation ON tenants.attached_documents_p58 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p59; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p59 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p59 attached_documents_p59_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p59_tenant_isolation ON tenants.attached_documents_p59 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p60; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p60 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p60 attached_documents_p60_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p60_tenant_isolation ON tenants.attached_documents_p60 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p61; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p61 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p61 attached_documents_p61_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p61_tenant_isolation ON tenants.attached_documents_p61 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p62; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p62 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p62 attached_documents_p62_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p62_tenant_isolation ON tenants.attached_documents_p62 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents_p63; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.attached_documents_p63 ENABLE ROW LEVEL SECURITY;

--
-- Name: attached_documents_p63 attached_documents_p63_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_p63_tenant_isolation ON tenants.attached_documents_p63 USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: attached_documents attached_documents_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY attached_documents_tenant_isolation ON tenants.attached_documents USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: corporate_groups; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.corporate_groups ENABLE ROW LEVEL SECURITY;

--
-- Name: corporate_groups corporate_groups_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY corporate_groups_tenant_isolation ON tenants.corporate_groups USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_address_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_address_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: department_address_validities department_address_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_address_validities_tenant_isolation ON tenants.department_address_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_addresses; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_addresses ENABLE ROW LEVEL SECURITY;

--
-- Name: department_addresses department_addresses_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_addresses_tenant_isolation ON tenants.department_addresses USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_contact_mechanism_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_contact_mechanism_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: department_contact_mechanism_validities department_contact_mechanism_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_contact_mechanism_validities_tenant_isolation ON tenants.department_contact_mechanism_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_contact_mechanisms; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_contact_mechanisms ENABLE ROW LEVEL SECURITY;

--
-- Name: department_contact_mechanisms department_contact_mechanisms_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_contact_mechanisms_tenant_isolation ON tenants.department_contact_mechanisms USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_group_assignments; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_group_assignments ENABLE ROW LEVEL SECURITY;

--
-- Name: department_group_assignments department_group_assignments_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_group_assignments_tenant_isolation ON tenants.department_group_assignments USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_menu_option_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_menu_option_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: department_menu_option_validities department_menu_option_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_menu_option_validities_tenant_isolation ON tenants.department_menu_option_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_menu_options; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_menu_options ENABLE ROW LEVEL SECURITY;

--
-- Name: department_menu_options department_menu_options_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_menu_options_tenant_isolation ON tenants.department_menu_options USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: department_translations; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.department_translations ENABLE ROW LEVEL SECURITY;

--
-- Name: department_translations department_translations_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY department_translations_tenant_isolation ON tenants.department_translations USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: departments; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.departments ENABLE ROW LEVEL SECURITY;

--
-- Name: departments departments_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY departments_tenant_isolation ON tenants.departments USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: employee_assignments; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.employee_assignments ENABLE ROW LEVEL SECURITY;

--
-- Name: employee_assignments employee_assignments_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY employee_assignments_tenant_isolation ON tenants.employee_assignments USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: employee_menu_option_overrides; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.employee_menu_option_overrides ENABLE ROW LEVEL SECURITY;

--
-- Name: employee_menu_option_overrides employee_menu_option_overrides_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY employee_menu_option_overrides_tenant_isolation ON tenants.employee_menu_option_overrides USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: employees; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.employees ENABLE ROW LEVEL SECURITY;

--
-- Name: employees employees_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY employees_tenant_isolation ON tenants.employees USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: job_families; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.job_families ENABLE ROW LEVEL SECURITY;

--
-- Name: job_families job_families_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY job_families_tenant_isolation ON tenants.job_families USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: jobs; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.jobs ENABLE ROW LEVEL SECURITY;

--
-- Name: jobs jobs_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY jobs_tenant_isolation ON tenants.jobs USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: menu_categories; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.menu_categories ENABLE ROW LEVEL SECURITY;

--
-- Name: menu_categories menu_categories_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY menu_categories_tenant_isolation ON tenants.menu_categories USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: menu_option_categories; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.menu_option_categories ENABLE ROW LEVEL SECURITY;

--
-- Name: menu_option_categories menu_option_categories_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY menu_option_categories_tenant_isolation ON tenants.menu_option_categories USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: menu_option_functions; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.menu_option_functions ENABLE ROW LEVEL SECURITY;

--
-- Name: menu_option_functions menu_option_functions_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY menu_option_functions_tenant_isolation ON tenants.menu_option_functions USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: menu_options; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.menu_options ENABLE ROW LEVEL SECURITY;

--
-- Name: menu_options menu_options_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY menu_options_tenant_isolation ON tenants.menu_options USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: organization_profiles; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.organization_profiles ENABLE ROW LEVEL SECURITY;

--
-- Name: organization_profiles organization_profiles_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY organization_profiles_tenant_isolation ON tenants.organization_profiles USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: parties; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.parties ENABLE ROW LEVEL SECURITY;

--
-- Name: parties parties_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY parties_tenant_isolation ON tenants.parties USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_contact_mechanism_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_contact_mechanism_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: party_contact_mechanism_validities party_contact_mechanism_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_contact_mechanism_validities_tenant_isolation ON tenants.party_contact_mechanism_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_contact_mechanisms; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_contact_mechanisms ENABLE ROW LEVEL SECURITY;

--
-- Name: party_contact_mechanisms party_contact_mechanisms_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_contact_mechanisms_tenant_isolation ON tenants.party_contact_mechanisms USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_identity_document_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_identity_document_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: party_identity_document_validities party_identity_document_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_identity_document_validities_tenant_isolation ON tenants.party_identity_document_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_identity_documents; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_identity_documents ENABLE ROW LEVEL SECURITY;

--
-- Name: party_identity_documents party_identity_documents_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_identity_documents_tenant_isolation ON tenants.party_identity_documents USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_merge_log; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_merge_log ENABLE ROW LEVEL SECURITY;

--
-- Name: party_merge_log party_merge_log_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_merge_log_tenant_isolation ON tenants.party_merge_log USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_name_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_name_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: party_name_validities party_name_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_name_validities_tenant_isolation ON tenants.party_name_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_names; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_names ENABLE ROW LEVEL SECURITY;

--
-- Name: party_names party_names_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_names_tenant_isolation ON tenants.party_names USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_nationalities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_nationalities ENABLE ROW LEVEL SECURITY;

--
-- Name: party_nationalities party_nationalities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_nationalities_tenant_isolation ON tenants.party_nationalities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: party_roles; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.party_roles ENABLE ROW LEVEL SECURITY;

--
-- Name: party_roles party_roles_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY party_roles_tenant_isolation ON tenants.party_roles USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: password_reset_tokens; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.password_reset_tokens ENABLE ROW LEVEL SECURITY;

--
-- Name: password_reset_tokens password_reset_tokens_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY password_reset_tokens_tenant_isolation ON tenants.password_reset_tokens USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: person_profiles; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.person_profiles ENABLE ROW LEVEL SECURITY;

--
-- Name: person_profiles person_profiles_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY person_profiles_tenant_isolation ON tenants.person_profiles USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: position_menu_option_validities; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.position_menu_option_validities ENABLE ROW LEVEL SECURITY;

--
-- Name: position_menu_option_validities position_menu_option_validities_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY position_menu_option_validities_tenant_isolation ON tenants.position_menu_option_validities USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: position_menu_options; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.position_menu_options ENABLE ROW LEVEL SECURITY;

--
-- Name: position_menu_options position_menu_options_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY position_menu_options_tenant_isolation ON tenants.position_menu_options USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: role_menu_options; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.role_menu_options ENABLE ROW LEVEL SECURITY;

--
-- Name: role_menu_options role_menu_options_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY role_menu_options_tenant_isolation ON tenants.role_menu_options USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: sessions; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.sessions ENABLE ROW LEVEL SECURITY;

--
-- Name: sessions sessions_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY sessions_tenant_isolation ON tenants.sessions USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: tenant_roles; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.tenant_roles ENABLE ROW LEVEL SECURITY;

--
-- Name: tenant_roles tenant_roles_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY tenant_roles_tenant_isolation ON tenants.tenant_roles USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: user_credentials; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.user_credentials ENABLE ROW LEVEL SECURITY;

--
-- Name: user_credentials user_credentials_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY user_credentials_tenant_isolation ON tenants.user_credentials USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- Name: user_role_assignments; Type: ROW SECURITY; Schema: tenants; Owner: -
--

ALTER TABLE tenants.user_role_assignments ENABLE ROW LEVEL SECURITY;

--
-- Name: user_role_assignments user_role_assignments_tenant_isolation; Type: POLICY; Schema: tenants; Owner: -
--

CREATE POLICY user_role_assignments_tenant_isolation ON tenants.user_role_assignments USING ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid)) WITH CHECK ((tenant_id = (NULLIF(current_setting('app.current_tenant_id'::text, true), ''::text))::uuid));


--
-- PostgreSQL database dump complete
--

\unrestrict 9bAq2HaXtKr8wu083tKmht0kwHDUshNlZGMhtg6nCbZ43HkB9eQqA6iyVOlYjsd

