# ADR 0001: Bootstrap and Lifecycle Refactor

- Status: Accepted
- Date: 2026-02-20

## Context

The server bootstrap logic, dependency wiring, route registration, background job scheduling, and shutdown handling were all concentrated in `main.go`. This created a few problems:

- High coupling between entrypoint code and concrete services/jobs.
- Difficult local testing for lifecycle logic because setup and runtime responsibilities were mixed.
- Weak architectural boundaries for operational concerns (startup validation, graceful shutdown, panic isolation).

The project constraint is to improve architecture and code quality without introducing new external infrastructure (for example Redis, Prometheus, or additional managed services).

## Decision

We refactor runtime composition into an application package and tighten boundaries with small interfaces.

### 1) Extract bootstrap and wiring from `main.go`

- Introduce `internal/app` with `Run(cfg *config.Config) error` as the runtime entrypoint.
- Keep `main.go` minimal: load config, call `app.Run`, handle fatal exit.
- Move middleware/route registration and lifecycle orchestration into internal app functions.

### 2) Introduce small interfaces at boundary points

- In scheduler runtime (`internal/app`):
  - `jobRunner` for jobs that only execute (`Run()`).
  - `lifecycleJob` for jobs that self-schedule (`Start()`, `Stop()`).
- In admin handler (`handlers/admin_handler.go`):
  - `IPOAdminService` for IPO creation use case.
  - `GMPJobRunner` for manual trigger.
  - `GMPHistoryJobRunner` for trigger + status/metrics reads.
- Store `*sql.DB` directly in `AdminHandler` for GMP query endpoint instead of reaching through a concrete IPO service.

### 3) Keep operational hardening in process

- Preserve graceful shutdown flow and signal handling.
- Preserve panic isolation for scheduled jobs.
- Preserve DB-aware health/readiness checks.

## Alternatives Considered

### A) Keep all logic in `main.go`

- Pros: no refactor effort.
- Cons: maintainability and testability remain poor; boundaries stay implicit.

### B) Full dependency injection framework

- Pros: stronger composition model and mocking ergonomics.
- Cons: added complexity and additional tooling not justified for current size.

### C) Microservice split for jobs/API

- Pros: runtime isolation and independent scaling.
- Cons: operational overhead and out of scope for current constraints.

## Consequences

### Positive

- Clearer architecture: entrypoint vs runtime composition vs HTTP handlers.
- Lower coupling due to narrow interfaces at integration points.
- Better testability potential for runtime pieces and handlers.
- Safer lifecycle behavior retained and easier to reason about.

### Negative / Trade-offs

- More files and indirection for newcomers.
- Interfaces are currently local and minimal; they can drift if not reviewed.
- Still one process for API + schedulers, so hard runtime isolation is limited.

## Follow-up Work

- Add focused tests around `adminAuthMiddleware` and scheduler lifecycle (`startBackgroundJobs`).
- Add constructor-level validation for critical dependencies to fail fast.
- Consider separate process for background jobs when operational constraints allow.
