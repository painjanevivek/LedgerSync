# LedgerSync API changelog

## 3.1.0 — 2026-08-31

- Added tenant-scoped webhook endpoint evidence list and detail operations with
  stable filters and bounded delivery history.
- Exposed only endpoint labels and origins; full URLs, paths, query strings,
  credentials, signing references, payloads, and raw errors remain private.
- Linked event delivery attempts to safe endpoint evidence and added an
  endpoint filter without changing immutable financial or event records.

## 3.0.0 — 2026-08-31

- Replaced cross-request correlation matching for webhook replay with an
  explicit durable approval identifier and caller-owned `Idempotency-Key` on
  both approval and execution.
- Exact retries now return the original approval or delivery job; a changed
  intent conflicts and never schedules additional delivery work.
- Preserved the immutable event and dead attempt, independent approval and
  execution actors, and the rule that replay resends an existing event without
  creating financial postings.
- This is a major contract change because replay execution now requires an
  `approval_id` body and both replay operations require `Idempotency-Key`.

## 2.1.0 — 2026-08-31

- Added the tenant-scoped `GET /approvals` evidence page for authorized funding
  and correction decisions.
- Preserved domain-qualified statuses, independent-review evidence, exact
  values, step-up state, and oldest-first cursor ordering.
- Defined `page_count` as the bounded current-page count; the contract does not
  claim or expose a cross-domain total or an approval export.

## 2.0.0 — 2026-08-29

- Removed API-client webhook activation. Endpoint control is now proven only by
  a server-initiated, signed, expiring challenge handled by the worker.
- `POST /developer/webhooks` returns endpoint metadata in
  `pending_verification`; it never returns a verification challenge.

All dates are UTC. Contract artifacts are reviewed and released together.

## 1.15.0 — 2026-08-28

- Clarified that approved webhook replays return an accepted durable job, not a mutable attempt record.
- A completed job appends immutable retry, dead-letter, or delivered attempt evidence while preserving the original event payload.
- No operation is deprecated and no sunset is scheduled.

## 1.14.0 — 2026-08-28

- Added tenant-scoped webhook registration, verification, external signing-key rotation, disablement, and stable delivery history.
- Added approval-backed replay scheduling that preserves dead attempts and requires requester-approver separation.
- Production registration requires HTTPS; sandbox HTTP is restricted to loopback, and raw signing material is never accepted.
- No operation is deprecated and no sunset is scheduled.

## 1.13.0 — 2026-08-28

- Added tenant-scoped credential metadata create, list, detail, rotate, and revoke operations.
- Added optimistic versions, exact retry identity, expiry, revocation, and rate-bounded last-used evidence without accepting or returning raw credentials.
- Added dedicated `credentials:read` and `credentials:write` scopes and recent production step-up protection for mutations.
- No operation is deprecated and no sunset is scheduled.

## 1.12.0 — 2026-08-28

- Added server-owned `X-LedgerSync-Mode` and `X-LedgerSync-API-Version` headers to every API response.
- Published semantic-version, supported-window, deprecation, and generated-artifact policy.
- No operation is deprecated and no sunset is scheduled.

## 1.11.0 — 2026-08-28

- Added additive transfer-correction request, review, approval, cancellation, posting, and immutable evidence operations.
- Advanced transfer and account history schemas so original and compensating records remain linked after lifecycle closure.
- No operation is deprecated and no sunset is scheduled.

## 1.10.0 — 2026-08-28

- Added controlled funding request, approval, posting, compensation, and reconciliation operations.
- Clarified that funding examples record customer-authorized external value evidence and do not claim custody or settlement.

## 1.9.0 — 2026-08-28

- Added the guided first-run evidence contract and server-owned orientation preferences.
