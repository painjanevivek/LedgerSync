# ADR 007: bounded external statement comparison

Date: 16 October 2026. Status: selected implementation, R15 in progress.

## Decision

Use a separate statement reconciliation module. The module accepts a documented synthetic CSV source, fixes one INR clearing mapping per tenant, preserves original bytes and their SHA-256, and stores normalized rows separately. It has no financial writer. Its non-login function owner cannot insert, update or delete accounts, transfers, journals, postings or balance projections.

Version 1 uses unquoted UTF-8 CSV, eleven ordered columns, canonical integer minor units, UTC seconds and a half-open statement period. LF and CRLF are accepted. BOM, quotes, blank records, unknown/reordered/duplicate columns, invalid UTF-8, control bytes, ambiguous amounts and inconsistent metadata are rejected. This narrow format makes independent validation inside the controlled database command practical. It is not a general bank CSV parser. The complete file must pass before acceptance.

A tenant can retain at most 100 accepted imports. Each source is at most 1 MiB and 1,000 rows. Source row identity is unique across the fixed mapping. Identical bytes return the accepted import. Repeated amounts with different row IDs remain valid. A changed body under an original actor/operation/key conflicts. Cancellation rolls back the accepted import and its receipt; retry uses the same original identity.

One immutable rule-v1 run is prepared per import. Preparation records database time, period, mapping version and at most 2,000 eligible posting IDs and exact values in one database transaction. A later backdated posting cannot enter this saved snapshot. Postings already linked under this mapping are excluded at preparation. Interrupted completion resumes the same prepared run. A completed run never changes its original classification.

The application exact matcher uses reference, amount, currency and direction. Both row and posting cardinality must be one. The controlled completion command independently checks the proposed graph against the frozen snapshot. Multiple candidates remain ambiguous. Mapping locks and unique row/posting constraints prevent conflicting consumption across runs. A posting consumed after another run's cutoff becomes an explicit exception; no alternative candidate is silently selected.

Review decisions have a tenant actor, assignee, reason, bounded comment, internal evidence link and optimistic version. Original classifications and every decision remain. A manual link may override reference only. It must use an eligible snapshot item with identical amount, currency and direction. A reviewed difference remains a difference. No statement action posts an adjustment. Any financial correction uses the existing separate correction workflow and approvals.

## Ownership and recovery

The application module owns parsing, matching and contracts. The database adapter owns scoped transactions and controlled function calls. HTTP and BFF adapters own authentication, scopes, CSRF, rate bounds, payload bounds and safe error mapping. The feature UI owns interaction state. Existing reconciliation scopes are reused; browser preference cannot enable authority.

Accepted source bytes, normalized rows, snapshots, matches, receipts and review events are retained without an automatic delete path in v1. Disable new acceptance if parser integrity is uncertain. Preserve existing evidence. The migration is forward-only. A future real source needs its own format, privacy/retention decision and managed qualification. The fixed local quotas prevent unbounded acceptance; they are not a real-source retention policy.

## Verification and costs

Tests precede implementation. The initial database test failed on the missing controlled mapping function. Actual restricted-role tests then passed original-key and byte replay, full-file atomic rejection, cancellation/retry, source immutability, unequal manual-link denial, reviewed reference override, concurrent review versions, tenant/actor denials and unchanged financial hashes. API, BFF, UI, contracts, migration/recovery and integrated closeout remain required before R15 is complete.

Independent Go and SQL CSV validation adds maintenance cost. One fixed narrow format and shared boundary fixtures make that cost explicit. Fuzzy matching, FX, general CSV dialects, provider connections, multiple mappings, automatic adjustments and source deletion are outside v1.
