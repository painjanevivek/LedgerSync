# Local-product Phase 0 — baseline, freeze, and stop-ship evidence

**Result:** `PASSED`

**Baseline date:** 2026-08-24

**Starting commit:** `89f5752`

**Gate:** [LPC-000](../pilot/local-product-completion-gates.md)

This document freezes the starting state for the local-product completion cycle and records closure of the three Phase 0 security stop-ships. It does not modify the [passed local-only MVP evidence](../pilot/local-mvp-gates.md).

**Security remediation commit:** `d674eaf`

**Phase 0 verification time:** `2026-08-24T17:52:53Z`

## Supported local boundary

- one Windows workstation running Docker Desktop;
- web access only at `http://127.0.0.1:3000`;
- API, PostgreSQL, and Redis reachable only on the private Docker network;
- deterministic INR demo data and internal same-currency ledger transfers;
- a server-controlled demo identity, with no browser-selected tenant or subject;
- no external deployment, customer data, managed identity, provider recovery, bank connectivity, FX, custody, or production claim.

The supported Compose configuration explicitly marks the deployment as `development` and sets `LEDGERSYNC_PUBLIC_ORIGIN=http://127.0.0.1:3000`. Those settings mitigate two configuration findings in the supported topology today, but configuration-dependent mitigation is not accepted as closure for a fail-open code path.

## Baseline runtime and preservation evidence

The pre-remediation local preflight recorded:

| Check | Baseline result |
|---|---|
| Normal supported stack | Healthy |
| Schema | `000012` |
| Applied migrations | `12` |
| Outbox | `0` pending, `0` dead |
| Latest reconciliation | `matched`, `0` mismatches |
| Protected backup | `backup-20260824T174418Z-89f5752` — `PASS` |
| Orphan runtime cleanup | `ledgersync-system` containers removed without removing volumes |
| Preserved orphan data | Both `ledgersync-system` named volumes remain preserved |

The preserved orphan volumes are not part of the supported runtime and must not be deleted as an incidental cleanup action. Normal local stop/restart must continue to preserve the supported PostgreSQL and Redis volumes.

## Immutable migration baseline

Migrations `000001` through `000012` are frozen and immutable from starting commit `89f5752`. A future schema change must be additive as `000013` or later. The SHA-256 values below make accidental edits detectable.

| Migration | SHA-256 |
|---|---|
| `000001_financial_schema.up.sql` | `07672cd26365b7b1ded8c26b9268d6e899d07c87ed344bb4800b06c32043acee` |
| `000002_transfer_ledger.up.sql` | `c11d1511daea10eed6d707bce674df5b7b84e0b6cd2b4132e0516c4c27e33cba` |
| `000003_ledger_integrity.up.sql` | `81e2a1043138bef2774889de6308dce7d3e526ae5d64e25f8809e2988fe0b538` |
| `000004_outbox_delivery_leases.up.sql` | `44143c798c606e9db193b33fef792950378ccf9d9c88876d448ba31a891bff29` |
| `000005_operational_evidence.up.sql` | `7e8310911d13e569cd733107fd5b98905a176e141ee5dda5495afbf368a9fa8d` |
| `000006_reconciliation_opening_balances.up.sql` | `ea599bacbe6a0a71cbc1889810ac1aa8eda7ca8821daa2991f03bd67987bf359` |
| `000007_operator_investigation.up.sql` | `ac195eb1752f0380a9d3e89cf534092e239dfe08c706c5d32a5caffdbb5f88aa` |
| `000008_reconciliation_delivery_truth.up.sql` | `9bbdd9ea20798767af11bc313b99c06025588a376557da818e4f8b307801c520` |
| `000009_pilot_security_controls.up.sql` | `2929f0c8667879983f8d5d77b15201434174242b420a83618af4908ce0782a87` |
| `000010_lifecycle_recovery_provisioning.up.sql` | `e8dd87611fb2693f1028e23101b9d4a00f47d52b8d6c43c48c6cee366e1b53b0` |
| `000011_account_directory_scale.up.sql` | `ac5ac9eac3c72c830907d52056af3b1071366c43f7d74696203d656aef2be78e` |
| `000012_transfer_velocity_capacity.up.sql` | `7ece49263be7f9ef4ccb7cab17f39a5cfbd421c22f33a41cb00d495e2cbde5ae` |

## Contract freeze

The supported route and identity contracts are frozen at the starting commit. Corrections must preserve route names and update implementation, contract, tests, and evidence together.

| Contract artifact | Baseline SHA-256 |
|---|---|
| `contracts/openapi.yaml` | `67d4297c40a4a392071dd669263aed592c251817413e3c5cb972d80806f5b47d` |
| `contracts/bff-actor-assertion.md` | `182ab4f8d03f9643a2e43faff7cc1dcb274060a23afeefc922dc1ef7db34da44` |
| `contracts/README.md` | `666f0269906c74f0d14fe13b53282992e094d141c58176088cfc9b91a32f81cd` |

The freeze does not prohibit a security correction. It requires any changed contract to be explicit, backward-conscious, test-covered, and rebound to the completion commit instead of being described as unchanged.

## Frozen financial and product invariants

The completion cycle must preserve all of the following:

1. Money is an ISO currency plus an exact integer minor-unit value; floating point does not cross a financial path.
2. A posted internal transfer commits one durable outcome, one balanced journal, exactly one debit and one credit posting, account versions/projections, audit obligation, idempotency outcome, and outbox records atomically.
3. Matching idempotent retries return the saved result without a second movement; changed intent with the same key conflicts.
4. Posted journals and postings are immutable. Corrections use linked compensating entries rather than updates or deletes.
5. PostgreSQL is financial authority. Redis is disposable, version-aware acceleration and cannot authorize or determine financial truth.
6. Every account, balance, history, transfer, and reconciliation read enforces tenant, subject, scope/role, and object relationship before disclosure, regardless of cache hit or miss.
7. A completed transfer's balance read satisfies its minimum committed version or returns truthful temporary unavailability; an older value is never labelled current.
8. Reconciliation covers the declared authoritative population, persists mismatch evidence, and cannot report an unintended empty or incomplete comparison as matched.
9. Financial posting, outbox publication, downstream delivery, and reconciliation are separate states backed by persisted evidence.
10. The local product moves value only between LedgerSync accounts in INR; no bank rail, card, FX, custody, or external settlement behavior is implied.
11. The browser uses the same-origin BFF and a server-controlled demo identity; it cannot choose a tenant/subject or call PostgreSQL, Redis, or the private API directly.
12. Unknown outcomes retain the original idempotency key, destructive recovery is explicit, and required financial/audit evidence survives stop, restart, backup, and restore.

## Phase 0 security stop-ships

All three items are closed by source-level regression evidence, a production web build, rebuilt supported containers, and a live local security smoke.

| ID | Finding and security invariant | Baseline severity/context | Supported-Compose countercontrol | Required closure evidence | Status |
|---|---|---|---|---|---|
| P0-SEC-001 | A same-tenant cached-balance read can return account data before object-level authorization. Cache hits and primary reads must enforce the same owner/object decision before disclosure. | `HIGH` in the API-specialist review; `MEDIUM` in the exact one-workstation local-boundary baseline because the server-controlled demo identity and seed narrow the immediately reachable actor set | Tenant-bound cache keys and the seeded demo operator reduce exposure but do not replace object authorization | Authorization now precedes cache access; wrapped authorization sentinels retain non-disclosing HTTP mapping; warm/cold non-owner and authorized cache-hit regression tests pass | `CLOSED` |
| P0-SEC-002 | Demo identity configuration can fail open when the deployment marker is missing or unknown. Demo mode must require an explicit development marker. | Stop-ship configuration weakness outside the exact supported Compose invocation | Supported Compose explicitly sets `LEDGERSYNC_DEPLOYMENT_ENV=development` | Missing, blank, unknown, `prod`, `production`, staging, preview, and typoed markers reject demo configuration; explicit development plus loopback origin works | `CLOSED` |
| P0-SEC-003 | Falling back to a Host-derived expected origin can permit DNS-rebinding CSRF when `LEDGERSYNC_PUBLIC_ORIGIN` is omitted. Cookie-authenticated mutation checks must use an explicit trusted origin and fail closed without it. | Stop-ship browser/BFF boundary weakness outside the exact supported Compose invocation | Supported Compose sets `LEDGERSYNC_PUBLIC_ORIGIN=http://127.0.0.1:3000` and publishes the web service on loopback | Host-derived fallback is removed; origin is startup-validated; mismatched/rebound Host returns `421` with `no-store`; supported host/origin and CSRF path passes | `CLOSED` |

## Phase 0 exit evidence

- Focused balance authorization regression tests passed for warmed and cold cache denial, authorized cache hits, and stable non-disclosing HTTP envelopes.
- `go test -count=1 ./...` and focused `go vet` passed for the remediation commit.
- Web lint passed; all `27/27` web unit/security tests passed; the Next.js production build passed.
- A contract test now enforces the frozen SHA-256 values of migrations `000001`–`000012` and scans canonical financial source paths for floating-point money operations.
- Supported API/web images rebuilt successfully from the corrected source.
- `scripts/status-local.ps1` passed after rebuild: schema `000012`, 12 migrations, outbox `0` pending/`0` dead, reconciliation `matched` with `0` mismatches.
- `scripts/test-phase6-security.ps1` passed: generated secrets, seven hardened containers, loopback-only publication, redacted logs, and authenticated reads.
- The protected pre-remediation backup `backup-20260824T174418Z-89f5752` remains the rollback artifact; no schema or normal financial data changed in Phase 0.
- The three specialist findings above were independently retested by separate API and web-security workstreams before integration.

`LPC-000` is passed. `LPC-010` is ready; later gates remain sequence-blocked.
