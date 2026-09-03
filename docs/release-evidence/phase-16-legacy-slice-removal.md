# Phase 16 legacy-slice removal evidence

Date: 2026-09-01 UTC

## Decision

LedgerSync now has one canonical product implementation: the root Go services and commands, the `web/` Next.js BFF/operator console, PostgreSQL/Redis adapters, migrations, supported deployment files, and their tests. The archived learning-prototype slice was removed atomically after the existing cleanup audit’s 98% confidence was revalidated against the current tree.

## Removed slice

- `backend/`: four disconnected prototype services and one ungenerated proto definition.
- `dashboard/`: the disconnected Next.js 14 demo UI.
- `simulation/`: the disconnected Python simulation service.
- `setup/`: the retired bootstrap SQL.
- `docker-compose.legacy-demo.yml`: the only runtime relationship between those paths; it referenced a missing Redis configuration.
- `tests/consistency_test.go`: a tagged placeholder that imported the absent generated proto package.
- `tests/dashboard_test.py`: three unconditional placeholder assertions.

No behavior was migrated from the old dashboard. The current `web/` surface already owns the supported product behavior and is qualified independently.

## Dependency and boundary changes

`go mod tidy` removed the legacy-only `github.com/go-redis/redis/v8` module and removed `github.com/stretchr/testify` and `google.golang.org/grpc` as direct requirements. Testify remains reachable through an OpenTelemetry dependency's tests, and gRPC remains an indirect OpenTelemetry exporter dependency; neither is represented as current first-party application code. The supported product continues to use the current Redis v9 client where required.

The production-boundary contract now fails when any removed path reappears. README and threat-model language now identify one canonical topology. Historical architecture and cleanup reviews were deliberately preserved without rewriting their original findings.

## Recovery

The deletion is recoverable with a Git revert of the dedicated Phase 16 commit. Restoring individual files or only the legacy Compose file is unsupported because the predecessor slice was incomplete and internally coupled.

## Qualification record

The commit is accepted only after:

- repository reference inventory and dependency convergence;
- `go test ./...`, `go vet ./...`, and contract/integration/system suites;
- supported and restore-drill Compose configuration validation when Docker is available;
- frontend lint, 175 unit/security tests, production build, 206 functional/visual browser checks, and performance budgets;
- generated OpenAPI artifact convergence, secret/dependency scanning available in the local environment, and whitespace review.

Environment-dependent Docker, container, real-stack, manual accessibility, and external security gates remain Phase 17 release evidence; absence of a local Docker daemon is never converted into a passing result.
