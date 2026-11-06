# Session Guardrails (MANDATORY)

- La tabla de sesiones **debe** tener `tenant_id uuid NOT NULL` con FK a `app.tenants(tenant_id) ON DELETE RESTRICT`.
- **Obligatorio** habilitar y forzar RLS: `ENABLE` + `FORCE ROW LEVEL SECURITY`.
- La política de aislamiento **debe** usar `NULLIF(current_setting('app.current_tenant_id', true), '')::uuid`.
- El token de sesión **nunca** se almacena en texto plano; solo su `token_hash` (ej. SHA-256).
- **Prohibido** conectar la aplicación como superusuario o rol con `BYPASSRLS`.
- Los índices de `app.sessions` **deben** comenzar por `tenant_id`.
- El backend **debe** establecer `SET LOCAL app.current_tenant_id` al inicio de cada transacción.