---
name: domain-party-model
description: >-
  Define la lógica de negocio basada en Party Model con aislamiento Multi-Tenant
  y sin herencia de tablas. Úsalo para entender la identidad, roles, tenants,
  suscripciones y estructura.
---

# Modelo de Dominio: Party Model (Siloed Multi-Tenant)

## 1. Principios de Aislamiento (CRÍTICO)
- **Identidad Aislada (Siloed)**: NO existe una identidad global única. Una persona que trabaja para dos tenants tendrá dos filas independientes en `parties`.
- **Un Tenant es una Suscripción**: La tabla `global.subscriptions` representa al tenant.
- Al crear un tenant, se crea una `party` que lo representa.

## 2. Estructura de Esquemas PostgreSQL
- Esquema `global` (Global): `role_types`, `subscriptions` (tenants), `contact_mechanisms` (master data).
- Esquema `tenants` (Multi-tenant con RLS): `parties`, `person_profiles`, `organization_profiles`, `party_roles`, `party_merge_log`, `party_contact_mechanisms`, `addresses`, `user_credentials`, `sessions`, etc.

## 2.1 Roles de sistema para onboarding
- `migrations/0009_role_types_seed.sql` siembra `global.role_types` con:
  - `TENANT`: la organización propietaria del tenant.
  - `ADMIN`: el primer administrador del tenant.
- El onboarding asigna `TENANT` a la party ORGANIZATION y `ADMIN` a la party
  INDIVIDUAL del administrador, dentro de la misma transacción.

## 2.2 Contact Master Data Management
- `global.contact_mechanisms` es el maestro global: solo `id`, `type`, `value`; unique `(type, value)` con collation ICU.
- `tenants.party_contact_mechanisms` relaciona una Party con un contacto global y guarda `is_primary`, `is_verified`, `contact_mechanism_order`, `start_date` y `end_date`.
- La vigencia se expresa con `start_date`/`end_date`; un contacto global solo puede estar activo una vez por tenant.
- `tenants.user_credentials` ya no guarda email: apunta a `party_contact_mechanism_id` (único) y mantiene solo el estado actual de seguridad.
- El login resuelve: email global → relación activa (`end_date IS NULL` y `start_date <= now`) → credencial.

## 3. Modelado SIN Herencia (Composición 1:1)
- No se permite que las tablas de perfil compartan la Primary Key (PK) de `parties`.
- **`parties`**: Tabla maestra. Tiene `id` (UUIDv7), `tenant_id`, `party_type`, y `display_name`.
- **`person_profiles`**: Tiene su propio `id` (UUIDv7), `tenant_id`, `party_id` (FK ÚNICA a `parties`), nombres, apellidos, fecha de nacimiento.
- **`organization_profiles`**: Tiene su propio `id` (UUIDv7), `tenant_id`, `party_id` (FK ÚNICA a `parties`), razón social, nombre comercial, RUC/Tax ID.

## 4. Reglas de Integridad
- Todas las tablas del esquema `tenants` **deben** tener `tenant_id uuid NOT NULL`.
- `party_roles` vincula un `party_id` con un `role_type_id` global.
- Si `party_type = INDIVIDUAL`, debe existir una fila en `person_profiles`. Si es `ORGANIZATION`, en `organization_profiles`.

## 5. Golden Record y Path Compression

- **`canonical_party_id`**: apunta al Golden Record final dentro del mismo tenant. Es `NULL` para el registro canónico y para los registros nunca fusionados.
- **`record_status`**: ciclo de vida de la identidad.
  - `ACTIVE`: registro vigente y visible; puede ser Golden Record.
  - `MERGED`: registro absorbido por otro; tiene `canonical_party_id` y `deleted_at` no nulos.
  - `ARCHIVED`: registro retirado sin fusión; no puede ser destino de una fusión.
- **Destino de la fusión**: debe ser una party `ACTIVE` del mismo tenant. Nunca se fusiona hacia una party `MERGED` o `ARCHIVED`, ni entre tenants distintos.
- **Fusión (`source` → `target`)**:
  1. El `source` pasa a `MERGED`, recibe `canonical_party_id = target` y `deleted_at`.
  2. Todos los registros que apuntaban al `source` se re-apuntan directamente al `target`.
  3. Se registra la operación en `tenants.party_merge_log`.
- **Path Compression**: al fusionar, no se dejan cadenas `A → B → C`. Si `B` se fusiona en `C`, cualquier `A` que apuntaba a `B` se actualiza a `C`. Así los registros antiguos siempre resuelven al Golden Record final en un solo salto.
- **Resolución**: `tenants.resolve_golden_party(p_party_id uuid)` usa `WITH RECURSIVE` para resolver cadenas históricas. Con path compression, la profundidad normal es 1; la recursión es una red de seguridad para datos previos a la compresión.
- **Trazabilidad**: `tenants.party_merge_log` es append-only y guarda `source_party_id`, `target_party_id`, `merged_at` y `merged_by`.
- **Invariantes**:
  - `canonical_party_id` nunca apunta a la propia fila.
  - `canonical_party_id` siempre pertenece al mismo `tenant_id`.
  - Un registro `MERGED` no puede ser destino de otra fusión.
  - La fusión completa (source, compresión y log) ocurre en una sola transacción.
## 6. Estructura Organizacional y Empleo

### Catálogo organizacional
- **`job_families`**: agrupa familias de puestos (`name`, `code`, `description`).
- **`jobs`**: puestos concretos dentro de una familia (`title`, `job_family_id`, `min_salary`, `max_salary`).
- **`departments`**: jerarquía recursiva mediante `parent_department_id`; `manager_party_id` apunta a un empleado del mismo tenant.
- Los códigos (`code`) son únicos por tenant entre registros no eliminados.
- `name`, `title` y los campos de búsqueda usan `COLLATE unaccent_case_insensitive`.
- `min_salary` y `max_salary` son `numeric(14,2)`; si ambos existen, `min_salary <= max_salary` y ninguno es negativo.

### Empleo
- **`employees`**: la PK es `party_id` (1:1 con `parties`), no un `id` independiente. Un empleado es una party `INDIVIDUAL`.
- Campos laborales: `employee_code`, `hire_date`, `status` (`ACTIVE`, `ON_LEAVE`, `TERMINATED`), `job_id`, `department_id` y `manager_party_id` opcional.
- `manager_party_id` es una self-FK a `employees.party_id`; un empleado **no puede ser su propio manager**.
- **`employee_assignments`**: historial de asignaciones (`employee_party_id`, `job_id`, `department_id`, `start_date`, `end_date`, `salary`).
- Una asignación abierta tiene `end_date IS NULL`; un empleado solo puede tener **una** asignación abierta.
- Dos asignaciones del mismo empleado **no pueden solaparse**: se usa `EXCLUDE USING GIST` con `tstzrange(start_date, COALESCE(end_date, 'infinity'), '[)')` y `btree_gist`. Prohibido resolverlo con triggers.
- `salary` es `numeric(14,2)` y no negativo.
- Todas las tablas llevan `tenant_id`, RLS `ENABLE` + `FORCE` y triggers de auditoría.
