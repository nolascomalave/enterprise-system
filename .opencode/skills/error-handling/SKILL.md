---
name: error-handling
description: >-
  Define el patrón Hybrid Unified App Error para manejar errores en todas las capas
  (dominio, casos de uso, infraestructura y HTTP). Úsalo siempre que crees o modifiques
  código que pueda producir errores.
---

# Error Handling: patrón Hybrid Unified App Error

> **Regla obligatoria:** `.opencode/rules/error-handling-guardrails.md`.
> Este documento explica el patrón; la regla manda. Si hay conflicto, gana la
> regla.

## 1. Idea general

El patrón es **híbrido** porque convive con los errores centinela del dominio:

- Los **errores centinela** (`domain.ErrPartyNotFound`, `auth.ErrInvalidToken`,
  ...) siguen existiendo y se comparan con `errors.Is`.
- Los errores que cruzan hacia HTTP se clasifican con `*apperror.AppError`,
  que aporta tipo, código estable, mensaje seguro y causa técnica.
- El **middleware centralizado** es el único punto que decide el status HTTP.

Ningún handler formatea mensajes de error ni escribe códigos HTTP a mano para
errores de negocio.

## 2. Tipo `AppError`

Paquete: `internal/common/apperror`.

```go
type Type string

type AppError struct {
    Type    Type   // categoría que decide el status HTTP
    Code    string // código estable para el cliente (ej. "invalid_party")
    Message string // mensaje seguro para el cliente
    Err     error  // causa técnica; solo para logs, nunca para el cliente
}
```

Métodos:

- `Error() string`: combina `Message` y `Err`; se usa en logs.
- `Unwrap() error`: devuelve `Err`, por lo que `errors.Is/As` siguen
  funcionando a través del `AppError`.

## 3. Tipos

| Constante | Uso | Status HTTP |
|---|---|---|
| `apperror.TypeNotFound` | El recurso no existe o no es visible para el tenant. | 404 |
| `apperror.TypeValidation` | Input inválido, regla de negocio incumplida. | 400 |
| `apperror.TypeUnauthorized` | Credenciales/token/sesión inválidos. | 401 |
| `apperror.TypeInternal` | Error técnico inesperado. | 500 |

`TypeInternal` **nunca** expone su `Message` real ni su `Err`: el middleware
responde siempre `internal server error` y registra el detalle en logs.

## 4. Constructores

```go
func NewNotFound(code, message string, err error) *AppError
func NewValidation(code, message string, err error) *AppError
func NewUnauthorized(code, message string, err error) *AppError
func NewInternal(err error) *AppError
func New(typ Type, code, message string, err error) *AppError
func From(err error) *AppError // preserva un AppError; cualquier otro → NewInternal
```

`NewInternal` fija `Code = "internal_error"` y
`Message = "internal server error"`. La causa (`err`) queda solo para logs.

## 5. Reglas por capa

### 5.1 Dominio

- Define **errores centinela** con `errors.New`, sin dependencias de HTTP ni
  de infraestructura.
- No conoce `apperror`.

```go
var ErrInvalidParty = errors.New("invalid party")
```

### 5.2 Casos de uso

- Envuelven la causa con `%w` para conservar la cadena:
  `fmt.Errorf("create party: %w", err)`.
- Cuando el caso de uso conoce la semántica del error, lo clasifica con un
  constructor de `apperror`:
  - recurso ausente → `NewNotFound`
  - input/regla de negocio → `NewValidation`
  - credenciales/sesión → `NewUnauthorized`
  - error técnico que no sabe clasificar → `NewInternal`
- Nunca devuelven `err.Error()` formateado para el cliente.

```go
detail, err := uc.parties.GetPartyDetail(ctx, tenantID, partyID)
if err != nil {
    if errors.Is(err, domain.ErrPartyNotFound) {
        return nil, apperror.NewNotFound(
            "party_not_found", "party not found", err,
        )
    }
    return nil, fmt.Errorf("get party: %w", err)
}
```

Nota: como `AppError` implementa `Unwrap`, envolver un `AppError` con `%w`
conserva su tipo y `errors.As` lo encuentra en el middleware.

### 5.3 Infraestructura (repositorios)

- Traducen errores técnicos (Ent, SQL, red) a `AppError`.
- Traducen los `not found` del ORM al centinela de dominio cuando el caso de
  uso lo espera; si no existe centinela, pueden devolver `NewNotFound`.
- Un `UniqueViolation`/constraint se traduce a la regla de negocio
  correspondiente (`domain.ErrTenantSlugTaken`, ...) o a `NewValidation`.
- Los errores inesperados se envuelven con `NewInternal` y conservan la causa.

```go
if _, err := client.Party.Create()...Save(ctx); err != nil {
    return apperror.NewInternal(fmt.Errorf("repository: create party: %w", err))
}
```

### 5.4 HTTP (handlers)

- Los handlers **solo** llaman a `c.Error(err)` y retornan.
- Los errores de binding/parseo se convierten en `NewValidation`; la ausencia
  de sesión/token en `NewUnauthorized`.
- No escriben `c.JSON` de error, no calculan status, no exponen `err.Error()`.

```go
if err := c.ShouldBindJSON(&request); err != nil {
    c.Error(apperror.NewValidation("bad_request", "bad request", err))
    return
}

result, err := h.uc.Create(c.Request.Context(), input)
if err != nil {
    c.Error(err) // el middleware decide el status
    return
}
```

### 5.5 Middleware centralizado

`ErrorMiddleware` es el único lugar que mapea errores:

1. `errors.As(err, &appErr)` → status según `appErr.Type`.
2. Errores centinela legacy (dominio/auth) → tabla de compatibilidad.
3. Cualquier otro error → log con detalle completo y respuesta
   `500 {"error":"internal server error","code":"internal_error"}`.

Los errores `TypeInternal` también se registran en logs con su cadena
completa. El cliente nunca ve nombres de tablas, SQL, hosts ni stack traces.

## 6. Formato de respuesta

```json
{
  "error": "party not found",
  "code": "party_not_found"
}
```

- `error`: mensaje seguro, legible.
- `code`: identificador estable para clientes/monitoreo.
- El status HTTP se deriva del `Type`.

## 7. Ejemplos completos

### Repositorio

```go
func (r *PartyRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*domain.Party, error) {
    row, err := r.client.Party.Get(ctx, id)
    if err != nil {
        if ent.IsNotFound(err) {
            return nil, domain.ErrPartyNotFound
        }
        return nil, apperror.NewInternal(fmt.Errorf("repository: get party: %w", err))
    }
    return toDomainParty(row), nil
}
```

### Caso de uso

```go
func (uc *GetParty) Execute(ctx context.Context, tenantID, id uuid.UUID) (*domain.Party, error) {
    party, err := uc.repo.Get(ctx, tenantID, id)
    if err != nil {
        if errors.Is(err, domain.ErrPartyNotFound) {
            return nil, apperror.NewNotFound("party_not_found", "party not found", err)
        }
        return nil, fmt.Errorf("get party: %w", err)
    }
    return party, nil
}
```

### Handler

```go
func (h *Handler) Get(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.Error(apperror.NewValidation("invalid_party_id", "invalid party id", err))
        return
    }
    party, err := h.uc.Execute(c.Request.Context(), tenantID, id)
    if err != nil {
        c.Error(err)
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": party})
}
```

## 8. Antipatrones prohibidos

- ❌ `c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})`.
- ❌ Devolver `err.Error()` de un error técnico al cliente.
- ❌ Hacer `switch err.(type)` o mapear status dentro de un handler.
- ❌ Crear `AppError` en el dominio (el dominio no conoce HTTP).
- ❌ Perder la causa: `apperror.NewInternal(nil)` o `fmt.Errorf("...")` sin `%w`.
- ❌ Loguear el mensaje seguro y descartar `Err`: el log debe incluir la
  cadena completa.

## 9. Checklist

- [ ] El error nuevo usa un centinela de dominio o un `*AppError`.
- [ ] La causa se conserva con `%w` o en el campo `Err`.
- [ ] El mensaje de `AppError.Message` es seguro para el cliente.
- [ ] El handler solo hace `c.Error(err)`.
- [ ] El status HTTP solo se decide en `ErrorMiddleware`.
- [ ] Los errores internos se registran en logs con detalle completo.
