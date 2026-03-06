- LSP diagnostics tool could not run because gopls is not available on PATH in this environment.

- Task 7 blocker: Postgres is unreachable at localhost:5432, causing `go run .` to exit before server binds to :8080.
- Task 7 blocker: Docker daemon is unavailable (`dockerDesktopLinuxEngine` pipe missing), so DB verification via `docker exec ... psql` cannot be executed.
