---
name: backend-go-ent
description: >-
  Define la arquitectura y el stack backend: Go, Clean Architecture, TDD,
  Gin (HTTP) y Ent (ORM). Úsalo para escribir código, tests, esquemas de Ent
  o endpoints.
---

# Stack Backend: Go + Clean Architecture + Ent + TDD

## 1. Arquitectura Limpia (Clean Architecture)
El código debe organizarse estrictamente en capas (de adentro hacia afuera):
- `internal/domain`: Entidades y reglas de negocio (independiente de frameworks). Define las interfaces de los repositorios.
- `internal/usecase`: Lógica de aplicación. Orquesta el flujo entre controladores y repositorios.
- `internal/infrastructure`: Implementación de repositorios usando Ent.
- `internal/delivery`: Handlers HTTP usando Gin (rutas, middlewares, DTOs).

### Unit of Work
- Los casos de uso que crean varias entidades relacionadas deben recibir una
  interfaz `UnitOfWork` y ejecutar todo dentro de una única transacción.
- La implementación Ent hace `Begin -> callback -> Commit` y `Rollback` ante
  cualquier error. El caso de uso no debe conocer `*ent.Tx`.

## 2. Test-Driven Development (TDD)
- **Obligatorio** escribir tests antes o junto con la implementación.
- Usar `stretchr/testify` para aserciones y mocks.
- Los casos de uso (`usecase`) deben tener tests unitarios exhaustivos con repositorios mockeados.
- Los endpoints de Gin deben tener tests de integración (usando `httptest`).

## 3. Ent (ORM)
- Los esquemas se definen en Go (paquete `ent/schema`).
- **Regla de Guardrail**: Los esquemas de Ent deben reflejar exactamente las reglas de DDL (ej. usar `field.UUID("id").DefaultFunc(uuid.NewV7)` en lugar de autoincrementales).
- Para el soft delete, Ent tiene soporte nativo con `field.Time("deleted_at").Optional()`. Puedes usar la feature `ent.SoftDelete` (Mixin) si aplica.
- Para las políticas RLS, se deben definir en las migraciones SQL de Ent o mediante `entsql.Annotation`.
- Para la lógica multitenant, usar un hook global en Ent que inyecte el `tenant_id` en el contexto de la transacción.

## 4. Gin
- Mantener los handlers ligeros. Solo validan el request (usando `binding`) y llaman al `usecase`.
- Manejar errores de dominio y mapearlos a códigos HTTP correctos (400, 404, 409, 500).

## 5. Resolución de Conflictos con el Documento de Diseño (v1.0)
- El documento original usaba `BIGINT` o `VARCHAR` o `TIMESTAMP`. El agente **debe ignorar eso** y usar `UUIDv7`, `text` y `timestamptz` según los guardrails.
- El documento original no tenía `tenant_id`. **Todas** las tablas deben llevar `tenant_id` y RLS, salvo que se indique lo contrario.