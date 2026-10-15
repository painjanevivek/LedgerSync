# ADR-001: Preserve the modular financial application

Date: 14 October 2026. Status: accepted for R00/R01 implementation design.

## Decision

Keep the Go API, worker, optional scheduler, Next.js BFF, PostgreSQL, and Redis. Refactor responsibilities in small packets. Do not rename the Go module or extract financial microservices in this programme without a new decision.

Domain packages own exact values and invariants. Application packages own use cases and their small repository interfaces. Platform adapters own database and transport details. HTTP handlers validate boundary input and map typed results. Bootstrap constructs dependencies.

The financial command owns its database transaction. A handler must not independently post a journal and then update a balance. PostgreSQL remains the financial authority. Redis is disposable and cannot certify a balance.

## Enforcement and current exceptions

`tests/architecture/boundaries_test.go` parses actual Go imports. It blocks domain imports into application, platform, transport, bootstrap, or commands. It blocks application imports of concrete database/cache drivers and platform/transport/bootstrap/command packages.

Two existing responsibilities remain explicit exceptions, not hidden exemptions:

- `internal/domain/identifier` implements `database/sql/driver` value conversion. No database connection or query belongs there. R04 will decide whether moving this compatibility contract offers sufficient benefit.
- Existing webhook dispatchers in application packages use `net/http`. Their adapter extraction belongs to R04. The new check does not falsely claim they are already pure application modules.

The guard covers production Go imports, including files behind build tags. It excludes test adapters. Go compilation checks import cycles. Frontend boundary changes and a full application coupling audit remain R04 work.

## Alternatives and consequences

A service rewrite would add network and consistency duties before the current database boundary is complete. A cosmetic folder move would not fix coupling. The chosen approach preserves behaviour and permits one responsibility to move at a time.

Keep mechanical moves separate from semantic changes. Preserve existing URLs and script wrappers until replacements pass. Add an interface only at a real dependency boundary.

Verification: `go test ./tests/architecture`. Negative policy cases prove important forbidden imports are rejected. R13 qualifies the combined candidate.

## R04 boundary update — 15 October 2026

Webhook HTTP adapters now live in platform packages. Application packages own transport-neutral ports and use cases. The import guard now rejects `net/http` in application production code. Existing dispatcher tests moved with their implementations. Signing-key cache, HTTP dispatch and public-address transport have separate files.

Continuous worker setup now uses the shared bootstrap runner with explicit node identity and progress options. Process lifetime and health remain in the command. Correction request/command state lives in a hook while the view retains its render contract. See [module map](../../portfolio/module-map.md).

Retain the named identifier `database/sql/driver` conversion exception. It owns no connection or query. Moving the value conversion would add caller changes without removing adapter authority. No other domain SQL exception is added.
