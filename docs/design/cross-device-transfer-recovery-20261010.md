# Cross-device transfer recovery — 2026-10-10

## Operator journey

1. Review and confirm the exact source, destination, currency, and amount.
2. The browser saves the confirmed intent through the authenticated BFF. PostgreSQL
   retains its immutable original key, reference, and fingerprint. This reserves
   no funds. Posting waits for the durable save acknowledgment.
3. If the command or response is lost, keep the result unconfirmed.
4. Sign in from a clean browser as the same stable user and tenant.
5. Open **Transfers → Recover a saved transfer** and read the current scoped page.
6. Open a known posted or rejected receipt without a financial write.
7. For an unresolved request, select **Review saved request** and check the original
   accounts and exact amount. Cancel returns focus to that request.
8. Select **Retry original request** explicitly. The request has an empty body.
   The server loads the original intent and resolves its original financial key.
   A known outcome is replayed. Otherwise current policy and permissions apply.
9. Inspect the receipt, balanced journal, and account evidence. Check event delivery
   separately. Delivery failure cannot trigger another transfer.

## Implemented controls

| Concern | Control |
|---|---|
| Browser data loss | PostgreSQL intent is primary; session storage is optional. |
| Duplicate execution | Original tenant, actor, operation, key, and fingerprint. |
| Changed intent | Immutable trigger, controlled prepare, unique key/reference, conflict checks. |
| Replacement data | Empty-body recovery; server reconstruction of original command. |
| Access | Session, CSRF/origin, fresh actor assertion, scopes, roles, current account permissions. |
| Discovery | Original-user/tenant page, maximum 25 rows, time/reference cursor. |
| Private identity | List JSON excludes keys and fingerprints. |
| Account state | Discovery works without active-account status or a complete picker. |
| Revoked permission | Request is hidden; a reference grants no access. |
| Known rejection | Insufficient-funds outcome is replayed with no postings. |
| Failed read/offline | No verified-empty claim or enabled stale recovery action. |
| Deployment | Migration 000040; same API and database, no new public port or service. |
| Database role | API SELECT and controlled prepare EXECUTE; no intent-table direct DML. |

API contract **3.4.0** includes the recovery routes and generated client catalogues.
The private retry route ends in `/retry` to coexist with transfer-correction routes.
The BFF reuses the financial response and permitted consistency-session metadata.

## Verification

- 211 frontend unit tests passed.
- 26 focused browser cases passed, including existing journey regressions, review,
  cancel/focus return, read-only access, failed refresh, safe prepare failure,
  narrow reflow, and axe checks.
- Full Go suite passed outside the Windows sandbox. The sandbox had blocked
  existing worker temporary-file renames and local webhook test sockets.
- Three recovery database integration tests passed on a separate isolated database:
  final rejection replay, durable scope/identity, and bounded pagination.
- All 19 diagrams rendered from updated Mermaid sources.
- API, migration, and frontend production images built. Main runtime health passed.
  Main ledger fingerprints before and after migration/restart match: six accounts,
  four transfers, ten postings, and unchanged exact balances and financial rows.
- The local backup restore drill passed at schema 000040. Restored journals and
  posted transfers were valid. Reconciliation found zero mismatches. The main
  project remained unchanged. Measured local restore time was 33.63 seconds.

### Real clean-device journey

The browser test verifies isolated Docker ownership before any financial command.
It creates two zero-balance accounts and funds the source with INR 5.00 through
normal evidence recording, approval, and separate balanced journal posting.

**Lost command:** durable preparation reached the real stack. The posting command
was dropped before the API received it. That browser context was closed. A clean
context signed in, found the saved intent, reviewed it, and retried. Two authenticated
original retries raced. They returned one transfer ID, including a safe replay.

**Lost committed response:** another real command committed. Only its response was
dropped. After closing that context, another clean context found the known posted
outcome and opened the original receipt. Discovery caused no automatic financial
POST. A deliberate retry returned the same ID with `Idempotent-Replay: true`.

The final PostgreSQL proof for these two requests was:

| Evidence | Result |
|---|---:|
| Immutable saved intents | 2 |
| Original transfers | 2 |
| Ledger postings | 4 |
| Exactly balanced transfer journals | 2 |
| Source balance | INR 3.00 |
| Destination balance | INR 2.00 |

Changed recovery bodies were rejected. List output contained no key or fingerprint.
No financial command was sent to the main local ledger. Separate browser contexts
model device/storage separation; physical-device and production IdP qualification
are not claimed.

## Scope limits

The inbox covers retained confirmed intents. Older clients that did not save intent
cannot be reconstructed from an empty list. Use history and exact-reference
investigation for those requests. The UI states this limit.

Recovery belongs to the original identity. A changed subject or tenant does not
inherit it. Support handoff does not grant authority to execute another user's
transfer. Preparation reserves no money and bypasses no later policy check.

## Sources and rendered evidence

- [System design](../architecture/ledgersync-system-design.md)
- [Recovery sequence PNG](../architecture/diagrams/09-7-4-unknown-transfer-result-and-follow-up-implemented-safety-rule.png)
- [Architecture PNG](../architecture/diagrams/03-4-component-architecture.png)
- [Deployment PNG](../architecture/diagrams/15-11-local-deployment-actual-repository-topology.png)
- [Real isolated recovery screenshot](qa/journey-20261010/cross-device-recovery.jpg)
- [Running main recovery screen](qa/journey-20261010/cross-device-running-app.png)
- [Real browser journey](../../web/tests/system/transfer-recovery.real-stack.spec.ts)
- [Database checks](../../tests/integration/transfer_recovery_test.go)
