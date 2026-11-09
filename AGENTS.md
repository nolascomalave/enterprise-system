<!-- BEGIN:database-guardrails -->

## Base de Datos y Auditoría
- Para cualquier tarea relacionada con **auditoría de datos** (creación de
  tablas de logs, triggers de seguimiento, preguntas sobre trazabilidad),
  carga y sigue estrictamente el skill `audit-logging`.
- Consulta el archivo `.dsh/skills/audit-logging/SKILL.md` para el DDL,
  la función de trigger y las restricciones de inmutabilidad.
- No propongas soluciones de auditoría que usen FKs a sesiones o usuarios
  en la tabla de logs; el historial debe ser independiente.
- Carga SIEMPRE el skill `project-guardrails` antes de generar DDL o migraciones.
- Usa el plugin `db-connector` para ejecutar cambios en PostgreSQL.

## Multitenencia y RLS
- Para cualquier tarea relacionada con **multitenencia**, diseño de tablas
  con `tenant_id` o políticas de Row Level Security, carga y sigue
  estrictamente el skill `multitenancy-rls`.
- Consulta el archivo `.dsh/skills/multitenancy-rls/SKILL.md` para el
  DDL, las políticas y las restricciones.

## Sesiones y Multitenencia
- Para cualquier tarea relacionada con **sesiones de usuario**, diseño de la tabla `app.sessions`, políticas de RLS o autenticación multitenencia, carga y sigue estrictamente el skill `session-multitenancy`.
- Consulta el archivo `.dsh/skills/session-multitenancy/SKILL.md` para el DDL, las políticas y las restricciones.

## Reglas Obligatorias de Base de Datos (CRÍTICO)
- **SIEMPRE** que vayas a generar DDL, migraciones, triggers, políticas RLS o código SQL, DEBES cargar y aplicar el skill `project-guardrails`.
- Este skill contiene las restricciones innegociables del proyecto (RLS forzado, `timestamptz`, soft delete, Party Model, etc.).

<!-- END:database-guardrails -->