# Skill: TDD Expert (Go & Postgres)
Description: Fuerza el ciclo estricto de desarrollo guiado por pruebas (Red-Green-Refactor) para el backend en Go.

## Rules
- **Prohibido** escribir código de producción sin un caso de prueba previo.
- **Paso RED**: Escribe primero la prueba en un archivo `_test.go` utilizando `testing` o `testify` y simula fallos.
- **Paso GREEN**: Genera el código mínimo indispensable en Go para que el test compile y pase con éxito.
- **Paso REFACTOR**: Limpia el código, optimiza punteros y mejora la legibilidad sin romper el test.