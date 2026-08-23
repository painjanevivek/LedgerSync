# Design-partner controlled pilot record

Create one copy per partner in the approved evidence system. This checked-in
file is a template only; it is not partner approval and must contain no secrets,
tokens, raw financial exports, or unnecessary personal data.

## Identity and boundary

| Field | Required entry |
|---|---|
| Partner/use case | Pending |
| Partner integration owner and LedgerSync owner | Pending |
| Jurisdiction/legal/custody approval references | Pending |
| One currency and precision | Pending |
| Explicit exclusions accepted | bank rails, cards, FX, custody, external settlement, public self-service |
| Pilot start/end and observation period | Pending |
| Support hours, contacts, status route | Pending |

## Technical gate references

| Gate | Evidence | Status |
|---|---|---|
| Phase 6 physical devices | Pending | BLOCKED |
| Phase 6 finance semantics | Pending | BLOCKED |
| Phase 6 roles/limits | Pending | BLOCKED |
| Five-minute production-like capacity/headroom | Pending | BLOCKED |
| `pilot-preflight --require-restore` | Pending | BLOCKED |
| No open critical/high security findings | Pending | BLOCKED |
| OpenAPI/integration retry review | Pending | BLOCKED |

No provisioning apply or traffic enablement occurs while any row is blocked.

## Provisioning record

| Field | Entry |
|---|---|
| Reviewed config reference | Pending |
| Validation fingerprint | Pending |
| Independent reviewer | Pending |
| Apply correlation ID and actor | Pending |
| Tenant/account/policy verification evidence | Pending |
| Credential references/expiry (never secret values) | Pending |
| Pre-traffic rollback owner | Pending |

The config decoder rejects unknown and trailing JSON so a reviewer cannot rely
on a field the provisioning workflow silently ignored. Rollback is available
only before any transfer; after movement begins, access can be disabled but
financial/evidence history is preserved.

## Traffic ramp

| Stage | Maximum approved scope | Observation window | Evidence owner | Decision |
|---|---|---|---|---|
| Sandbox | Non-production accounts; no real financial reliance | Pending | Partner engineering | Pending |
| Internal rehearsal | Exact partner journey in isolated production-like tenant | Pending | LedgerSync operations | Pending |
| Partner read only | Approved account subset, no `transfers:write` | Pending | Product/security | Pending |
| Limited writes | Approved accounts, per-transfer and 24h limits | Pending | Risk/finance/operations | Pending |
| Stable first partner | No unresolved stop condition for approved window | Pending | Pilot owner | Pending |

Second or third partner onboarding is prohibited until the final row is signed.

## Daily control record

| UTC date/window | Transfer outcomes/retries | Reconciliation/run ID | RYEW/outbox/cache | Auth denials | Latency/errors/backup age | Support/incidents | Decision/owner |
|---|---|---|---|---|---|---|---|
| Pending | Pending | Pending | Pending | Pending | Pending | Pending | Pending |

## Weekly product review

Record integration effort, time to complete a safe first transfer, representative
investigation time, support volume/root causes, API/documentation gaps, operator
trust feedback, and any requested scope expansion. Scope expansion remains out
of pilot until a separate PRD/risk program is approved.

## Immediate pause record

| Trigger | Detected UTC | Movement disabled UTC | Incident/evidence | Authority | Reopen evidence | Status |
|---|---|---|---|---|---|---|
| mismatch, duplicate, authorization breach, unsafe unknown result, stale-current balance, restore/backup failure, unowned critical alert | — | — | — | — | — | Not triggered |

## Phase decision

| Decision | Required signers | Evidence reference | UTC date |
|---|---|---|---|
| Continue / pause / remediate / stop / expand to next partner | Product, finance, security, operations, partner owner | Pending | Pending |
