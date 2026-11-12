---
name: transaction-management
description: >-
  Patrón de transacciones y subtransacciones para el backend Go con Ent.
  Úsalo al implementar casos de uso que agrupan varias operaciones,
  repositorios que deben participar en una transacción existente,
  Unit of Work o lógica aislada con SAVEPOINT.
---

# Transaction Management: transacciones y savepoints en Go + Ent

> **Regla obligatoria:** `.opencode/rules/transaction-guardrails.md`.
> Este documento explica el patrón; la regla manda. Si hay conflicto, gana la
> regla.

## 1. Modelo mental

Hay **dos operaciones distintas**, no una sola con dos modos:

| Operación | Cuándo usarla | Qué hace si ya hay transacción |
|---|---|---|
| `WithinTransaction(ctx, fn)` | Unidad atómica de trabajo (caso de uso) | **Se une** a la transacción ambiente. No abre otra. |
| `WithinSavepoint(ctx, fn)` | Bloque aislado tipo *try/catch* dentro de una unidad de trabajo | Abre `SAVEPOINT`, ejecuta `fn` y hace `RELEASE` o `ROLLBACK TO SAVEPOINT`. |

Regla mental:

- **`WithinTransaction` = transacción o join.** El commit/rollback real solo
  ocurre en la invocación raíz.
- **`WithinSavepoint` = bloque aislado.** Si `fn` falla, se revierte solo ese
  bloque; la transacción padre sigue viva y decide si continúa o aborta.

El puerto de aplicación vive en `internal/usecase/transaction.go`:

```go
type TransactionManager interface {
    WithinTransaction(ctx context.Context, fn func(context.Context) error) error
    WithinSavepoint(ctx context.Context, fn func(context.Context) error) error
}
```

El caso de uso **nunca** conoce `*ent.Client`, `*ent.Tx` ni SQL de savepoints.

## 2. Implementación real

`internal/infrastructure/transaction/manager.go`:

- `WithinTransaction`:
  - si el `ctx` ya lleva transacción (`TxFromContext`), ejecuta `fn(ctx)` y
    devuelve su resultado (join);
  - si no, `client.Tx(ctx)`, publica la transacción en el contexto y hace
    `Commit` o `Rollback`.
  - un pánico también ejecuta `Rollback` antes de propagarse.
- `WithinSavepoint`:
  - exige transacción ambiente; si no la hay devuelve
    `ErrNoActiveTransaction`;
  - genera un nombre `sp_<n>` (nunca input de usuario);
  - ejecuta `SAVEPOINT sp_n`, `fn`, y después `RELEASE SAVEPOINT sp_n`;
  - si `fn` falla o entra en pánico: `ROLLBACK TO SAVEPOINT sp_n` +
    `RELEASE SAVEPOINT sp_n`, y devuelve el error original;
  - **nunca** hace `Rollback` de la transacción padre.

Ent v0.14.6 **no** expone `Tx.Savepoint()`. Por eso:

- `ent/generate.go` habilita `sql/execquery`;
- `ent.Tx.ExecContext` ejecuta el SQL de savepoints;
- `enttenant.sessionTx` reenvía `ExecContext`/`QueryContext` al driver real.

No se debe replicar este SQL en casos de uso ni repositorios.

## 3. Contexto de transacción

La transacción viaja **solo** en el `context.Context`, usando las claves de Ent:

```go
// manager.go
func withTx(ctx context.Context, tx *ent.Tx) context.Context {
    return ent.NewTxContext(ctx, tx)
}

func TxFromContext(ctx context.Context) (*ent.Tx, bool) {
    tx := ent.TxFromContext(ctx)
    return tx, tx != nil
}
```

Reglas:

- El callback siempre usa el `ctx` recibido. Nunca `context.Background()`.
- No guardar `*ent.Tx` en campos de structs ni variables globales.
- No abrir `client.Tx(ctx)` fuera del manager.
- Los hooks de Ent ven la transacción porque se publica con
  `ent.NewTxContext`.

## 4. Repositorios

El repositorio nunca decide abrir transacción. Obtiene el cliente ambiente:

```go
func (r *PartyRepository) execute(
    ctx context.Context,
    fn func(*ent.Client) error,
) error {
    client, err := clientFromContext(ctx, r.client)
    if err != nil {
        return err
    }
    return fn(client)
}

func clientFromContext(ctx context.Context, fallback *ent.Client) (*ent.Client, error) {
    if fallback == nil {
        return nil, errNilClient
    }
    return transaction.ClientFromContext(ctx, fallback), nil
}
```

`transaction.ClientFromContext` devuelve `tx.Client()` si hay transacción
ambiente o el cliente raíz si no. El cliente raíz es solo un *fallback* para
acceso global; **las lecturas con RLS también deben ir en transacción**.

### Lecturas y RLS

`SET LOCAL` es de ámbito transaccional. Fuera de una transacción no se publica
`app.current_tenant_id` y la política RLS devuelve cero filas. Por eso los
casos de uso envuelven también las lecturas:

```go
func readWithinTransaction[T any](
    ctx context.Context,
    tx TransactionManager,
    fn func(context.Context) (T, error),
) (T, error)
```

No crees un helper de lectura que use el cliente raíz para tablas con RLS.

## 5. Casos de uso

### 5.1 Escritura atómica

```go
func (uc *CreatePartyUseCase) Execute(ctx context.Context, in Input) error {
    return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
        party, err := domain.NewParty(in.TenantID, in.DisplayName, uc.now())
        if err != nil {
            return err
        }
        if err := uc.parties.Create(ctx, party); err != nil {
            return err
        }
        return uc.profiles.Create(ctx, party.ID, in.Profile)
    })
}
```

### 5.2 Anidamiento: join, no savepoint

```go
func (uc *Checkout) Execute(ctx context.Context) error {
    return uc.tx.WithinTransaction(ctx, func(ctx context.Context) error {
        if err := uc.orders.Create(ctx); err != nil {
            return err
        }
        // Ya hay transacción: esto se une a ella.
        return uc.audit.Log(ctx)
    })
}
```

No esperes una segunda transacción ni un savepoint aquí. Si el objetivo es
aislar un bloque, usa `WithinSavepoint` explícitamente.

### 5.3 Bloque aislado tipo try/catch

```go
err := uc.tx.WithinSavepoint(ctx, func(ctx context.Context) error {
    return uc.repo.CreateMergeLog(ctx, sourceID, targetID)
})
if err != nil {
    // El savepoint ya revirtió el bloque; el padre sigue vivo.
    log.Printf("merge log skipped: %v", err)
}
// El resto de la transacción padre puede continuar y commitear.
```

Usa `WithinSavepoint` cuando el fallo del bloque **no** debe abortar la unidad
de trabajo completa (por ejemplo, telemetría/auditoría secundaria). No lo uses
para ocultar errores de negocio.

### 5.4 Unit of Work

Un `UnitOfWork` es un caso de uso más: recibe `TransactionManager` y ejecuta el
callback dentro de `WithinTransaction`. Los stores que recibe participan por el
`ctx`; no abren transacción propia. El onboarding del proyecto es el ejemplo:
un único `WithinTransaction` cubre subscription global + grafo tenant y va
publicando el tenant con `BindTenant` antes de la primera escritura tenant.

## 6. Semántica de savepoints

Éxito:

```sql
SAVEPOINT sp_<n>;
-- operaciones del callback
RELEASE SAVEPOINT sp_<n>;
```

- Los cambios quedan dentro de la transacción padre.
- El padre **no** se commitea todavía.

Fallo:

```sql
SAVEPOINT sp_<n>;
-- operaciones del callback
ROLLBACK TO SAVEPOINT sp_<n>;
RELEASE SAVEPOINT sp_<n>;
```

- Se revierten únicamente los cambios posteriores al `SAVEPOINT`.
- La transacción padre sigue activa.
- Se devuelve el error original (con `errors.Join` si también falla el
  rollback del savepoint).
- El `ROLLBACK` usa `context.WithoutCancel` para ejecutarse aunque la petición
  se haya cancelado.

### Tabla de decisión

| Situación | Acción | Resultado |
|---|---|---|
| `WithinTransaction` sin tx ambiente, callback OK | `Commit` | Cambios persistidos |
| `WithinTransaction` sin tx ambiente, callback error/pánico | `Rollback` | Todo revertido |
| `WithinTransaction` con tx ambiente | join | Nada que commitear aquí |
| `WithinSavepoint`, callback OK | `RELEASE` | Cambios del bloque quedan en el padre |
| `WithinSavepoint`, callback error/pánico | `ROLLBACK TO` + `RELEASE` | Solo se revierte el bloque; padre vivo |
| `WithinSavepoint` sin tx ambiente | `ErrNoActiveTransaction` | No se ejecuta el callback |

## 7. Multitenencia y RLS

El driver `internal/infrastructure/enttenant` aplica el contexto de sesión con
`set_config(..., true)` (equivalente a `SET LOCAL`) antes de la primera query
de la transacción.

Reglas innegociables:

1. **El tenant no cambia dentro de una transacción.** El driver solo permite
   `uuid.Nil → tenant` (onboarding) y cambios de usuario/sesión en el mismo
   tenant. Un cambio de tenant devuelve error.
2. **Fail-closed:** si ya se aplicó un tenant y llega una query sin
   `SessionContext`, el driver devuelve error en vez de reutilizar el tenant
   anterior.
3. **Binding antes del savepoint.** El binding de tenant debe ocurrir antes de
   abrir un `WithinSavepoint`; no dentro del bloque.
4. **Lecturas RLS en transacción.** Ver §4.

## 8. Reglas de diseño

1. **Un solo dueño del commit:** solo la invocación raíz de
   `WithinTransaction` hace `Commit`.
2. **Un solo dueño del rollback raíz:** solo la invocación raíz hace
   `Rollback` cuando el callback raíz falla.
3. **Ningún repositorio abre transacciones:** usan `ClientFromContext`.
4. **No capturar `*ent.Tx` en structs:** la transacción vive en el `ctx`.
5. **No usar `context.Background()`** dentro de callbacks o repositorios.
6. **No cambiar de tenant dentro de una transacción.**
7. **No usar savepoints para ocultar errores de negocio.**
8. **Los errores de dominio viajan sin envolver** cuando el handler HTTP los
   necesita para mapear `400/404/409`; envuelve solo con `%w` si añades
   contexto.
9. **La auditoría es transaccional:** un `ROLLBACK TO SAVEPOINT` también
   revierte la auditoría escrita dentro del bloque. Es lo esperado.
10. **`WithinTransaction` anidado = join**, no savepoint.

## 9. Testing

### Manager

Los tests unitarios usan un `dialect.Driver` falso que registra SQL y
commit/rollback (`internal/infrastructure/transaction/manager_test.go`).
Deben cubrir, como mínimo:

- commit en éxito y rollback en error;
- join de una transacción ambiente (no se abre una segunda);
- `WithinSavepoint` sin transacción → `ErrNoActiveTransaction`;
- orden `SAVEPOINT` → `RELEASE` en éxito;
- orden `SAVEPOINT` → `ROLLBACK TO SAVEPOINT` → `RELEASE` en error;
- savepoints anidados: solo se revierte el interno;
- el `Commit` del padre **sí** ocurre tras un savepoint fallido;
- pánico: rollback del savepoint y rollback del padre;
- el contexto de Ent ve la transacción (`ent.TxFromContext`).

### Test estático

`TestNoDirectTransactionsOutsideManager` falla si aparece `client.Tx(` o
`.Tx(ctx)` fuera de `internal/infrastructure/transaction` y del driver de
tenancy.

### Casos de uso

- Verifica que se invoca el manager (contador del fake).
- Verifica que un error de repositorio se propaga.
- Verifica que un savepoint fallido no cierra la transacción padre.
- En integración real, inserta una fila antes del savepoint, provoca un fallo
  dentro y comprueba que la primera fila sigue visible dentro de la
  transacción.

## 10. Antipatrones prohibidos

- ❌ Abrir `client.Tx(ctx)` dentro de un repositorio o caso de uso.
- ❌ Esperar que `WithinTransaction` anidado abra un savepoint.
- ❌ Abrir `WithinSavepoint` sin transacción padre.
- ❌ Hacer `Commit` o `Rollback` del padre desde el bloque aislado.
- ❌ Ignorar el error de `ROLLBACK TO SAVEPOINT`.
- ❌ Guardar `tx` en un campo del repositorio o del caso de uso.
- ❌ Pasar `context.Background()` a un repositorio dentro de la transacción.
- ❌ Usar el cliente raíz para leer tablas con RLS.
- ❌ Cambiar `app.current_tenant_id` después de comenzar la transacción.
- ❌ Hacer visible un `*ent.Tx` en `internal/usecase` o `internal/domain`.

## 11. Checklist rápido

- [ ] El caso de uso recibe `TransactionManager`, no `*ent.Client`.
- [ ] El callback usa el `ctx` que recibe; no crea uno nuevo.
- [ ] El repositorio usa `transaction.ClientFromContext`.
- [ ] Las lecturas con RLS también van en transacción.
- [ ] `WithinTransaction` raíz hace `Commit` solo si el callback termina bien.
- [ ] `WithinTransaction` raíz hace `Rollback` si el callback falla.
- [ ] `WithinTransaction` anidado se une a la transacción ambiente.
- [ ] `WithinSavepoint` se usa solo para bloques aislados.
- [ ] El savepoint fallido hace `ROLLBACK TO SAVEPOINT` + `RELEASE`.
- [ ] Un fallo en el savepoint no llama `Rollback` del padre.
- [ ] Los tests cubren éxito, error raíz, error de savepoint y pánico.
