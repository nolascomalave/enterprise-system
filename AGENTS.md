<!-- BEGIN:project-context -->
## Contexto del Proyecto
- Somos un sistema empresarial basado en Party Model.
- Backend en Go, Clean Architecture, TDD, Gin y Ent.
- Para tareas de negocio, consulta `.opencode/skills/domain-party-model/SKILL.md`.
- Para tareas de código, consulta `.opencode/skills/backend-go-ent/SKILL.md`.

## Transacciones (OBLIGATORIO)
- Carga y aplica `.opencode/rules/transaction-guardrails.md` **antes** de
  tocar transacciones, repositorios o casos de uso de escritura.
- Carga el skill `.opencode/skills/transaction-management/SKILL.md` para el
  patrón completo.
- `WithinTransaction` se **une** a la transacción ambiente; `WithinSavepoint`
  es para bloques aislados tipo *try/catch*.
- Nunca abras `client.Tx` fuera del `TransactionManager`. Las lecturas con RLS
  también van dentro de una transacción.

## Manejo de Errores (OBLIGATORIO)
- Carga y aplica `.opencode/rules/error-handling-guardrails.md` y el skill
  `.opencode/skills/error-handling/SKILL.md` siempre que crees o modifiques
  código que pueda producir errores.
- Los handlers solo llaman a `c.Error(err)`; el `ErrorMiddleware` es el único
  lugar que mapea errores a códigos HTTP.
- Nunca devuelvas `err.Error()` ni detalles técnicos en una respuesta HTTP.
  Los errores internos se loguean completos y el cliente recibe un 500
  genérico.
- Usa `internal/common/apperror`: centinelas en dominio, `NewNotFound`/
  `NewValidation`/`NewUnauthorized` para errores de negocio y `NewInternal`
  para fallos técnicos.
<!-- END:project-context -->


<!-- BEGIN:database-guardrails -->
## Base de Datos y Auditoría
- Para cualquier tarea relacionada con **auditoría de datos** (creación de
  tablas de logs, triggers de seguimiento, preguntas sobre trazabilidad),
  carga y sigue estrictamente el skill `audit-logging`.
- Consulta el archivo `.opencode/skills/audit-logging/SKILL.md` para el DDL,
  la función de trigger y las restricciones de inmutabilidad.
- No propongas soluciones de auditoría que usen FKs a sesiones o usuarios
  en la tabla de logs; el historial debe ser independiente.
- Carga SIEMPRE el skill `project-guardrails` antes de generar DDL o migraciones.
- Usa el plugin `db-connector` para ejecutar cambios en PostgreSQL.

## Multitenencia y RLS
- Para cualquier tarea relacionada con **multitenencia**, diseño de tablas
  con `tenant_id` o políticas de Row Level Security, carga y sigue
  estrictamente el skill `multitenancy-rls`.
- Consulta el archivo `.opencode/skills/multitenancy-rls/SKILL.md` para el
  DDL, las políticas y las restricciones.

## Sesiones y Multitenencia
- Para cualquier tarea relacionada con **sesiones de usuario**, diseño de la tabla `tenants.sessions`, políticas de RLS o autenticación multitenencia, carga y sigue estrictamente el skill `session-multitenancy`.
- Consulta el archivo `.opencode/skills/session-multitenancy/SKILL.md` para el DDL, las políticas y las restricciones.

## Reglas Obligatorias de Base de Datos (CRÍTICO)
- **SIEMPRE** que vayas a generar DDL, migraciones, triggers, políticas RLS o código SQL, DEBES cargar y aplicar el skill `project-guardrails`.
- Este skill contiene las restricciones innegociables del proyecto (RLS forzado, `timestamptz`, soft delete, Party Model, etc.).

## Arquitectura de Datos (Regla de Oro)
- Las tablas de catálogo (`role_types`) son globales (sin `tenant_id`).
- Las tablas de identidad (`parties`, `people`, `organizations`, `party_roles`, etc.) viven en el esquema `tenants` y **SIEMPRE** llevan `tenant_id` con RLS aplicado.
- La tabla `subscriptions` vive en el esquema `global` y representa a los Tenants.
- Un empleado que trabaja en dos tenants diferentes tendrá dos filas separadas en `parties`, una por cada `tenant_id`.
<!-- END:database-guardrails -->