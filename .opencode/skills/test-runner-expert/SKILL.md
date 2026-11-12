# Skill: Go Test Runner Expert
Description: Automatiza la ejecución y monitoreo de las pruebas del sistema en la terminal.

## Execution Rules
- Cada vez que se genere o modifique un archivo de código o de prueba, se debe invocar el comando de testing.
- Comando principal: `go test ./... -v` (o `go test ./internal/...` según la estructura).
- Si el resultado de la terminal arroja un error (`FAIL`), el agente debe detenerse, leer el stack trace y corregir el código inmediatamente antes de continuar.