---
name: session-multitenancy
description: >-
  Aplica el patrón de tabla de sesiones con ámbito de tenant y Row Level
  Security (RLS) en PostgreSQL. Úsalo al diseñar la tabla de sesiones,
  políticas de aislamiento o al preguntar sobre autenticación multitenencia.
---

# Estándar de Sesiones Multitenencia

## 1. Estructura de la tabla `app.sessions`
- **`id uuid PRIMARY KEY DEFAULT uuidv7()`**: Identificador de la sesión.
- **`tenant_id uuid NOT NULL REFERENCES app.tenants(tenant_id) ON DELETE RESTRICT`**: Discriminador obligatorio para RLS.
- **`user_id uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE`**: Usuario propietario.
- **`token_hash bytea NOT NULL UNIQUE`**: Solo se almacena el hash del token (SHA-256), nunca el token en claro.
- **`expires_at timestamptz NOT NULL`** con `CHECK (expires_at > created_at)`.
- **`revoked_at timestamptz`**: Para invalidación explícita (logout, seguridad).
- **`created_at`, `last_used_at timestamptz`** para ciclo de vida.
- **`metadata jsonb DEFAULT '{}'`** para IP, user-agent u otra información contextual.

## 2. Índices obligatorios
- `(tenant_id)` para que la política RLS lo use eficientemente.
- `(token_hash)` para la validación rápida del token en cada petición autenticada.
- `(user_id) WHERE revoked_at IS NULL` para listar sesiones activas de un usuario.

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


## 5. Antipatrones prohibidos
- ❌ Almacenar el token de sesión en texto plano (usar siempre `token_hash`).
- ❌ Omitir `FORCE ROW LEVEL SECURITY` (el propietario de la tabla evade la RLS).
- ❌ No incluir `tenant_id` en los índices (la RLS siempre filtra por él).
- ❌ Conectar la aplicación como superusuario o rol con `BYPASSRLS`.


> **Nota:** Puedes incluir el DDL completo dentro del Skill o guardarlo en un archivo `docs/sessions_ddl.sql` y referenciarlo. El agente podrá leerlo si se lo indicas.