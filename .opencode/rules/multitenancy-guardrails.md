# Multitenancy Guardrails (MANDATORY)

- Toda tabla de negocio **debe** tener `tenant_id uuid NOT NULL` con FK a
  `app.tenants(tenant_id) ON DELETE RESTRICT`.
- **Obligatorio** habilitar y forzar RLS: `ENABLE` + `FORCE ROW LEVEL SECURITY`.
- La política de aislamiento **debe** usar `NULLIF(current_setting('app.current_tenant_id', true), '')::uuid`.
- **Prohibido** conectar la aplicación como superusuario o rol con `BYPASSRLS`.
- Los índices de tablas con RLS **deben** comenzar por `tenant_id`.
- El backend **debe** establecer `SET LOCAL app.current_tenant_id` al inicio
  de cada transacción.