---
name: project-guardrails
description: >-
  Reglas obligatorias del proyecto para el diseño y modificación de bases de datos.
  Aplica para auditoría, multitenencia, sesiones, RLS, modelado, collations ICU 
  y restricciones de exclusión. Úsalo SIEMPRE que vayas a generar DDL, 
  migraciones, triggers, políticas RLS o código SQL.
---

# Project Guardrails (MANDATORY)

Estas reglas son de obligado cumplimiento. **Nunca** generes una estructura de base de datos que viole estos principios.

## 1. Multitenencia y RLS
- Toda tabla de negocio **debe** tener `tenant_id uuid NOT NULL` con FK a `app.tenants(tenant_id) ON DELETE RESTRICT`.
- **Obligatorio** habilitar y forzar RLS: `ENABLE` + `FORCE ROW LEVEL SECURITY`.
- La política de aislamiento **debe** usar `NULLIF(current_setting('app.current_tenant_id', true), '')::uuid` (comportamiento fail-closed).
- Los índices de tablas con RLS **deben** comenzar por `tenant_id`.
- El backend **debe** establecer `SET LOCAL app.current_tenant_id` al inicio de cada transacción.
- **Prohibido** conectar la aplicación como superusuario o rol con `BYPASSRLS`.

## 2. Sesiones
- La tabla de sesiones **debe** tener `tenant_id uuid NOT NULL`.
- El token de sesión **nunca** se almacena en texto plano; solo su `token_hash` (ej. SHA-256).

## 3. Auditoría y Logs
- La tabla de auditoría **debe** estar particionada por `changed_at`.
- Los campos de fecha deben ser `timestamptz`. `changed_at` debe ser `NOT NULL DEFAULT clock_timestamp()`.
- **Prohibido** definir claves foráneas hacia `session_id` o `user_access_id` en la tabla de logs.
- La tabla de auditoría es **append-only**.

## 4. Modelado General y Timestamps
- **Claves Primarias (PK)**: Usar siempre `id uuid PRIMARY KEY DEFAULT uuidv7()`.
- **Timestamps**: Todos los campos de fecha deben ser `timestamptz`. Los campos `created_at` y `updated_at` deben ser `NOT NULL DEFAULT NOW()`.
- **Soft delete**: Se implementa exclusivamente con `deleted_at timestamptz`.
- **Claves foráneas**: `ON DELETE RESTRICT` por defecto; `CASCADE` requiere justificación.
- **Índices**: Debe existir al menos un índice parcial para soft delete (`WHERE deleted_at IS NULL`).

## 5. Antipatrones Prohibidos
- ❌ `timestamp` sin zona horaria.
- ❌ `float` para dinero.
- ❌ `varchar(255)` sin motivo.
- ❌ `SELECT *`.

## 6. Collations para Búsquedas Insensibles (NUEVO)
- La base de datos cuenta con la collation `unaccent_case_insensitive` (ICU, no determinista).
- **Regla**: En las consultas a la API, cualquier consulta que se realice comparando un campo con un texto de búsqueda proveniente de un cliente, debe aplicarse esta collation sobre el campo comparado.
- **Ejemplo**: `name text COLLATE unaccent_case_insensitive LIKE %cómo respirar%`

## 7. Restricciones de Exclusión y Fechas (NUEVO)
- La base de datos tiene instalada la extensión `btree_gist`.
- Para evitar solapamiento de fechas (ej. vigencias, contratos, horarios), **prohibido** usar triggers. Se **debe** usar una restricción `EXCLUDE USING GIST`.
- **Ejemplo**:
  ```sql
  ALTER TABLE tenants.shareholding_validity
    ADD CONSTRAINT shareholding_validity_uq_no_overlap
    EXCLUDE USING GIST (
      shareholding_id WITH =,
      tstzrange(start_date, COALESCE(end_date, 'infinity'::timestamptz), '[)') WITH &&
    );