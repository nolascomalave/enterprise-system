---
name: multitenancy-rls
description: >-
  Aplica el patrón de esquema compartido con discriminador de tenant y
  Row Level Security (RLS) en PostgreSQL. Úsalo al diseñar tablas,
  políticas de aislamiento o al preguntar sobre multitenencia.
---

# Estándar de Multitenencia con RLS

## 1. Patrón arquitectónico
- **Esquema compartido con columna `tenant_id`** como discriminador.
- Toda tabla de negocio debe incluir `tenant_id uuid NOT NULL` con FK a
  `global.subscriptions(id) ON DELETE RESTRICT`.

## 2. Estructura de tablas
- **Tabla de tenants** (`global.subscriptions`): catálogo global, sin RLS.
- **Tablas de negocio**: cada una con `tenant_id`, campos de auditoría
  (`created_at`, `updated_at`, `deleted_at` como `timestamptz`) e índices
  que comiencen por `tenant_id`.

## 3. Row Level Security (RLS)
- **Siempre** usar `ENABLE ROW LEVEL SECURITY` y `FORCE ROW LEVEL SECURITY`.
- La política debe comparar `tenant_id` con la variable de sesión
  `app.current_tenant_id` usando `NULLIF(current_setting(...), '')::uuid`
  para un comportamiento *fail-closed*.
- La política debe cubrir `FOR ALL` con `USING` y `WITH CHECK`.

## 4. Contexto de sesión
- El backend debe ejecutar `SET LOCAL app.current_tenant_id = '...'` al
  inicio de cada transacción.
- Usar `LOCAL` para evitar fugas entre peticiones en el pool de conexiones.

## 5. Onboarding de un tenant
- `global.subscriptions` no tiene RLS: se inserta primero dentro de la transacción.
- Después se enlaza el nuevo `tenant_id` con `enttenant.WithSessionContext`.
- El wrapper `sessionTx` de `enttenant` aplica `SET LOCAL` de forma perezosa
  antes de la primera consulta con el nuevo contexto; así la misma transacción
  puede pasar de inserts globales a inserts con RLS.
- Todo el grafo (suscripción, parties, perfiles, roles y credenciales) se crea
  en una sola transacción; cualquier fallo hace rollback completo.

## 6. Antipatrones prohibidos
- ❌ Omitir `FORCE ROW LEVEL SECURITY` (el propietario de la tabla evade la RLS).
- ❌ No incluir `tenant_id` en los índices (la RLS siempre filtra por él).
- ❌ Conectar la aplicación como superusuario o rol con `BYPASSRLS`.
- ❌ Usar `current_setting` sin `NULLIF` (riesgo de devolver todas las filas).