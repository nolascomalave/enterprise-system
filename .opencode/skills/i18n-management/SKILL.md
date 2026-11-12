---
name: i18n-management
description: >-
  Estandariza la internacionalización de catálogos y entidades con nombres o
  descripciones visibles. Úsalo al crear o modificar entidades globales,
  catálogos compartidos o entidades tenant que necesiten traducciones.
---

# i18n Management: traducciones normalizadas y fallback

> **Regla obligatoria:** aplica este patrón siempre que una entidad tenga un
> nombre o descripción visible para el usuario.

## 1. Patrón de tablas

Toda entidad global o catálogo compartido con textos visibles debe tener una
tabla hermana:

```sql
CREATE TABLE global.<entidad>_translations (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  <entidad>_id  uuid NOT NULL REFERENCES global.<entidad>(id) ON DELETE RESTRICT,
  locale_code   varchar(10) NOT NULL REFERENCES global.locales(code) ON DELETE RESTRICT,
  name          text NOT NULL,
  description   text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (<entidad>_id, locale_code)
);
```

- La tabla principal conserva solo datos invariantes (código, scope, estado).
- `name`/`description` viven en la tabla de traducciones.
- `locale_code` es BCP 47 y referencia `global.locales(code)`.
- Para entidades tenant, la tabla incluye `tenant_id`, RLS, auditoría y una FK
  compuesta `(<entidad>_id, tenant_id)` para impedir cruces entre tenants.
- Los catálogos globales no llevan RLS.

## 2. Fallback en la capa de casos de uso

El repositorio busca en este orden:

1. traducción del locale solicitado;
2. traducción del locale marcado `is_default = true` en `global.locales`;
3. valor base de la tabla principal (o el `code` si el nombre se movió a
   traducciones).

El caso de uso orquesta la cadena; el repositorio solo expone las consultas
necesarias (`Get<Entity>Translation`, `GetDefaultLocale`). Nunca se resuelve el
fallback en SQL ni en el handler.

## 3. LocaleMiddleware

- Extrae `Accept-Language`, respeta q-values y normaliza variantes
  (`en-US → en`, `pt-BR`).
- Fallback a `es` si no hay coincidencia.
- Inyecta el locale en `context.Context`.
- Los handlers leen el locale con `LocaleFromContext(ctx)` y lo pasan al caso
  de uso; no acceden directamente al header.

## 4. Tests obligatorios

- Traducción directa en el locale solicitado.
- Fallback al locale por defecto.
- Fallback al valor base.
- Listados completos: p. ej. `Accept-Language: en` devuelve `Passport` y
  `Accept-Language: es` devuelve `Pasaporte`.
