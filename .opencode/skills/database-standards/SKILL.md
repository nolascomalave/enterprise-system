---
name: Database Standards (Party Model & Audit)
description: >-
  Aplica el Party Model, soft delete, auditoría con timestamptz,
  normalización, claves foráneas e índices. Úsalo al crear o modificar
  tablas, migraciones o modelos ORM.
---

# Estándar de Base de Datos

## 1. Modelado: Party Model (Identidad vs. Rol)

- **Party**: entidad atómica e inmutable que representa a cualquier actor
  (persona, organización, sistema). Tabla `party` con `id` (UUIDv7 o bigint
  generado), y campos base de auditoría.
- **Roles**: especializaciones de un Party. Por ejemplo `customer`,
  `employee`, `supplier`. Cada rol tiene su propia tabla con `party_id` como
  FK única y `ON DELETE RESTRICT`.
- **Regla**: Nunca mezclar atributos de rol en la tabla `party`. Un Party
  puede tener múltiples roles simultáneos.

## 2. Auditoría y marcas de tiempo

- **Creación**: `created_at timestamptz NOT NULL DEFAULT NOW()`
- **Actualización**: `updated_at timestamptz NOT NULL DEFAULT NOW()`
  (actualizar mediante trigger o a nivel de aplicación).
- **Inactivación / Soft Delete**: `deleted_at timestamptz` (nullable).
  **Prohibido** usar `boolean is_deleted` o `deleted boolean`.
- **Logs de cambios**: tabla `audit_log` con `id`, `table_name`, `record_id`,
  `operation` (INSERT/UPDATE/DELETE), `old_data jsonb`, `new_data jsonb`,
  `changed_by`, `changed_at timestamptz NOT NULL DEFAULT NOW()`. Insertar
  mediante triggers en las tablas auditadas.

## 3. Normalización y claves

- **Normalización**: mínimo 3NF. La desnormalización solo se permite si hay
  evidencia de `EXPLAIN ANALYZE` que demuestre una mejora real de rendimiento.
- **Claves primarias**: `bigint GENERATED ALWAYS AS IDENTITY` o `UUIDv7`.
  Evitar UUIDv4 aleatorio como PK (fragmentación de índice).
- **Claves foráneas**: siempre explícitas. `ON DELETE RESTRICT` por defecto;
  `ON DELETE CASCADE` solo cuando la relación sea de composición fuerte y
  esté justificado.
- **Índices**:
  - Índice parcial para soft delete: `CREATE INDEX ... ON tabla (id) WHERE deleted_at IS NULL`.
  - Índices compuestos ordenados por selectividad (columna más selectiva primero).
  - Cada índice debe citar la consulta que sirve; prohibido crear índices
    “por si acaso”.

## 4. Antipatrones prohibidos

- ❌ `timestamp` sin zona horaria (usar siempre `timestamptz`).
- ❌ `boolean` para soft delete.
- ❌ `varchar(255)` sin justificación; usar `text` con `CHECK` si aplica.
- ❌ `float` o `double` para dinero; usar `numeric(precision, scale)`.
- ❌ Ignorar la normalización o desnormalizar sin evidencia.
- ❌ Crear índices sin una consulta objetivo clara.
- ❌ Usar `SELECT *` en producción.

## 5. Flujo de trabajo al generar DDL

1. Identificar Party(s) y Roles involucrados.
2. Aplicar los campos de auditoría en **todas** las tablas.
3. Definir FKs y sus políticas `ON DELETE`/`ON UPDATE`.
4. Diseñar índices basados en los patrones de consulta conocidos.
5. Verificar que no se esté usando ningún antipatrón de la lista.
6. Si hay logs de auditoría, generar el trigger correspondiente.