# Tasks: Provider-led outbound payouts

## Phase 1: Foundations

- [ ] T001 Add payout schema, immutable evidence protections, and role grants in `migrations/000027_payouts.up.sql`
- [ ] T002 [P] Write payout domain-state and exact-fee tests in `internal/domain/payout/payout_test.go`
- [ ] T003 [P] Write reservation and settlement integration scenarios in `tests/integration/payouts_test.go`
- [ ] T004 Define provider-neutral adapter and deterministic fake sandbox in `internal/application/payouts/provider.go`

## Phase 2: Request and approval

- [ ] T005 [US1] Implement transaction-safe payout request/reservation/idempotency in `internal/platform/db/payout_repository.go`
- [ ] T006 [US1] Implement payout request service and API contract in `internal/application/payouts/service.go` and `contracts/openapi.yaml`
- [ ] T007 [US2] Implement dual-control approval, expiry, audit, and durable provider work in `internal/application/payouts/approval.go`
- [ ] T008 [US2] Add authorized payout routes and BFF forwarding in `internal/transport/http/handlers/payouts.go` and `web/src/app/api/payouts/`

## Phase 3: Provider result and reconciliation

- [ ] T009 [US3] Implement durable provider dispatch and callback verification in `internal/application/payouts/worker.go`
- [ ] T010 [US3] Implement settlement/fee postings and exactly-once reservation release in `internal/platform/db/payout_repository.go`
- [ ] T011 [US4] Implement settlement reconciliation and mismatch evidence in `internal/application/payouts/reconciliation.go`

## Phase 4: Operator and validation

- [ ] T012 [US4] Add payout investigation, approval, and safe-next-action UI in `web/src/features/payouts/`
- [ ] T013 Add payout contract, browser, accessibility, fault, and performance coverage in `tests/` and `web/tests/`
- [ ] T014 Update release evidence and master status in `docs/release-evidence/` and `docs/plans/ledgersync-master-progress.md`

T001–T004 precede all stories. T005–T006 precede T007–T010. T011 depends on
T010. T012 depends on read contracts from T006 and T011.
