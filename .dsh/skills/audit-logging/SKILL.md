---
name: audit-logging
description: >-
  Aplica un sistema de auditoría centralizado, particionado e inmutable
  para registrar cambios (quién, qué, cuándo) en tablas PostgreSQL.
  Úsalo al crear tablas de auditoría, triggers de logging o al
  preguntar sobre trazabilidad de datos.
---

# Estándar de Auditoría de Datos

## 1. Arquitectura
- **Tabla centralizada** en un esquema dedicado `audit` (nunca columnas
  `created_by`/`updated_by` en cada tabla de negocio).
- **Particionado por rango** de `changed_at` (mensual) para escalabilidad
  y retención eficiente.
- **Inmutabilidad** garantizada: la tabla es *append-only*. Se prohíben
  `UPDATE` y `DELETE` mediante un trigger de denegación.

## 2. Contexto de la aplicación (el "quién")
- El backend debe establecer, al inicio de cada transacción:
  ```sql
  SELECT set_config('app.current_user_id', '...', true);
  SELECT set_config('app.current_session_id', '...', true);
  ```
- El trigger lee estos valores con `current_setting('app.current_user_id', true)`.
- **Sin claves foráneas** a `session_id` ni `user_access_id`: se guardan como
  valores para preservar el historial incluso si el usuario o la sesión
  se eliminan.

## 3. DDL y función de trigger
(El DDL completo, la tabla `audit.change_log` y la función
`audit.fn_log_change()` viven en `migrations/0003_audit_change_log.sql`).

## 4. Redacción de campos sensibles
`audit.fn_log_change()` acepta nombres de columnas en `TG_ARGV` y elimina esas
claves de `old_data`, `new_data` y `changed_fields`. Úsalo para secretos:

```sql
CREATE TRIGGER user_credentials_audit_trg
  AFTER INSERT OR UPDATE OR DELETE ON tenants.user_credentials
  FOR EACH ROW EXECUTE FUNCTION audit.fn_log_change(
    'password_hash', 'totp_secret', 'backup_codes_hash'
  );
```

Nunca audites tokens, hashes de refresh tokens, contraseñas ni secretos TOTP.

## 5. Antipatrones prohibidos
- ❌ `session_id` con FK y `ON DELETE SET NULL`.
- ❌ Tabla de auditoría sin particionado.
- ❌ Permitir `UPDATE`/`DELETE` sobre la tabla de logs.
- ❌ Usar `timestamp` sin zona horaria (siempre `timestamptz`).

> **Nota:** Puedes pegar el DDL completo y la función de trigger directamente en la sección 3, o guardarlos en un archivo separado (ej. `docs/audit_ddl.sql`) y referenciarlo. El agente podrá leerlo si se lo indicas.