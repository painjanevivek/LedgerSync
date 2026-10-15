# Financial writer cutover

Updated: 15 October 2026. R03 is locally verified through migration 000060. The normal runtime is not cut over. See the completion evidence and controlled-write cutover procedure.

The [source inventory](financial-writer-inventory.json) lists SQL writes and controlled calls found in production Go adapter string literals. Each entry names its file, function and line. It is a discovery aid. It does not establish an effective database grant. Dynamic retention SQL, migrations, triggers, seed/restore tools and controlled SQL bodies need the separate review below.

## Final qualification

All protected runtime financial writers use controlled capabilities. Recovery approval/replay, reconciliation run/mismatch/audit persistence, fenced worker completion and conservative retention now have verified narrow functions. The API, worker, reconciliation, provisioning and maintenance roles have no direct protected financial or audit DML. The source guard also checks the query templates. Ordinary direct operational metadata exceptions are listed in the [cutover procedure](controlled-write-cutover.md). Manual delivery import is administrative; no workload receives EXECUTE.

The table below retains the conversion sequence and intermediate notes. Any pending wording in it belongs to that earlier checkpoint. Final results are in [R03 completion](../release-evidence/portfolio/r03/completion.json).

## Historical conversion order and state

| Family | Existing or implemented boundary | Remaining direct authority and next change |
|---|---|---|
| Customer account creation | Existing controlled zero-opening provision function creates account, projection, opening and ownership rows, with audit | Verify all provisioning callers after insert revocation. Retain lifecycle UPDATE only until its replacement passes. |
| Funding clearing setup | Controlled exact-zero creation; finance and policy checks; one tenant/currency account; creation audit | Distinct-actor concurrency and same-key replay passed. |
| Account metadata and status | Controlled lock and mutation; version, lifecycle, authoritative close checks, required audit and outbox; Go owns parent idempotency transaction | Restricted create/edit/freeze/reactivate/close/replay passed. Additional hostile direct-call tests remain. |
| Provisioning | Controlled tenant, policy snapshot, role, credential and zero-account setup; bounded rollback | Restricted apply/replay/rollback and denial after funding passed. No direct financial writes remain. |
| Transfer posting and request reference | Controlled submit, retained recovery intent and reference attachment | Full integration/fault tests pass with API transfer/journal/posting/velocity writes revoked. |
| Funding requests and decisions | Controlled request, approval, rejection, compensation request and posting | Restricted full workflow, exact postings, immutable approvals and replay passed. Final negative and concurrent decision qualification remains. |
| Transfer correction requests and decisions | Controlled request, approval, rejection, cancellation, expiry and exact compensation posting | Independent actor, recent step-up, exact reversal and replay passed. Additional direct-call and concurrent decision qualification remains. |
| Outbox and webhook processing | Bounded claims; controlled renewal/completion/publication/receipt; controlled verification activation | Direct worker queue/finalize writes revoked. Full worker, duplicate/stale lease and process crash tests passed. |
| Recovery approval and replay | Scoped operator/approver commands with immutable action records | Convert approval/replay writes. Keep separate actors, approved target, expiry, request key and original dead evidence. |
| Reconciliation | Scoped complete tenant snapshot and atomic run/mismatch/audit records | Convert evidence persistence while retaining read-only financial authority. Keep scheduler login-to-tenant restrictions. |
| Audit | Controlled bounded append; tenant/actor binding, metadata shape and sensitive-key checks | Ordinary audit INSERT revoked. Full integration/fault paths passed. Additional hostile metadata tests remain. |
| Retention | Bounded scoped deletion of eligible operational records | Keep explicit operating identity and retained financial/idempotency evidence. Convert or document only required maintenance authority. |
| Credential, webhook and saved investigation state | Scoped metadata commands and append-only event records | Keep each operation's role and actor boundary. No generic arbitrary-table write function is allowed. |

## First cutover

Migration `000047` adds controlled clearing-account setup. It accepts no opening amount, account kind or balance value. It checks finance membership, active policy, tenant context and actor context. A competing serializable creation retries the whole command with a new snapshot. It creates the account, zero projection, zero opening and creation audit in one transaction.

The API and provisioning roles no longer receive INSERT on accounts, opening balances, balance projections, account owners or credit permissions. Existing controlled customer-account creation owns those writes. The current capability matrix has been updated for this narrow cutover. Other current gaps remain visible.

The failing restricted-role test is retained in the private R03 evidence directory. The replacement passed the same test. Concurrent request and broader workflow qualification are recorded as tests complete. R03 is complete only when the target matrix and all financial workflows pass under restricted roles.

## Rules for the next packets

Use typed, fixed-signature functions. Use fixed object resolution. Do not pass caller SQL or arbitrary table names. Keep required financial state, idempotency, audit and event obligations in one transaction. Use application validation for useful errors and database checks for the protected invariant. Neither layer substitutes for the other.

Do not remove the previous grant before a working caller and a positive restricted-role test exist. Do not weaken a target expectation to match an unsafe grant. Retain historical evidence and issue records. All migrations and tests run in the disposable local stack until the full candidate and cutover procedure are qualified.
