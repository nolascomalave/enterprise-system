# Audit Guardrails (MANDATORY)

Estas reglas son de obligado cumplimiento en cualquier cambio que afecte a la
estructura de la base de datos (DDL, migraciones, modelos ORM).

- **Timestamps**: Todos los campos de fecha deben ser `timestamptz`. Los
  campos `created_at` y `updated_at` deben ser `NOT NULL DEFAULT NOW()`.
- **Soft delete**: Se implementa exclusivamente con `deleted_at timestamptz`.
  Queda prohibido `boolean is_deleted`, `deleted boolean` o similares.
- **Auditoría**: Toda tabla debe tener `created_at`, `updated_at` y, si aplica
  borrado lógico, `deleted_at`. Las tablas que requieran trazabilidad de
  cambios deben tener una tabla de logs (`audit_log`) poblada por triggers.
- **Party Model**: La identidad se modela en `party`; los roles en tablas
  separadas con FK a `party`. No se permite duplicar atributos de identidad en
  las tablas de rol.
- **Claves foráneas**: Toda relación debe tener FK explícita. `ON DELETE RESTRICT`
  es el valor por defecto; `CASCADE` requiere justificación documentada.
- **Índices**: Debe existir al menos un índice parcial para soft delete
  (`WHERE deleted_at IS NULL`) en tablas con borrado lógico.
- **Antipatrones**: Rechazar cualquier propuesta que use `timestamp` sin
  timezone, `float` para dinero, `varchar(255)` sin motivo o `SELECT *`.
- La tabla de auditoría **debe** estar particionada por `changed_at`.
- **Prohibido** definir claves foráneas hacia `session_id` o `user_access_id`
  en la tabla de logs. Los IDs se guardan como valores, no como relaciones.
- La tabla de auditoría es **append-only**: rechazar cualquier `UPDATE` o
  `DELETE` mediante un trigger y revocar permisos al rol de la aplicación.
- Todos los campos de fecha deben ser `timestamptz`. `changed_at` debe ser
  `NOT NULL DEFAULT clock_timestamp()`.