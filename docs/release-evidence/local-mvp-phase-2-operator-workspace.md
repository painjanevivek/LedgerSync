# Local MVP Phase 2 — functional operator workspace evidence

**Result:** `PASSED`

**Executed:** 2026-08-24 on the local Windows/Docker Desktop workstation

**Evidence binding:** the Git commit containing this document

## What was proved

Every supported local operator route reads and writes through the same-origin browser/BFF path backed by the real PostgreSQL, Redis, API, and worker containers. The transfer workspace retains an exact transfer intent and idempotency key across an unknown outcome, refuses to reuse that key for a changed payload, and never downgrades an already posted result when a follow-up read fails.

## Route and control inventory

| Route | Purpose exercised | Result |
|---|---|---|
| `/` | Overview evidence, refresh, account and transfer links, reconciliation link, pagination | Passed |
| `/accounts` | Authorized directory, search/filter controls, stable paging, account links | Passed |
| `/accounts/{id}` | Independent balance and immutable history reads, refresh, paging | Passed |
| `/transfers` | Prepare/review/confirm, search/status filter, refresh, paging, detail links | Passed |
| `/transfers/{id}` | Financial status, delivery status, journal, exact facts, two postings | Passed |
| `/reconciliation` | Latest stop-ship result, refresh, history, paging | Passed |
| `/reconciliation/{id}` | Immutable run scope, watermark, counts, mismatch evidence | Passed |

`/admin` remains deny-by-default with HTTP 404 and is not linked. In local demo mode, `/sign-in` redirects to the already established operator workspace instead of presenting a non-functional authentication surface.

## Retry-safety and final-outcome behavior

- Session storage records the schema version, idempotency key, source account, destination account, currency, and canonical integer `amount_minor` together.
- Reloading an unknown outcome restores the exact review and exposes only **Retry same transfer**. Editing is locked until the exact outcome is confirmed.
- A changed amount, destination, currency, corrupt record, non-canonical amount, or same-account record cannot reuse the retained key.
- Definitive rejection clears the key so a corrected intent receives a new key.
- A posted response becomes the final UI state before optional detail/history refreshes. A failed follow-up read cannot turn posted money into an unknown outcome.
- The final confirmation displays transfer ID, journal transaction evidence, exact amount, posted UTC, source, destination, and the committed source balance/version. A credit-only destination balance is deliberately not disclosed.

## Reproducible checks

| Check | Result |
|---|---|
| Web lint | Passed |
| Web unit suite | 22 passed |
| Next.js production build and TypeScript validation | Passed |
| Full Chromium E2E suite | 48 passed |
| Transfer-specific E2E suite at 390 x 844 | 7 passed; no horizontal overflow |
| Windows Chromium responsive visual baseline | Passed after human inspection |
| Linux Chromium responsive visual baseline in the official Playwright container | Passed |
| Real-stack route reconnaissance | All seven supported routes returned HTTP 200 with the expected heading and real controls |
| Real browser transfer | INR 1.00 posted; confirmation showed journal, UTC, and two balance versions |
| Immutable transfer detail | Financial status posted; delivery state separate; exactly one debit and one credit posting |
| Browser console on the real successful journey | 0 errors |
| Real reconciliation read after transfer | Passed with 0 mismatches |

The first live browser probe intentionally used INR 0.01 and was rejected by the configured tenant minimum. That definitive 422 result correctly left no unknown intent in the operator workflow. The passing journey then used an allowed INR 1.00 exact amount.

## Progressive disclosure and failure truthfulness

Directory, current balance, immutable history, transfer detail, and reconciliation evidence load independently. A failed current-balance read does not display stale data as current; a failed history read does not imply an empty ledger; and financial posting remains distinct from downstream delivery. On narrow screens, the authoritative unknown-outcome panel and same-key retry action remain in document flow without sticky overlap.

## Boundary

All browser requests remained on IPv4 loopback. PostgreSQL, Redis, API, and worker endpoints stayed container-private. This evidence proves the one-workstation demo workspace only; it does not claim managed identity, cloud deployment, custody, bank rails, or production-pilot readiness.
