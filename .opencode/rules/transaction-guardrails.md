# Transaction Guardrails (OBLIGATORIO)

Estas reglas son de obligado cumplimiento para todo el backend.

## Prohibiciones

- **PROHIBIDO** llamar a `client.Tx(ctx)` fuera de
  `internal/infrastructure/transaction`. La única forma de abrir una
  transacción es `TransactionManager.WithinTransaction`.
- **PROHIBIDO** que un repositorio haga `Begin`/`Commit`/`Rollback` por su
  cuenta. Los repositorios solo usan el cliente ambiente.
- **PROHIBIDO** ejecutar `SAVEPOINT`, `ROLLBACK TO SAVEPOINT` o
  `RELEASE SAVEPOINT` directamente. Usa `TransactionManager.WithinSavepoint`.
- **PROHIBIDO** anidar `WithinTransaction` esperando una segunda transacción:
  si ya existe una transacción ambiente, el manager se une a ella.
- **PROHIBIDO** concatenar input de usuario en nombres de savepoint.
- **PROHIBIDO** cambiar de tenant dentro de un `WithinSavepoint` o de una
  transacción ya abierta. El binding (`BindTenant`) debe hacerse antes de
  abrir la transacción o antes del `SAVEPOINT`. El driver solo permite la
  transición inicial `uuid.Nil → tenant` (onboarding) y cambios de
  usuario/sesión dentro del mismo tenant.

## Obligaciones

- Todo caso de uso de escritura debe ejecutar sus escrituras dentro de:

  ```go
  err := txManager.WithinTransaction(ctx, func(ctx context.Context) error {
      // escrituras
      return nil
  })
  ```

- **Las lecturas sobre tablas con RLS también deben ejecutarse dentro de una
  transacción.** `SET LOCAL` es de ámbito transaccional: fuera de una
  transacción no se publica `app.current_tenant_id` y la política RLS
  devuelve cero filas (o filtra mal). Usa `readWithinTransaction` en la capa
  de caso de uso.
- Los repositorios obtienen el cliente con
  `transaction.ClientFromContext(ctx, r.client)`: devuelve `tx.Client()` si
  hay transacción ambiente y el cliente raíz si no.
- Los pasos parciales que pueden fallar sin abortar la transacción padre usan
  `WithinSavepoint`. Un error en el callback ejecuta `ROLLBACK TO SAVEPOINT`
  y la transacción padre sigue viva.
- Los savepoints se nombran `sp_<n>` generados por el manager.
- La transacción ambiente se publica con `ent.NewTxContext`, de modo que los
  hooks de Ent observan la misma transacción (`ent.TxFromContext`).

## Manejo de errores

- `WithinTransaction`: `Rollback` si el callback falla; `Commit` si termina
  bien. Un pánico también ejecuta `Rollback` antes de propagarse.
- `WithinSavepoint`: en error o pánico hace `ROLLBACK TO SAVEPOINT` +
  `RELEASE` del savepoint interno; **nunca** hace `Rollback` de la
  transacción padre.
- Los rollbacks usan `context.WithoutCancel` para ejecutarse aunque el
  contexto esté cancelado.

## Tests obligatorios

- Debe existir y mantenerse el test que demuestra que un fallo dentro de
  `WithinSavepoint` produce el orden `SAVEPOINT` → `ROLLBACK TO SAVEPOINT` →
  `RELEASE`, y que el `Commit` de la transacción padre **sí** se ejecuta.
- Todo caso de uso de escritura y toda lectura con RLS debe tener un test que
  verifique que usa el manager (no abre transacciones por su cuenta).
- Debe existir el test estático `TestNoDirectTransactionsOutsideManager`, que
  falla si aparece `client.Tx(` fuera de los adaptadores permitidos.
- El driver de tenancy debe tener tests de:
  - transición inicial `Nil → tenant` permitida (onboarding),
  - cambio de tenant dentro de la transacción rechazado,
  - query sin `SessionContext` tras el binding rechazada (fail-closed).
