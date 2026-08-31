# LedgerSync API changelog

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
