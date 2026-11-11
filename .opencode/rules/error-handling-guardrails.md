# Error Handling Guardrails (OBLIGATORIO)

Estas reglas son de obligado cumplimiento para todo el backend.

## Prohibiciones

- **PROHIBIDO** devolver `err.Error()` o cualquier error técnico directamente
  en una respuesta HTTP.
- **PROHIBIDO** escribir respuestas de error desde un handler
  (`c.JSON(4xx/5xx, ...)`) para errores de negocio. El handler solo llama a
  `c.Error(err)`.
- **PROHIBIDO** mapear errores a códigos HTTP fuera de `ErrorMiddleware`.
- **PROHIBIDO** exponer al cliente nombres de tablas, SQL, hosts, stack traces
  o mensajes de drivers/base de datos.
- **PROHIBIDO** crear `AppError` en la capa de dominio.
- **PROHIBIDO** descartar la causa técnica: todo error envuelto debe conservar
  su cadena con `%w` o en el campo `Err`.

## Obligaciones

- Todo error que llegue a la capa HTTP debe ser un `*apperror.AppError` o ser
  envuelto con `apperror.NewInternal`.
- Los errores centinela del dominio (`domain.Err*`, `auth.Err*`) se conservan
  y se comparan con `errors.Is`; el middleware los traduce como capa de
  compatibilidad.
- El **middleware centralizado (`ErrorMiddleware`) es el único lugar donde se
  mapean errores a códigos HTTP**:
  - `TypeNotFound` → 404
  - `TypeValidation` → 400
  - `TypeUnauthorized` → 401
  - `TypeInternal` → 500
- Los errores técnicos (base de datos, red, serialización) **NUNCA** se
  exponen al cliente: se registran en logs con el detalle completo y se
  responde con un `500` genérico (`internal server error`).
- Los handlers traducen fallos de binding/parseo a
  `apperror.NewValidation(...)` y la falta de sesión/token a
  `apperror.NewUnauthorized(...)`.
- La infraestructura traduce errores técnicos a `AppError`; los casos de uso
  los envuelven con `%w` y añaden clasificación semántica cuando la conocen.

## Contrato de respuesta

```json
{
  "error": "mensaje seguro para el cliente",
  "code": "codigo_estable"
}
```

## Tests obligatorios

- `ErrorMiddleware` debe tener tests que cubran:
  - cada `apperror.Type` → status HTTP correcto;
  - `AppError` envuelto con `%w` sigue siendo detectado;
  - `TypeInternal` responde `internal_error` genérico y **no** filtra el
    mensaje técnico;
  - un error desconocido responde `500` genérico;
  - los centinelas legacy siguen mapeándose.
- Todo repositorio que traduzca errores técnicos debe conservar
  `errors.Is/As` sobre la causa original.
