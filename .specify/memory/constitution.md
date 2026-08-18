# LedgerSync Constitution
<!-- Example: Spec Constitution, TaskFlow Constitution, etc. -->

## Core Principles

### I. Financial Correctness First
<!-- Example: I. Library-First -->
Money is represented exactly, posted ledger history is immutable, and every transfer is idempotent, authorized, balanced, and reconciled before performance work is accepted.
<!-- Example: Every feature starts as a standalone library; Libraries must be self-contained, independently testable, documented; Clear purpose required - no organizational-only libraries -->

### II. PostgreSQL Is Authoritative
<!-- Example: II. CLI Interface -->
PostgreSQL records financial truth. Caches, replicas, and events may improve availability or speed but may never determine money correctness.
<!-- Example: Every library exposes functionality via CLI; Text in/out protocol: stdin/args → stdout, errors → stderr; Support JSON + human-readable formats -->

### III. Test Evidence Is Required
<!-- Example: III. Test-First (NON-NEGOTIABLE) -->
Tests are written before financial behavior and must prove concurrency safety, retry safety, authorization, reconciliation, and read-your-writes behavior under faults.
<!-- Example: TDD mandatory: Tests written → User approved → Tests fail → Then implement; Red-Green-Refactor cycle strictly enforced -->

### IV. Secure by Default
<!-- Example: IV. Integration Testing -->
Every account action is authorized server-side. Secrets, tokens, PII, and raw balances are never committed or logged. Internal dependencies are private by default.
<!-- Example: Focus areas requiring integration tests: New library contract tests, Contract changes, Inter-service communication, Shared schemas -->

### V. Controlled Change and Observability
<!-- Example: V. Observability, VI. Versioning & Breaking Changes, VII. Simplicity -->
Financial schema changes use reviewed migrations. Each request/event is traceable, alerts link to runbooks, and recovery is demonstrated through restore drills.
<!-- Example: Text I/O ensures debuggability; Structured logging required; Or: MAJOR.MINOR.BUILD format; Or: Start simple, YAGNI principles -->

## Delivery Constraints
<!-- Example: Additional Constraints, Security Requirements, Performance Standards, etc. -->

- Production behavior is built in the root modular Go application and worker, not in the legacy demo services.
- A release cannot bypass reconciliation, RYEW fault, authorization, and restore evidence.
- Complexity such as Kafka, Kubernetes, and multi-region writes requires a measured justification.
<!-- Example: Technology stack requirements, compliance standards, deployment policies, etc. -->

## Development Workflow
<!-- Example: Development Workflow, Review Process, Quality Gates, etc. -->

- Use the task plan in `specs/001-secure-transfer-core/tasks.md` in dependency order.
- Keep interfaces documented before cross-component implementation.
- Prefer narrow adapters and explicit domain invariants over generic abstractions.
<!-- Example: Code review requirements, testing gates, deployment approval process, etc. -->

## Governance
<!-- Example: Constitution supersedes all other practices; Amendments require documentation, approval, migration plan -->

This constitution supersedes legacy demo conventions. Changes require an ADR, migration/compatibility assessment where applicable, and updated test evidence.
<!-- Example: All PRs/reviews must verify compliance; Complexity must be justified; Use [GUIDANCE_FILE] for runtime development guidance -->

**Version**: 1.0.0 | **Ratified**: 2026-08-18 | **Last Amended**: 2026-08-18
<!-- Example: Version: 2.1.1 | Ratified: 2025-06-13 | Last Amended: 2025-07-16 -->
