# LedgerSync operator onboarding journey contract

This contract defines the resumable first-run journey for the supported local product. It supplements the canonical [UI state and recovery contract](ui-state-contract.md); it does not create financial authority or relax any write prerequisite.

## Journey boundary

- The journey remains inside the operator workspace. It is not a blocking landing page.
- It follows one INR ledger record from local dependency health through account, transfer, ledger, reconciliation, delivery, export, and recovery evidence.
- PostgreSQL is the financial authority. Redis remains disposable acceleration and never satisfies a financial step.
- Green completion must name its source: either durable stored evidence or an explicit server-owned operator confirmation. The two sources are never interchangeable.
- A missing product capability is `unavailable`, not complete, empty, or failed. The controlled-funding step remains blocked until Phase 5 supplies approved funding journals.

## Ordered steps

| ID | Operator outcome | Completion source |
|---|---|---|
| `confirm_health` | Confirm local system health | Explicit operator confirmation after opening current dependency evidence |
| `understand_authority` | Understand PostgreSQL authority and Redis disposability | Explicit operator confirmation |
| `inspect_accounts` | Inspect an authorized demo account and its history | Stored evidence available, then explicit operator confirmation |
| `create_account` | Create an active zero-balance INR account | Durable account evidence |
| `fund_account` | Fund through an approved ledger event | Phase 5 durable funding evidence; unavailable before that workflow exists |
| `post_transfer` | Move an exact integer-minor-unit amount | Durable posted-transfer evidence |
| `retry_transfer` | Verify same-intent, same-key retry behavior | Durable transfer evidence, then explicit operator confirmation |
| `inspect_postings` | Inspect equal debit/credit postings and balance versions | Durable journal evidence, then explicit operator confirmation |
| `run_reconciliation` | Compare ledger postings with PostgreSQL balance truth at a watermark | Durable completed reconciliation evidence |
| `inspect_delivery` | Inspect outbox publication and delivery attempts separately from posting | Durable event evidence, then explicit operator confirmation |
| `export_evidence` | Review a bounded, sanitized evidence export | Durable exportable evidence, then explicit operator confirmation |
| `create_backup` | Create and verify protected recovery evidence | Durable verified-backup evidence |

The service owns ordering, prerequisites, evidence state, recommendation, and completion labels. The browser does not infer completion from navigation, clicks, local storage, or optimistic state.

## Preference and concurrency rules

- Preferences are scoped to the authenticated tenant and subject and stored in PostgreSQL.
- The mutable preference contains only presentation state: dismissal and allowlisted operator-confirmed step IDs. It cannot create accounts, money, journals, reconciliations, events, exports, or backups.
- Updates require the current integer preference version. A stale version returns `409 preference_version_conflict`; the browser refreshes authoritative state instead of overwriting another session.
- Unknown mutation outcomes do not receive an automatic retry. The browser refreshes first because the write may already have committed.
- Reset clears only manual confirmations. It never deletes ledger, audit, event, reconciliation, export, or recovery evidence.
- Dismissal compacts the guide in place, preserves server-owned progress, and exposes a keyboard-operable reopen control.

## Recommendation and safe-stop rules

- Overview presents one recommended next action: the first incomplete step in service-defined order.
- A recommendation links to the narrowest existing product surface and preserves its allowlisted return context.
- Commands remain disabled when scope, connectivity, evidence freshness, or product capability is insufficient.
- Controlled funding is intentionally stop-ship until the approved Phase 5 workflow exists; a transfer is never presented as funding.
- Stopping the local runtime preserves PostgreSQL preferences and evidence. Restarting manual progress never resets product data.
- No global keyboard shortcut is introduced: the current controls are discoverable in normal tab order, and a hidden shortcut would add conflict without improving the journey.

## Plain-language definitions

- **Idempotency:** retry the same intent with the same key so a lost response cannot create another movement.
- **Double entry:** every posted journal balances equal debit and credit postings.
- **Reconciliation:** compare stored ledger postings with authoritative balance truth at a recorded watermark.
- **Projection:** a derived view of authoritative records; useful for reading, never a replacement for the ledger.
- **Response unknown:** the request may have committed although the browser did not receive the result; refresh evidence before deciding what to do next.
