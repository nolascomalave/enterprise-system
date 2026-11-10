---
name: session-multitenancy
description: >-
  Aplica el patrón de tabla de sesiones con ámbito de tenant y Row Level
  Security (RLS) en PostgreSQL. Úsalo al diseñar la tabla de sesiones,
  políticas de aislamiento o al preguntar sobre autenticación multitenencia.
---

# Estándar de Sesiones Multitenencia

## 1. Estructura de la tabla `tenants.sessions`
- **`id uuid PRIMARY KEY DEFAULT uuidv7()`**: Identificador de la sesión.
- **`tenant_id uuid NOT NULL REFERENCES global.subscriptions(id) ON DELETE RESTRICT`**: Discriminador obligatorio para RLS.
- **`user_id uuid NOT NULL REFERENCES tenants.parties(id) ON DELETE CASCADE`**: Party propietaria de la sesión; el CASCADE invalida sus sesiones al eliminar la party.
- **`token_hash bytea NOT NULL UNIQUE`**: Solo se almacena el hash del token (SHA-256), nunca el token en claro.
- **`expires_at timestamptz NOT NULL`** con `CHECK (expires_at > created_at)`.
- **`revoked_at timestamptz`**: Para invalidación explícita (logout, seguridad).
- **`created_at`, `updated_at`, `deleted_at`, `last_used_at timestamptz`** para ciclo de vida y soft delete.
- **`metadata jsonb NOT NULL DEFAULT '{}'`** con `CHECK (jsonb_typeof(metadata) = 'object')` para IP, user-agent u otra información contextual.

## 2. Índices obligatorios
- `(tenant_id)` para que la política RLS lo use eficientemente.
- `(token_hash)` para la validación rápida del token en cada petición autenticada. Es la excepción documentada a "los índices empiezan por tenant_id": el token se resuelve antes de conocer el tenant.
- `(tenant_id, user_id) WHERE revoked_at IS NULL AND deleted_at IS NULL` para listar sesiones activas de una party dentro del tenant.

## 3. Row Level Security (RLS)
- **Siempre** usar `ENABLE ROW LEVEL SECURITY` y `FORCE ROW LEVEL SECURITY`.
- La política debe cubrir `FOR ALL` con `USING` y `WITH CHECK`, comparando `tenant_id` con `NULLIF(current_setting('app.current_tenant_id', true), '')::uuid` para un comportamiento *fail-closed*.

## 4. Contexto de sesión
- Al inicio de cada transacción, el backend debe ejecutar:
  ```sql
  SET LOCAL app.current_tenant_id = 'uuid-del-tenant';
  SET LOCAL app.current_session_id = 'uuid-de-la-sesion';
  SET LOCAL app.current_user_id = 'uuid-del-usuario';
  ```
- Usar `LOCAL` para evitar fugas entre peticiones en el pool de conexiones.


## 5. Tokens de recuperación de contraseña
- Viven en `tenants.password_reset_tokens` (migración `0008_password_reset_tokens.sql`).
- Solo se guarda `token_hash bytea` (SHA-256); el token en claro solo se envía por correo.
- Son de un solo uso: `used_at` se marca atómicamente al consumirlos.
- Deben expirar (`expires_at`) y tener RLS forzado por `tenant_id`.
- Al consumir un token se revocan **todas** las sesiones del usuario.
- En un cambio de contraseña autenticado se revocan todas las sesiones
  **excepto** la sesión actual.
- No se escribe `token_hash` en la auditoría; usar
  `audit.fn_log_change('token_hash')`.


## 6. Antipatrones prohibidos
- ❌ Almacenar el token de sesión en texto plano (usar siempre `token_hash`).
- ❌ Omitir `FORCE ROW LEVEL SECURITY` (el propietario de la tabla evade la RLS).
- ❌ No incluir `tenant_id` en los índices (la RLS siempre filtra por él), salvo la excepción documentada de `token_hash`.
- ❌ Conectar la aplicación como superusuario o rol con `BYPASSRLS`.


> **Nota:** El DDL de referencia vive en `migrations/0001_app_sessions.sql` (tabla `tenants.sessions`).
