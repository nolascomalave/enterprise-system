# Enterprise System API

API multi-tenant basada en **Party Model**, escrita en Go con Clean Architecture, Gin, Ent y PostgreSQL 18.

## Stack

- Go 1.25
- Gin
- Ent
- PostgreSQL 18 (`uuidv7`, RLS, particiones de auditoría)
- JWT + refresh tokens rotativos + TOTP
- Swagger/OpenAPI

## Requisitos

- Go 1.25+
- PostgreSQL 18+
- `psql`
- `swag` (se ejecuta vía `go run` desde el Makefile)
- Opcional: Atlas para aplicar el esquema Ent

## Variables de entorno

| Variable | Obligatoria | Default | Descripción |
|---|---:|---|---|
| `DATABASE_URL` | Sí | — | DSN de PostgreSQL |
| `JWT_SECRET` | Sí | — | Secreto HS256 de al menos 32 bytes |
| `JWT_ISSUER` | No | `enterprise-system` | Emisor de los JWT |
| `PORT` | No | `8080` | Puerto HTTP |
| `COOKIE_SECURE` | No | `true` | `false` solo para HTTP local |
| `CORS_ALLOWED_ORIGINS` | No | `http://localhost:3000` | Orígenes exactos separados por coma; `*` permite todos sin credenciales |
| `TRUSTED_PROXIES` | No | vacío | IPs/CIDRs de proxies confiables |
| `SHUTDOWN_TIMEOUT` | No | `10s` | Tiempo máximo de graceful shutdown |
| `GIN_MODE` | No | `debug` | Usa `release` en producción |
| `ATLAS_DEV_DATABASE_URL` | Solo Atlas | — | Base de desarrollo vacía para `atlas schema apply` |

Ejemplo para desarrollo local:

```bash
export DATABASE_URL='postgresql://ai_mcp_user:password@localhost:5432/enterprise_system'
export JWT_SECRET='01234567890123456789012345678901'
export COOKIE_SECURE=false
export CORS_ALLOWED_ORIGINS='http://localhost:3000'
export GIN_MODE=release
go run ./cmd/api
```

## Diagrama de base de datos

La estructura completa (19 tablas, RLS, auth, organización, Contact MDM y auditoría) está en:

- [Diagrama ER](docs/database/README.md)
- [PNG](docs/database/database-er.png)
- [SVG](docs/database/database-er.svg)
- [Fuente Mermaid](docs/database/database-er.mmd)

## Base de datos y migraciones

El esquema final se aplica en dos fases:

1. **Esquema Ent con Atlas** (18 tablas: `global`, `tenants`):

```bash
atlas schema apply \
  -u "$DATABASE_URL?sslmode=disable" \
  --dev-url "$ATLAS_DEV_DATABASE_URL" \
  --to "ent://ent/schema" \
  --auto-approve
```

2. **RLS, auditoría, checks y collation**:

```bash
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f migrations/0002_party_model_rls.sql
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f migrations/0003_audit_change_log.sql
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/atlas_post_schema.sql
```

El script [scripts/atlas_post_schema.sql](scripts/atlas_post_schema.sql) completa lo que Ent no expresa: RLS en las 15 tablas tenant, políticas fail-closed, checks, `EXCLUDE USING GIST`, triggers de auditoría, collation ICU, `resolve_golden_party`, los roles `TENANT`/`ADMIN` y la unicidad case-insensitive del contacto global.

Verificación:

```bash
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f scripts/verify_schema.sql
```

> Las migraciones `0001` y `0004`–`0008` son etapas históricas incrementales. No deben aplicarse literalmente después del esquema Ent final porque contienen `CREATE TABLE`/`ALTER` de versiones anteriores. El estado final está cubierto por Atlas + `atlas_post_schema.sql`.

> No conectes la aplicación con un rol superusuario ni con `BYPASSRLS`.

## Ejecutar

```bash
make run
# o
go run ./cmd/api
```

El servidor configura timeouts HTTP y realiza **graceful shutdown** ante `SIGINT`/`SIGTERM` usando `http.Server.Shutdown` con `SHUTDOWN_TIMEOUT`.

## Health check

```bash
curl http://localhost:8080/api/v1/health
```

```json
{"data":{"status":"ok","service":"enterprise-system","timestamp":"2026-11-10T12:00:00Z"}}
```

## Swagger / OpenAPI

Generar o regenerar la documentación:

```bash
make swagger
```

La documentación se sirve en:

```text
http://localhost:8080/swagger/index.html
```

En Swagger UI usa **Authorize** con `Bearer <access_token>` para probar los endpoints protegidos.

El comando ejecuta:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.4 init \
  -g cmd/api/main.go -o docs --parseDependency --parseInternal
```

Los archivos `docs/docs.go`, `docs/swagger.json` y `docs/swagger.yaml` se versionan.

## Endpoints principales

| Método | Ruta | Auth | Descripción |
|---|---|---|---|
| `GET` | `/api/v1/health` | No | Liveness |
| `GET` | `/api/v1/locations` | No | Jerarquía geográfica; acepta `country`, `parent_id` y `Accept-Language` |
| `POST` | `/api/v1/auth/login` | No | Login; puede devolver `temp_token` para 2FA |
| `POST` | `/api/v1/auth/2fa/verify` | No | Verificación TOTP/backup code |
| `POST` | `/api/v1/auth/refresh` | Cookie | Rota refresh token |
| `POST` | `/api/v1/auth/logout` | Cookie | Revoca la sesión |
| `POST` | `/api/v1/auth/forgot-password` | No | Solicita reset |
| `POST` | `/api/v1/auth/reset-password` | No | Consume token de reset |
| `POST` | `/api/v1/auth/change-password` | JWT | Cambia contraseña |
| `POST` | `/api/v1/onboarding/register` | No | Provisiona tenant + admin |
| `POST` | `/api/v1/client-applications` | JWT | Crea una app cliente OAuth M2M |
| `POST` | `/api/v1/client-applications/first-party` | Platform key | Crea app oficial FIRST_PARTY (PLATFORM o tenant) |
| `POST` | `/api/v1/oauth/token` | No | Client Credentials: emite JWT M2M |
| `GET` | `/api/v1/platform/status` | M2M `platform:admin` | Comprueba acceso de plataforma |
| `POST` | `/api/v1/parties` | JWT | Crea party |
| `GET` | `/api/v1/parties` | JWT | Lista parties |
| `GET` | `/api/v1/parties/{id}` | JWT | Obtiene party |
| `POST` | `/api/v1/parties/merge` | JWT | Fusiona source en target (Golden Record) |
| `GET` | `/api/v1/jobs` | JWT | Lista jobs |
| `POST` | `/api/v1/jobs` | JWT | Crea job |
| `PUT` | `/api/v1/jobs/{id}` | JWT | Actualiza job |
| `GET` | `/api/v1/departments` | JWT | Lista departments |
| `POST` | `/api/v1/departments` | JWT | Crea department (manager opcional) |
| `POST` | `/api/v1/departments/{id}/contacts` | JWT | Crea contacto de department con vigencia |
| `POST` | `/api/v1/departments/{id}/addresses` | JWT | Crea dirección de department con vigencia |
| `GET` | `/api/v1/employees` | JWT | Lista employees |
| `POST` | `/api/v1/employees` | JWT | Crea employee |

Los endpoints protegidos esperan:

```http
Authorization: Bearer <access_token>
```

## Multilingüismo (i18n)

Los textos localizados se resuelven en la capa de aplicación con el header
`Accept-Language`:

1. Se toma el primer locale soportado del header (`es`, `en`, `pt-BR`).
2. Si no hay coincidencia o el header falta, se usa `es`.
3. Para cada ubicación se busca la traducción del locale solicitado.
4. Si no existe, se usa la traducción del locale marcado
   `global.locales.is_default = true`.
5. Si tampoco existe, se devuelve `global.locations.default_name`.

Ejemplo:

```http
GET /api/v1/locations?country=VE
Accept-Language: en-US,en;q=0.9
```

La respuesta incluye `name` (nombre resuelto), `default_name` (valor base) y
`locale` (locale efectivo usado). El middleware `LocaleMiddleware` inyecta el
locale en el `context.Context`; el caso de uso
`GetTranslatedLocation` aplica la cadena de fallback.

Las direcciones de Parties conservan su historial en
`tenants.address_validities`; `is_primary` ya no vive en `tenants.addresses`.
Los contactos y direcciones de Departments usan el mismo patrón mediante
`department_contact_mechanism_validities` y
`department_address_validities`.

## OAuth 2.0 M2M (Client Credentials)

1. Un administrador autenticado crea la app:
   `POST /api/v1/client-applications` con `name` y `scopes`.
   La respuesta incluye `client_secret` **una sola vez**; solo se guarda su
   hash bcrypt en `global.client_applications`.
2. La app intercambia credenciales:
   `POST /api/v1/oauth/token` con `client_id` y `client_secret`.
3. El servidor emite un JWT corto (`client_credentials`) con `tenant_id`,
   `client_id` y `scopes`.
4. `TenantMiddleware` acepta ese token, inyecta el tenant y exige el scope de
   la ruta (`parties:read`, `parties:write`, `identity:write`, `org:read`,
   `org:write`, `documents:write`, `admin:write`). Una ruta sin scope asignado
   se rechaza con `403 forbidden`.

### Apps oficiales y apps de plataforma

- `client_type`: `FIRST_PARTY` (apps oficiales) o `THIRD_PARTY`.
- `application_scope`: `TENANT_SPECIFIC` (requiere `tenant_id`) o `PLATFORM`
  (sin tenant y solo para `FIRST_PARTY`).
- Las apps de plataforma usan los scopes `platform:admin` y `reports:global`.
- El endpoint `POST /api/v1/client-applications/first-party` se protege con la
  cabecera `X-Platform-Admin-Key`, que debe coincidir con `PLATFORM_ADMIN_KEY`.
- Un token de una app `FIRST_PARTY`/`PLATFORM` no lleva `tenant_id`; solo puede
  acceder a rutas de plataforma mapeadas a sus scopes.

## CORS

El middleware permite los orígenes configurados y responde a preflight `OPTIONS` con `204`. Permite `GET, POST, PUT, DELETE, PATCH, OPTIONS` y las cabeceras `Authorization`, `Content-Type`, `Accept`, `Origin`, `X-Requested-With`.

Para Next.js local:

```bash
export CORS_ALLOWED_ORIGINS='http://localhost:3000'
```

## Tests

```bash
make test
make vet
```

Incluyen pruebas de handlers con `httptest`, CORS/preflight, health, Swagger, autenticación, onboarding con rollback y contexto RLS diferido.

## Seguridad

- Contraseñas y códigos de respaldo con bcrypt.
- Refresh tokens y reset tokens almacenados solo como hash.
- Cookies de refresh `HttpOnly`, `Secure`, `SameSite=Strict`.
- Rate limiting estricto en login, refresh, 2FA y onboarding.
- Auditoría con redacción de secretos.
- El rate limiter es en memoria; para múltiples instancias usa Redis.
