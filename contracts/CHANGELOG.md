# LedgerSync API changelog

All dates are UTC. Contract artifacts are reviewed and released together.

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
