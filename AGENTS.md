<!-- BEGIN:database-audit-rules -->

## Base de Datos y Auditoría
- Para cualquier tarea relacionada con **auditoría de datos** (creación de
  tablas de logs, triggers de seguimiento, preguntas sobre trazabilidad),
  carga y sigue estrictamente el skill `audit-logging`.
- Consulta el archivo `.opencode/skills/audit-logging/SKILL.md` para el DDL,
  la función de trigger y las restricciones de inmutabilidad.
- No propongas soluciones de auditoría que usen FKs a sesiones o usuarios
  en la tabla de logs; el historial debe ser independiente.

## Reglas de Auditoría
- Aplica las restricciones de la regla `audit-guardrails` para cualquier tarea
  relacionada con la creación de tablas de logs, triggers de auditoría o
  trazabilidad de datos.
- Referencia: @.opencode/rules/audit-guardrails.md

<!-- END:database-audit-rules -->


<!-- BEGIN: database-multitenancy-rules -->

## Multitenencia y RLS
- Para cualquier tarea relacionada con **multitenencia**, diseño de tablas
  con `tenant_id` o políticas de Row Level Security, carga y sigue
  estrictamente el skill `multitenancy-rls`.
- Consulta el archivo `.opencode/skills/multitenancy-rls/SKILL.md` para el
  DDL, las políticas y las restricciones.
- Aplica las restricciones obligatorias de la regla `multitenancy-guardrails`.
- Referencia: @.opencode/rules/multitenancy-guardrails.md

<!-- END: database-multitenancy-rules -->


<!-- BEGIN: database-multitenancy-sessions-rules -->

## Sesiones y Multitenencia
- Para cualquier tarea relacionada con **sesiones de usuario**, diseño de la tabla `app.sessions`, políticas de RLS o autenticación multitenencia, carga y sigue estrictamente el skill `session-multitenancy`.
- Consulta el archivo `.opencode/skills/session-multitenancy/SKILL.md` para el DDL, las políticas y las restricciones.
- Aplica las restricciones obligatorias de la regla `session-guardrails`.
- Referencia: @.opencode/rules/session-guardrails.md

<!-- END: database-multitenancy-sessions-rules -->